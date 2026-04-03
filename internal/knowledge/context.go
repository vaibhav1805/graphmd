package knowledge

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─── context section type ─────────────────────────────────────────────────────

// ContextSection represents a single retrieved section of a document to be
// included in a RAG-ready context block.
type ContextSection struct {
	File           string  // relative file path, e.g. "auth-service.md"
	HeadingPath    string  // e.g. "Database Layer" or "" for preamble
	Content        string  // raw chunk content (markdown)
	Score          float64 // relevance score from retrieval
	ReasoningTrace string  // LLM explanation for why this section matched (pageindex only)
}

// ─── block assembly ───────────────────────────────────────────────────────────

// AssembleContextBlock formats a slice of ContextSection values as a
// markdown-formatted context block suitable for direct injection into an LLM
// prompt.
//
// Output format:
//
//	# Context for: {query}
//
//	## [{file} § {heading}]
//	{content}
//
//	## [{file}]
//	{content} (when HeadingPath is empty — preamble section)
//
// Returns an empty string when sections is nil or empty.
func AssembleContextBlock(query string, sections []ContextSection) string {
	if len(sections) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Context for: ")
	sb.WriteString(query)
	sb.WriteByte('\n')

	for _, s := range sections {
		sb.WriteByte('\n')
		sb.WriteString("## [")
		sb.WriteString(s.File)
		if s.HeadingPath != "" {
			sb.WriteString(" \u00a7 ")
			sb.WriteString(s.HeadingPath)
		}
		sb.WriteString("]\n")
		sb.WriteString(s.Content)
		sb.WriteByte('\n')
	}

	return sb.String()
}

// sectionsFromBM25Results converts a slice of SearchResult values (from a
// BM25 search) into a slice of ContextSection for use in the BM25 fallback
// path of CmdContext.
//
// Mapping:
//   - SearchResult.RelPath     → ContextSection.File
//   - SearchResult.HeadingPath → ContextSection.HeadingPath
//   - SearchResult.ContentPreview → ContextSection.Content (falls back to Snippet)
//   - SearchResult.Score       → ContextSection.Score
func sectionsFromBM25Results(results []SearchResult) []ContextSection {
	sections := make([]ContextSection, 0, len(results))
	for _, r := range results {
		content := r.ContentPreview
		if content == "" {
			content = r.Snippet
		}
		sections = append(sections, ContextSection{
			File:        r.RelPath,
			HeadingPath: r.HeadingPath,
			Content:     content,
			Score:       r.Score,
		})
	}
	return sections
}

// ─── context args and parser ──────────────────────────────────────────────────

// ContextArgs holds parsed CLI arguments for CmdContext.
type ContextArgs struct {
	Query    string
	Dir      string
	Top      int    // max sections to return; default 5
	Format   string // "markdown" (default) | "json"
	Strategy string // "" (env var/default) | "bm25" | "pageindex"
	Model    string // LLM model; default "claude-sonnet-4-5"
}

// ParseContextArgs parses raw CLI arguments for the context command.
//
// Usage: bmd context QUERY [--dir DIR] [--top N] [--format markdown|json] [--strategy bm25|pageindex] [--model MODEL]
func ParseContextArgs(args []string) (*ContextArgs, error) {
	positionals, flags := splitPositionalsAndFlags(args)

	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var a ContextArgs
	fs.StringVar(&a.Dir, "dir", ".", "Directory to search")
	fs.IntVar(&a.Top, "top", 5, "Maximum number of sections to return")
	fs.StringVar(&a.Format, "format", "markdown", "Output format (markdown|json)")
	fs.StringVar(&a.Strategy, "strategy", "", "Strategy: '' or 'bm25' (default) | 'pageindex'")
	fs.StringVar(&a.Model, "model", "claude-sonnet-4-5", "LLM model for PageIndex retrieval")

	if err := fs.Parse(flags); err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	if len(positionals) < 1 {
		return nil, fmt.Errorf("context: QUERY argument required")
	}
	a.Query = positionals[0]

	// Resolve strategy: flag → env var → default
	a.Strategy = resolveStrategy(a.Strategy)

	return &a, nil
}

// ─── command implementation ───────────────────────────────────────────────────

