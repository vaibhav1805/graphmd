package knowledge

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Extractor extracts relationship edges from a single Document.
//
// Three extraction strategies are applied in sequence:
//  1. LinkExtractor — markdown links → EdgeReferences edges (confidence 1.0)
//  2. MentionExtractor — prose dependency patterns → EdgeDependsOn / EdgeMentions (confidence 0.7)
//  3. CodeExtractor — import/require statements in fenced code blocks → EdgeCalls (confidence 0.9)
type Extractor struct {
	// root is the absolute path of the scanned directory.  It is used to
	// validate whether link targets exist on disk.
	root string

	// md is the goldmark instance used to parse markdown into an AST.
	md goldmark.Markdown
}

// NewExtractor creates an Extractor for documents rooted at root.
// root should be the same root directory used by ScanDirectory so that link
// targets can be resolved correctly.
func NewExtractor(root string) *Extractor {
	return &Extractor{
		root: root,
		md:   goldmark.New(),
	}
}

// Extract analyses doc and returns all discovered relationship edges.
//
// The three extractors are applied in sequence; results are concatenated.
// Malformed links and code patterns are skipped silently (no panics).
func (ex *Extractor) Extract(doc *Document) []*Edge {
	var edges []*Edge
	edges = append(edges, ex.extractLinks(doc)...)
	edges = append(edges, ex.extractMentions(doc)...)
	edges = append(edges, ex.extractCode(doc)...)
	return edges
}

// --- LinkExtractor -----------------------------------------------------------

// extractLinks walks the goldmark AST for doc.Content and produces a
// EdgeReferences edge for every markdown link whose destination resolves to
// another markdown file.
func (ex *Extractor) extractLinks(doc *Document) []*Edge {
	src := []byte(doc.Content)
	reader := text.NewReader(src)
	parsed := ex.md.Parser().Parse(reader)

	var edges []*Edge

	ast.Walk(parsed, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck
		if !entering {
			return ast.WalkContinue, nil
		}

		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}

		dest := string(link.Destination)
		if dest == "" {
			return ast.WalkContinue, nil
		}

		// Skip non-file links (http, https, mailto, anchors, …).
		lower := strings.ToLower(dest)
		if strings.HasPrefix(lower, "http://") ||
			strings.HasPrefix(lower, "https://") ||
			strings.HasPrefix(lower, "mailto:") ||
			strings.HasPrefix(lower, "#") ||
			strings.HasPrefix(lower, "ftp://") {
			return ast.WalkContinue, nil
		}

		// Strip inline anchor fragment (#section) before resolving.
		if idx := strings.Index(dest, "#"); idx >= 0 {
			dest = dest[:idx]
		}
		if dest == "" {
			return ast.WalkContinue, nil
		}

		// Resolve to canonical relative path.
		canonical, confidence := ResolveLink(doc.RelPath, dest, ex.root)
		if canonical == "" {
			return ast.WalkContinue, nil
		}

		// Extract link text as evidence.
		var linkText strings.Builder
		for c := link.FirstChild(); c != nil; c = c.NextSibling() {
			if textNode, ok := c.(*ast.Text); ok {
				linkText.Write(textNode.Segment.Value(src))
			}
		}
		evidence := linkText.String()
		if evidence == "" {
			evidence = dest
		}

		edge, err := NewEdge(doc.ID, canonical, EdgeReferences, confidence, evidence)
		if err != nil {
			// Skip self-loops and invalid edges silently.
			return ast.WalkContinue, nil
		}

		edges = append(edges, edge)
		return ast.WalkContinue, nil
	}) //nolint:errcheck

	return edges
}

// --- MentionExtractor --------------------------------------------------------

// mentionPatterns is a list of (compiled regex, EdgeType) pairs.
// Each regex captures the referenced component name in the first capture group.
// Patterns are matched case-insensitively against each line of plain text.
var mentionPatterns = []struct {
	re       *regexp.Regexp
	edgeType EdgeType
}{
	{regexp.MustCompile(`(?i)\bdepends\s+on\s+([\w\-./]+)`), EdgeDependsOn},
	{regexp.MustCompile(`(?i)\brequires\s+([\w\-./]+)`), EdgeDependsOn},
	{regexp.MustCompile(`(?i)\bcalls\s+([\w\-./]+)`), EdgeCalls},
	{regexp.MustCompile(`(?i)\bintegrates\s+with\s+([\w\-./]+)`), EdgeMentions},
	{regexp.MustCompile(`(?i)\bimplements\s+([\w\-./]+)`), EdgeImplements},
	{regexp.MustCompile(`(?i)\buses\s+([\w\-./]+)`), EdgeMentions},
}

