// Package knowledge provides markdown indexing and full-text search capabilities
// using the BM25 (Okapi BM25) ranking algorithm.
//
// Usage flow:
//  1. ScanDirectory to collect Documents from a directory tree
//  2. NewIndex + Build to construct a searchable BM25 index
//  3. Search to retrieve ranked results
//  4. Save/Load to persist the index between runs
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Document represents a single markdown file in the knowledge base.
// It holds both document metadata and the plain-text content used for indexing.
type Document struct {
	// ID is a stable identifier derived from the relative file path.
	ID string

	// Path is the absolute filesystem path to the markdown file.
	Path string

	// RelPath is the path relative to the scanned root directory.
	// Example: root=/docs, file=/docs/services/auth.md → "services/auth.md"
	RelPath string

	// Title is the first H1 heading extracted from the content, or the
	// filename stem if no H1 is present.
	Title string

	// Content holds the raw markdown source text.
	Content string

	// PlainText is the content with markdown syntax stripped, used for
	// tokenisation and snippet extraction.
	PlainText string

	// LastModified records when the file was last changed on disk.
	// Used for cache-invalidation in incremental indexing.
	LastModified time.Time

	// ContentHash is the MD5 hex digest of Content. Unchanged files share
	// the same hash and can be skipped during incremental updates.
	ContentHash string
}

// NewDocument creates a Document with the supplied metadata.
// relPath is stored as-is; the caller is responsible for normalisation.
// Returns an error if any required field is empty.
func NewDocument(id, path, relPath, title, content, plainText string, lastModified time.Time) (*Document, error) {
	if id == "" {
		return nil, fmt.Errorf("knowledge.NewDocument: id must not be empty")
	}
	if path == "" {
		return nil, fmt.Errorf("knowledge.NewDocument: path must not be empty")
	}
	if relPath == "" {
		return nil, fmt.Errorf("knowledge.NewDocument: relPath must not be empty")
	}
	return &Document{
		ID:           id,
		Path:         path,
		RelPath:      relPath,
		Title:        title,
		Content:      content,
		PlainText:    plainText,
		LastModified: lastModified,
	}, nil
}

// DocumentFromFile reads a markdown file at path, computes its metadata and
// plain text, and returns a populated Document.
//
// relPath is the path relative to the index root; it is used as the document
// ID and for display purposes.  If relPath is empty the function returns an
// error.
func DocumentFromFile(absPath, relPath string) (*Document, error) {
	if relPath == "" {
		return nil, fmt.Errorf("knowledge.DocumentFromFile: relPath must not be empty")
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		return nil, fmt.Errorf("knowledge.DocumentFromFile: stat %q: %w", absPath, err)
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("knowledge.DocumentFromFile: read %q: %w", absPath, err)
	}

	content := string(raw)
	title := extractTitle(content, relPath)
	plainText := stripMarkdown(content)
	hash := contentHash(raw)

	// Use the relPath as the document ID (forward slashes, no leading slash).
	id := filepath.ToSlash(relPath)

	return &Document{
		ID:           id,
		Path:         absPath,
		RelPath:      relPath,
		Title:        title,
		Content:      content,
		PlainText:    plainText,
		LastModified: info.ModTime(),
		ContentHash:  hash,
	}, nil
}

// DocumentCollection is an ordered set of Documents backed by a slice.
// Insertion order is preserved; no deduplication is performed.
type DocumentCollection struct {
	docs []*Document
	byID map[string]*Document
}

// NewDocumentCollection returns an empty, ready-to-use DocumentCollection.
func NewDocumentCollection() *DocumentCollection {
	return &DocumentCollection{byID: make(map[string]*Document)}
}

// Add appends doc to the collection. If a document with the same ID already
// exists it is replaced in place (order is preserved for the original entry).
func (c *DocumentCollection) Add(doc *Document) {
	if _, exists := c.byID[doc.ID]; exists {
		for i, d := range c.docs {
			if d.ID == doc.ID {
				c.docs[i] = doc
				break
			}
		}
	} else {
		c.docs = append(c.docs, doc)
	}
	c.byID[doc.ID] = doc
}

// Get returns the document with the given ID, or nil if not found.
func (c *DocumentCollection) Get(id string) *Document {
	return c.byID[id]
}

// All returns all documents in insertion order.
func (c *DocumentCollection) All() []*Document {
	out := make([]*Document, len(c.docs))
	copy(out, c.docs)
	return out
}

// Len returns the number of documents in the collection.
func (c *DocumentCollection) Len() int { return len(c.docs) }

// Remove deletes the document with the given ID. It is a no-op if the ID is
// not present.
func (c *DocumentCollection) Remove(id string) {
	if _, ok := c.byID[id]; !ok {
		return
	}
	delete(c.byID, id)
	for i, d := range c.docs {
		if d.ID == id {
			c.docs = append(c.docs[:i], c.docs[i+1:]...)
			return
		}
	}
}

// --- helpers -----------------------------------------------------------------

// extractTitle returns the text of the first H1 heading in the markdown source,
// or the filename stem (without extension) if no H1 is found.
func extractTitle(content, relPath string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	// Fall back to the filename stem.
	base := filepath.Base(relPath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// stripMarkdown performs a simple best-effort stripping of common markdown
// syntax to produce a plain-text representation suitable for tokenisation.
// It is intentionally lightweight — perfect HTML conversion is not required.
func stripMarkdown(content string) string {
	var b strings.Builder
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Remove heading markers.
		stripped := strings.TrimLeft(line, "#")
		if stripped != line {
			b.WriteString(strings.TrimSpace(stripped))
			b.WriteByte('\n')
			continue
		}

		// Remove list markers (- * + and numbered lists).
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
			b.WriteString(trimmed[2:])
			b.WriteByte('\n')
			continue
		}

		// Fenced code block delimiter lines — skip them but keep content.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			continue
		}

		// Blockquote marker.
		if strings.HasPrefix(trimmed, "> ") {
			b.WriteString(trimmed[2:])
			b.WriteByte('\n')
			continue
		}

		b.WriteString(line)
		b.WriteByte('\n')
	}

	// Remove inline markdown: bold, italic, code spans, links.
	text := b.String()
	text = removeInlineMarkdown(text)
	return text
}

// removeInlineMarkdown strips bold, italic, inline code and link syntax.
// This is deliberately simple; correctness trumps completeness.
func removeInlineMarkdown(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		switch {
		case i+1 < len(s) && s[i] == '*' && s[i+1] == '*':
			// Skip **bold** markers.
			i += 2
		case s[i] == '*' || s[i] == '_':
			// Skip single * or _ italic markers.
			i++
		case s[i] == '`':
			// Skip backtick for inline code.
			i++
		case s[i] == '[':
			// Replace [text](url) with text.
			end := strings.Index(s[i:], "]")
			if end >= 0 {
				b.WriteString(s[i+1 : i+end])
				i += end + 1
				// Skip the (url) part if present.
				if i < len(s) && s[i] == '(' {
					close := strings.Index(s[i:], ")")
					if close >= 0 {
						i += close + 1
					}
				}
			} else {
				b.WriteByte(s[i])
				i++
			}
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// contentHash returns the hex-encoded MD5 digest of data.
// MD5 is used here for speed (not security) to detect file content changes.
func contentHash(data []byte) string {
	// Imported inline to avoid a top-level import; crypto/md5 is stdlib.
	// We call it here through a helper to keep the import visible.
	return md5Hex(data)
}