// CmdContext implements `bmd context QUERY`.  It assembles a RAG-ready context
// block by retrieving the most relevant sections for the given query.
//
// Strategy (respects --strategy flag, env var BMD_STRATEGY, or auto-detect):
//  1. If strategy == "pageindex": attempt to load .bmd-tree.json files and invoke RunPageIndexQuery.
//  2. If strategy == "bm25" or if PageIndex unavailable: use BM25 chunk search.
//  3. If strategy is auto (from env var or default): attempt PageIndex first, fall back to BM25.
//
// Output:
//   - format=="markdown": prints AssembleContextBlock output to stdout.
//   - format=="json":     prints a ContractResponse envelope to stdout.
func CmdContext(args []string) error {
	a, err := ParseContextArgs(args)
	if err != nil {
		return err
	}

	absDir, err := filepath.Abs(a.Dir)
	if err != nil {
		return fmt.Errorf("context: resolve dir %q: %w", a.Dir, err)
	}

	var sections []ContextSection

	// Strategy routing: force BM25, or attempt PageIndex if available.
	useBM25Only := strings.ToLower(a.Strategy) == "bm25"

	if !useBM25Only {
		// Attempt PageIndex tree path (either explicit pageindex strategy or auto-detect).
		trees, treeErr := LoadTreeFiles(absDir)
		if treeErr == nil && len(trees) > 0 {
			fmt.Fprintf(os.Stderr, "Using PageIndex trees (%d files)\n", len(trees))
			cfg := PageIndexConfig{
				ExecutablePath: "pageindex",
				Model:          a.Model,
			}
			piSections, piErr := RunPageIndexQuery(cfg, a.Query, trees, a.Top)
			if piErr == nil {
				sections = piSections
			} else {
				// PageIndex query failed — fall through to BM25.
				fmt.Fprintf(os.Stderr, "PageIndex query failed (%v); falling back to BM25 chunk search\n", piErr)
			}
		}
	}

	// BM25 fallback when no trees or PageIndex failed.
	if sections == nil {
		fmt.Fprintf(os.Stderr, "No .bmd-tree.json files found; falling back to BM25 chunk search\n")

		dbPath := defaultDBPath(absDir)
		db, dbErr := openOrBuildIndex(absDir, dbPath)
		if dbErr != nil {
			// If we still can't build an index, bail out gracefully.
			fmt.Printf("No relevant context found for: %s\n", a.Query)
			return nil
		}
		defer db.Close() //nolint:errcheck

		idx := NewIndex()
		if loadErr := db.LoadIndex(idx); loadErr != nil {
			fmt.Printf("No relevant context found for: %s\n", a.Query)
			return nil
		}

		// Re-scan to populate ContentPreview (DB stores only metadata).
		// Use default Knowledge configuration for backward compatibility.
		k := DefaultKnowledge()
		if docs, scanErr := k.Scan(absDir); scanErr == nil {
			_ = idx.Build(docs)
		}

		results, searchErr := idx.Search(a.Query, a.Top)
		if searchErr != nil || len(results) == 0 {
			fmt.Printf("No relevant context found for: %s\n", a.Query)
			return nil
		}

		sections = sectionsFromBM25Results(results)
	}

	if len(sections) == 0 {
		fmt.Printf("No relevant context found for: %s\n", a.Query)
		return nil
	}

	// Format and print output.
	if strings.ToLower(a.Format) == "json" {
		printContextJSON(a.Query, sections)
		return nil
	}

	// Default: markdown.
	fmt.Print(AssembleContextBlock(a.Query, sections))
	return nil
}

// printContextJSON marshals sections as a ContractResponse envelope and prints
// to stdout.
func printContextJSON(query string, sections []ContextSection) {
	type sectionJSON struct {
		File        string  `json:"file"`
		HeadingPath string  `json:"heading_path,omitempty"`
		Content     string  `json:"content"`
		Score       float64 `json:"score"`
	}

	items := make([]sectionJSON, len(sections))
	for i, s := range sections {
		items[i] = sectionJSON{
			File:        s.File,
			HeadingPath: s.HeadingPath,
			Content:     s.Content,
			Score:       roundFloat(s.Score, 4),
		}
	}

	payload := map[string]interface{}{
		"query":    query,
		"sections": items,
		"count":    len(items),
	}

	fmt.Println(marshalContract(NewOKResponse("Context assembled", payload)))
}

// ─── pageindex query subprocess ───────────────────────────────────────────────

// pageIndexQueryResult is the per-section JSON object returned by
// `pageindex query --format json`.
type pageIndexQueryResult struct {
	File           string  `json:"file"`
	HeadingPath    string  `json:"heading_path"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
	ReasoningTrace string  `json:"reasoning_trace"`
}

// RunPageIndexQuery invokes the pageindex CLI to search an existing tree index.
// It marshals trees to JSON and passes them via stdin to the subprocess, then
// parses the JSON array response into []ContextSection.
//
// Command invoked:
//
//	pageindex query --query QUERY --model MODEL --top N --format json
//
// Returns (nil, ErrPageIndexNotFound wrapping) when the binary is not found.
// Returns (nil, err) on other subprocess or parse failures; caller should fall
// back to BM25.
func RunPageIndexQuery(cfg PageIndexConfig, query string, trees []FileTree, top int) ([]ContextSection, error) {
	treesJSON, err := json.Marshal(trees)
	if err != nil {
		return nil, fmt.Errorf("RunPageIndexQuery: marshal trees: %w", err)
	}

	cmd := buildPageIndexQueryCmd(cfg, query, top)
	cmd.Stdin = bytes.NewReader(treesJSON)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if runErr := cmd.Run(); runErr != nil {
		errMsg := runErr.Error()
		if strings.Contains(errMsg, "executable file not found") ||
			strings.Contains(errMsg, "no such file or directory") {
			return nil, fmt.Errorf("pageindex not found: %w", ErrPageIndexNotFound)
		}
		return nil, fmt.Errorf("RunPageIndexQuery: subprocess: %w", runErr)
	}

	var results []pageIndexQueryResult
	if parseErr := json.Unmarshal(stdout.Bytes(), &results); parseErr != nil {
		return nil, fmt.Errorf("RunPageIndexQuery: parse JSON: %w", parseErr)
	}

	sections := make([]ContextSection, len(results))
	for i, r := range results {
		sections[i] = ContextSection{
			File:           r.File,
			HeadingPath:    r.HeadingPath,
			Content:        r.Content,
			Score:          r.Score,
			ReasoningTrace: r.ReasoningTrace,
		}
	}

	return sections, nil
}

// buildPageIndexQueryCmd constructs the exec.Cmd for a pageindex query
// invocation.  Extracted for testability.
func buildPageIndexQueryCmd(cfg PageIndexConfig, query string, top int) *exec.Cmd {
	return exec.Command(
		cfg.ExecutablePath,
		"query",
		"--query", query,
		"--model", cfg.Model,
		"--top", fmt.Sprintf("%d", top),
		"--format", "json",
	)
}