// extractMentions searches the plain-text content of doc for prose dependency
// patterns and produces EdgeDependsOn / EdgeMentions edges.
func (ex *Extractor) extractMentions(doc *Document) []*Edge {
	lines := strings.Split(doc.PlainText, "\n")

	var edges []*Edge
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for _, pat := range mentionPatterns {
			matches := pat.re.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				mentioned := m[1]
				if mentioned == "" {
					continue
				}

				// Normalise to lower case for dedup key.
				dedupKey := doc.ID + "\x00" + strings.ToLower(mentioned) + "\x00" + string(pat.edgeType)
				if seen[dedupKey] {
					continue
				}
				seen[dedupKey] = true

				// Attempt to resolve as a file path for higher-fidelity linking.
				// If resolution fails, use the mention text directly as target.
				target := strings.ToLower(mentioned)
				canonical, confidence := ResolveLink(doc.RelPath, mentioned+".md", ex.root)
				if canonical != "" {
					target = canonical
				} else {
					confidence = ConfidenceMention
				}

				edge, err := NewEdge(doc.ID, target, pat.edgeType, confidence, line)
				if err != nil {
					continue
				}
				edges = append(edges, edge)
			}
		}
	}

	return edges
}

// --- CodeExtractor -----------------------------------------------------------

// codeExtractors maps language names to their import pattern extractors.
// Each extractor function accepts a code block body and returns a slice of
// (target, evidence) pairs.
var codeExtractors = map[string]func(string) []codeRef{
	"go":         extractGoImports,
	"python":     extractPythonImports,
	"py":         extractPythonImports,
	"javascript": extractJSImports,
	"js":         extractJSImports,
	"typescript": extractJSImports,
	"ts":         extractJSImports,
}

// codeRef is an intermediate result from a language-specific code extractor.
type codeRef struct {
	target   string
	evidence string
}

// extractCode parses fenced code blocks in the raw markdown source and
// produces EdgeCalls edges for each detected import/require statement.
func (ex *Extractor) extractCode(doc *Document) []*Edge {
	var edges []*Edge
	seen := make(map[string]bool)

	// Use a simple state machine to parse fenced code blocks without relying on
	// goldmark code-block node extraction.
	type codeBlock struct {
		lang string
		body strings.Builder
	}

	lines := strings.Split(doc.Content, "\n")
	var block *codeBlock

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		if block == nil {
			// Look for opening fence with optional language specifier.
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				fence := trimmed[:3]
				lang := strings.ToLower(strings.TrimSpace(trimmed[len(fence):]))
				// Strip any trailing fence characters that some editors add.
				lang = strings.TrimRight(lang, "`~")
				block = &codeBlock{lang: lang}
			}
			continue
		}

		// Check for closing fence.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			// Process the accumulated block.
			if extFn, ok := codeExtractors[block.lang]; ok {
				refs := extFn(block.body.String())
				for _, ref := range refs {
					dedupKey := doc.ID + "\x00" + ref.target
					if seen[dedupKey] {
						continue
					}
					seen[dedupKey] = true

					edge, err := NewEdge(doc.ID, ref.target, EdgeCalls, ConfidenceCode, ref.evidence)
					if err != nil {
						continue
					}
					edges = append(edges, edge)
				}
			}
			block = nil
			continue
		}

		block.body.WriteString(rawLine)
		block.body.WriteByte('\n')
	}

	return edges
}

// --- Language-specific import extractors ------------------------------------

var (
	goImportSingle = regexp.MustCompile(`^\s*import\s+"([^"]+)"`)
	goImportBlock  = regexp.MustCompile(`"([^"]+)"`)
	goFuncCall     = regexp.MustCompile(`(\w+)\.(\w+)\(`)

	pyFromImport = regexp.MustCompile(`^\s*from\s+([\w.]+)\s+import`)
	pyImport     = regexp.MustCompile(`^\s*import\s+([\w.,\s]+)`)
	pyFuncCall   = regexp.MustCompile(`([\w]+)\.([\w]+)\(`)

	// jsImport matches:
	//   import 'module'
	//   import { foo } from 'module'
	//   import * as x from "module"
	//   const x = require('module')
	// The (?:from\s+)? allows the optional "from" keyword before the quote.
	jsImport = regexp.MustCompile(`(?:import|require)(?:\s+[\w{}\s*,]+from)?\s*(?:\(?\s*)?['"]([^'"]+)['"]`)
)

// extractGoImports finds import paths and qualified function calls in Go code.
func extractGoImports(body string) []codeRef {
	var refs []codeRef
	lines := strings.Split(body, "\n")
	inImportBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Ignore comment lines.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if strings.HasPrefix(trimmed, "import (") || trimmed == "import(" {
			inImportBlock = true
			continue
		}
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock {
			m := goImportBlock.FindStringSubmatch(trimmed)
			if m != nil && m[1] != "" {
				refs = append(refs, codeRef{target: m[1], evidence: strings.TrimSpace(line)})
			}
			continue
		}

		// Single-line import.
		if m := goImportSingle.FindStringSubmatch(line); m != nil {
			refs = append(refs, codeRef{target: m[1], evidence: strings.TrimSpace(line)})
			continue
		}

		// pkg.Func() call patterns (skip common builtins).
		for _, m := range goFuncCall.FindAllStringSubmatch(line, -1) {
			pkg := m[1]
			if isGoBuiltinPkg(pkg) {
				continue
			}
			refs = append(refs, codeRef{target: pkg, evidence: strings.TrimSpace(line)})
		}
	}

	return refs
}

