package knowledge

import (
	"strings"
)

// Section represents a parsed heading section in a markdown document.
// It records the heading text, level (1-6), and the line range.
type Section struct {
	// Heading is the full heading line (e.g. "## Installation").
	Heading string

	// Level is the heading depth: 1 for H1, 2 for H2, ..., 6 for H6.
	// Level 0 is used for the implicit root section (before any heading).
	Level int

	// LineStart is the 0-based index of the first line of this section
	// (the heading line itself).
	LineStart int

	// LineEnd is the 0-based index of the last line of this section
	// (inclusive). For the last section in a document, this equals
	// len(lines)-1.
	LineEnd int

	// Content holds the non-heading body lines of this section.
	Content []string
}

// ParseMarkdownStructure extracts the heading hierarchy from a slice of
// markdown lines. It returns one Section per heading found, plus a root
// section (level 0) that spans the content before the first heading.
//
// Headings are detected by ATX-style markers (one to six leading '#'
// characters followed by a space). Setext-style headings are not supported.
//
// The returned sections are in document order. Each section's LineEnd is
// set to the line immediately before the next section's heading, or to
// len(lines)-1 for the final section.
func ParseMarkdownStructure(lines []string) []Section {
	// Collect heading positions first.
	type headingPos struct {
		line  int
		level int
		text  string
	}
	var headings []headingPos

	for i, line := range lines {
		level, text, ok := parseHeadingLine(line)
		if !ok {
			continue
		}
		headings = append(headings, headingPos{line: i, level: level, text: text})
	}

	// Build sections from heading positions.
	// Always start with a root section (level 0).
	sections := make([]Section, 0, len(headings)+1)

	rootEnd := len(lines) - 1
	if len(headings) > 0 {
		rootEnd = headings[0].line - 1
		if rootEnd < 0 {
			rootEnd = -1
		}
	}

	// Root section holds pre-heading content (may be empty for documents
	// that start with a heading).
	root := Section{
		Level:     0,
		LineStart: 0,
		LineEnd:   rootEnd,
	}
	if rootEnd >= 0 {
		root.Content = lines[0 : rootEnd+1]
	}
	sections = append(sections, root)

	for idx, h := range headings {
		end := len(lines) - 1
		if idx+1 < len(headings) {
			end = headings[idx+1].line - 1
		}

		bodyStart := h.line + 1
		var content []string
		if bodyStart <= end {
			content = lines[bodyStart : end+1]
		}

		sections = append(sections, Section{
			Heading:   h.text,
			Level:     h.level,
			LineStart: h.line,
			LineEnd:   end,
			Content:   content,
		})
	}

	return sections
}

// parseHeadingLine detects an ATX-style markdown heading and returns its
// level (1-6), the heading text (without '#' markers and leading/trailing
// whitespace), and true when matched. Returns (0, "", false) for non-heading
// lines.
func parseHeadingLine(line string) (level int, text string, ok bool) {
	// Count leading '#' characters.
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}

	// Must be followed by a space (or end of string for empty headings).
	if n < len(line) && line[n] != ' ' {
		return 0, "", false
	}

	text = strings.TrimSpace(line[n:])
	return n, text, true
}

// TreeNodeFromMarkdown converts a markdown content string into a hierarchical
// FileTree. The returned tree uses the existing TreeNode / FileTree types so it
// integrates with SaveTreeFile, LoadTreeFiles, and LLMSemanticRelationships.
//
// The algorithm:
//  1. Split content into lines.
//  2. Parse heading structure with ParseMarkdownStructure.
//  3. Build a root TreeNode whose children are the top-level (H1) sections.
//     H2+ sections become children of the nearest containing H1 (and so on for
//     deeper nesting).
//
// Summary fields are populated from the first non-blank body line of each
// section (a lightweight heuristic that avoids an LLM call). Callers who need
// LLM-quality summaries should call GenerateSectionSummary on each node after
// building the tree.
//
// filePath is embedded in the returned FileTree.File field and is not used for
// any file I/O.
func TreeNodeFromMarkdown(filePath, content string) FileTree {
	lines := splitMarkdownLines(content)
	sections := ParseMarkdownStructure(lines)

	// Root node spans the entire document.
	totalLines := len(lines)
	rootSummary := extractFirstSentence(sections[0].Content)
	// If no pre-heading content, use the first section's heading as the summary.
	if rootSummary == "" && len(sections) > 1 {
		rootSummary = sections[1].Heading
	}
	rootNode := &TreeNode{
		Heading:   "",
		Summary:   rootSummary,
		LineStart: 0,
		LineEnd:   max0(totalLines - 1),
	}

	// Stack-based tree building: stack[i] holds the current open node at
	// heading level i. Level 0 = rootNode.
	type stackEntry struct {
		node  *TreeNode
		level int
	}
	stack := []stackEntry{{node: rootNode, level: 0}}

	// Skip sections[0] (the root / pre-heading section).
	for _, sec := range sections[1:] {
		node := &TreeNode{
			Heading:   sec.Heading,
			Summary:   extractFirstSentence(sec.Content),
			LineStart: sec.LineStart,
			LineEnd:   sec.LineEnd,
		}

		// Pop stack entries whose level >= current section level so we attach
		// to the nearest ancestor at a shallower level.
		for len(stack) > 1 && stack[len(stack)-1].level >= sec.Level {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1].node
		parent.Children = append(parent.Children, node)
		stack = append(stack, stackEntry{node: node, level: sec.Level})
	}

	return FileTree{
		File: filePath,
		Root: rootNode,
	}
}

// splitMarkdownLines splits content on '\n', preserving empty lines.
func splitMarkdownLines(content string) []string {
	return strings.Split(content, "\n")
}

// extractFirstSentence returns the first non-blank line from lines,
// truncated to 200 characters. Returns "" when lines is empty or all blank.
func extractFirstSentence(lines []string) string {
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// Skip markdown headings that may appear in content (shouldn't happen,
		// but defensive).
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(trimmed) > 200 {
			return trimmed[:200]
		}
		return trimmed
	}
	return ""
}

// max0 returns n if n >= 0, else 0.
func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
