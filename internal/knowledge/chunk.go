package knowledge

import (
	"strconv"
	"strings"
)

// Chunk represents a section of a markdown document bounded by headings.
// It is the unit of BM25 indexing in chunk mode.
type Chunk struct {
	// DocID is a unique identifier: "relPath#HeadingPath:L{startLine}"
	// The :L suffix prevents collisions when the same heading text appears multiple times.
	DocID string

	// RelPath is the file-relative path of the parent document.
	RelPath string

	// HeadingPath is the breadcrumb from the top heading to the current section,
	// e.g. "Installation > Prerequisites". Empty string ("") for document preamble
	// (content before the first heading in the file).
	HeadingPath string

	// StartLine is 1-based line number of the first line of this chunk.
	StartLine int

	// EndLine is 1-based line number of the last line of this chunk (inclusive).
	EndLine int

	// Content is the raw markdown text of this section (including the heading line).
	Content string
}

// extractChunks splits a markdown document into section chunks at heading boundaries.
// It returns at least one chunk for any non-empty content.
// Content before the first heading is returned as a chunk with HeadingPath="".
//
// The algorithm is a line-by-line scan (consistent with extractTitle in document.go).
// No goldmark dependency: heading detection uses prefix matching on "#" markers.
func extractChunks(relPath, content string) []Chunk {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var chunks []Chunk

	// headingStack holds the current heading text at each level (1-indexed, 0 unused).
	headingStack := make([]string, 7)

	chunkStart := 1   // 1-based start line of current chunk
	var chunkLines []string
	currentPath := "" // heading path for current chunk

	for i, line := range lines {
		lineNum := i + 1
		level, text, isHeading := parseHeading(line)

		if isHeading {
			if i > 0 {
				// Emit the accumulated chunk before starting a new one.
				if len(chunkLines) > 0 {
					chunks = append(chunks, buildChunk(relPath, currentPath, chunkStart, lineNum-1, chunkLines))
				}
				chunkStart = lineNum
				chunkLines = nil
			}
			// Update heading stack: set current level, clear all deeper levels.
			headingStack[level] = text
			for l := level + 1; l <= 6; l++ {
				headingStack[l] = ""
			}
			currentPath = buildPath(headingStack)
		}
		chunkLines = append(chunkLines, line)
	}

	// Emit final chunk.
	if len(chunkLines) > 0 {
		chunks = append(chunks, buildChunk(relPath, currentPath, chunkStart, len(lines), chunkLines))
	}

	return chunks
}

// buildChunk constructs a Chunk from accumulated lines.
// DocID uses :L{startLine} suffix to guarantee uniqueness across repeated heading names.
func buildChunk(relPath, headingPath string, startLine, endLine int, lines []string) Chunk {
	docID := relPath + "#" + headingPath + ":L" + strconv.Itoa(startLine)
	return Chunk{
		DocID:       docID,
		RelPath:     relPath,
		HeadingPath: headingPath,
		StartLine:   startLine,
		EndLine:     endLine,
		Content:     strings.Join(lines, "\n"),
	}
}

// parseHeading detects a markdown ATX heading line (e.g. "## Installation").
// Returns (level, text, true) when detected; (0, "", false) otherwise.
// Handles optional trailing # characters as CommonMark specifies.
func parseHeading(line string) (level int, text string, ok bool) {
	trimmed := strings.TrimSpace(line)
	for l := 6; l >= 1; l-- {
		prefix := strings.Repeat("#", l) + " "
		if strings.HasPrefix(trimmed, prefix) {
			headingText := strings.TrimPrefix(trimmed, prefix)
			// Strip optional trailing hashes (CommonMark spec)
			headingText = strings.TrimRight(headingText, "# ")
			return l, strings.TrimSpace(headingText), true
		}
	}
	return 0, "", false
}

// buildPath assembles the heading breadcrumb from the stack.
// e.g. headingStack = ["", "Installation", "Prerequisites", "", "", "", ""] → "Installation > Prerequisites"
func buildPath(stack []string) string {
	var parts []string
	for i := 1; i <= 6; i++ {
		if stack[i] != "" {
			parts = append(parts, stack[i])
		}
	}
	return strings.Join(parts, " > ")
}