// extractPythonImports finds import statements in Python code.
func extractPythonImports(body string) []codeRef {
	var refs []codeRef
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// from X import ...
		if m := pyFromImport.FindStringSubmatch(line); m != nil {
			refs = append(refs, codeRef{target: m[1], evidence: trimmed})
			continue
		}

		// import X, Y, Z
		if m := pyImport.FindStringSubmatch(line); m != nil {
			for _, mod := range strings.Split(m[1], ",") {
				mod = strings.TrimSpace(mod)
				if mod != "" {
					refs = append(refs, codeRef{target: mod, evidence: trimmed})
				}
			}
			continue
		}

		// module.function() call patterns.
		for _, m := range pyFuncCall.FindAllStringSubmatch(line, -1) {
			pkg := m[1]
			if isPyBuiltin(pkg) {
				continue
			}
			refs = append(refs, codeRef{target: pkg, evidence: trimmed})
		}
	}

	return refs
}

// extractJSImports finds import/require statements in JavaScript/TypeScript.
func extractJSImports(body string) []codeRef {
	var refs []codeRef
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Ignore comment lines.
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		for _, m := range jsImport.FindAllStringSubmatch(line, -1) {
			if m[1] != "" {
				refs = append(refs, codeRef{target: m[1], evidence: trimmed})
			}
		}
	}

	return refs
}

// --- helpers for filtering builtins ------------------------------------------

// isGoBuiltinPkg returns true for single-character and stdlib-common identifiers
// that appear as package references in qualified calls but are not imports.
func isGoBuiltinPkg(name string) bool {
	switch name {
	case "fmt", "os", "io", "log", "err", "t", "r", "s", "b", "n",
		"strings", "strconv", "sync", "time", "math", "bytes",
		"bufio", "sort", "rand", "big", "http", "url", "json", "context":
		return true
	}
	return false
}

// isPyBuiltin returns true for common Python built-in names used in method calls.
func isPyBuiltin(name string) bool {
	switch name {
	case "self", "cls", "str", "int", "float", "list", "dict", "set",
		"tuple", "bool", "len", "range", "print", "type", "super",
		"object", "None", "True", "False":
		return true
	}
	return false
}

// --- ResolveLink -------------------------------------------------------------

// ResolveLink resolves a link destination found in sourcePath to a canonical
// relative path (relative to the index root).
//
// Parameters:
//   - sourcePath: relative path of the document containing the link (e.g. "services/auth.md")
//   - linkDest: the raw link destination from the markdown (e.g. "../api/gateway.md")
//   - root: absolute filesystem path of the index root directory
//
// Returns:
//   - canonical: the resolved relative path (forward slashes), or "" to signal
//     that the link should be skipped (self-reference, etc.)
//   - confidence: ConfidenceLink (1.0) when the target exists on disk, or
//     ConfidenceUnresolved (0.5) when it cannot be found
//
// Path resolution rules:
//  1. Strip query strings and anchor fragments (#...) before processing.
//  2. If linkDest is an absolute path starting with "/" it is treated as
//     relative to root.
//  3. Relative paths are resolved from the directory containing sourcePath.
//  4. The result is normalised to forward slashes.
//  5. A link that resolves to sourcePath itself (self-reference) returns "".
//  6. Circular symlinks are not followed (os.Lstat is used for existence checks).
func ResolveLink(sourcePath, linkDest, root string) (canonical string, confidence float64) {
	if linkDest == "" {
		return "", 0
	}

	// Strip anchor fragment.
	if idx := strings.Index(linkDest, "#"); idx >= 0 {
		linkDest = linkDest[:idx]
	}

	// Strip query string.
	if idx := strings.Index(linkDest, "?"); idx >= 0 {
		linkDest = linkDest[:idx]
	}

	linkDest = strings.TrimSpace(linkDest)
	if linkDest == "" {
		return "", 0
	}

	// Normalise path separators (Windows compatibility).
	linkDest = filepath.ToSlash(linkDest)
	sourcePath = filepath.ToSlash(sourcePath)

	var resolved string

	if strings.HasPrefix(linkDest, "/") {
		// Absolute-from-root: treat as relative to the root.
		resolved = path.Clean(strings.TrimPrefix(linkDest, "/"))
	} else {
		// Relative: resolve from the directory containing sourcePath.
		sourceDir := path.Dir(sourcePath)
		resolved = path.Clean(path.Join(sourceDir, linkDest))
	}

	// Reject self-references.
	if resolved == path.Clean(sourcePath) {
		return "", 0
	}

	// Check whether the resolved target exists on disk to set confidence.
	confidence = ConfidenceLink
	if root != "" {
		absTarget := filepath.Join(filepath.FromSlash(root), filepath.FromSlash(resolved))
		// Use Lstat to avoid following symlinks (circular link guard).
		if _, err := os.Lstat(absTarget); err != nil {
			confidence = ConfidenceUnresolved
		}
	}

	return resolved, confidence
}
