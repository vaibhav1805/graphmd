package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SearchResult is the user-facing result returned by Index.Search.
type SearchResult struct {
	// DocID is the document's unique identifier.
	// After chunk-level indexing this is the chunk DocID:
	// "relPath#HeadingPath:L{startLine}". RelPath continues to identify the file.
	DocID string `json:"docId"`

	// Path is the absolute filesystem path to the source file.
	Path string `json:"path"`

	// RelPath is the root-relative path (forward slashes).
	RelPath string `json:"relPath"`

	// Title is the document's heading title (or filename stem).
	Title string `json:"title"`

	// Score is the BM25 relevance score.  Higher is more relevant.
	// The value is the raw BM25 sum and is NOT normalised to [0,1] because
	// normalisation is corpus-dependent and can be misleading for single-query use.
	Score float64 `json:"score"`

	// Snippet is up to 200 runes of plain-text content starting from the first
	// line that contains any query term, or the document beginning.
	Snippet string `json:"snippet"`

	// MatchCount is the number of distinct query terms that appeared in this
	// document.
	MatchCount int `json:"matchCount"`

	// New chunk-level fields (zero value = file-level result, backward compatible)
	HeadingPath    string `json:"heading_path,omitempty"`
	StartLine      int    `json:"start_line,omitempty"`
	EndLine        int    `json:"end_line,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
}

// indexPersisted is the on-disk serialisation format for Index.
// It mirrors the in-memory BM25Index fields so that a loaded index is
// functionally identical to one built from scratch.
type indexPersisted struct {
	Params   BM25Params              `json:"params"`
	Docs     []persistedDoc          `json:"docs"`
	Postings map[string][]PostingEntry `json:"postings"`
	Stats    IndexStats              `json:"stats"`
}

// persistedDoc is the serialised form of indexedDoc (which is unexported).
type persistedDoc struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	RelPath     string `json:"relPath"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Len         int    `json:"len"`
	HeadingPath string `json:"heading_path,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
}

// Index is the top-level, user-facing API for knowledge base indexing and
// search.  It wraps BM25Index with persistence, snippet generation, and
// incremental update support.
//
// Zero value is NOT valid; always create via NewIndex.
type Index struct {
	bm25      *BM25Index
	tokenizer *Tokenizer
	params    BM25Params

	// docMeta stores per-document metadata needed for staleness detection.
	// Keyed by document ID (= RelPath, forward slashes).
	docMeta map[string]docMeta
}

// docMeta holds the metadata tracked for incremental update detection.
type docMeta struct {
	Hash         string
	LastModified int64 // Unix timestamp (nanoseconds)
}

// NewIndex creates a ready-to-use Index with the standard BM25 parameters and
// the default tokenizer configuration (stop-word removal enabled).
func NewIndex() *Index {
	params := DefaultBM25Params()
	tok := NewTokenizer(DefaultTokenizerConfig())
	return &Index{
		bm25:      NewBM25Index(params, tok),
		tokenizer: tok,
		params:    params,
		docMeta:   make(map[string]docMeta),
	}
}

// Build indexes all documents in docs, replacing any previously indexed content.
// The index is rebuilt from scratch, so stale data from previous calls is discarded.
func (idx *Index) Build(documents []Document) error {
	// Reset internal state.
	idx.bm25 = NewBM25Index(idx.params, idx.tokenizer)
	idx.docMeta = make(map[string]docMeta, len(documents))

	for i := range documents {
		doc := documents[i]
		idx.bm25.AddDocument(doc)
		idx.docMeta[doc.ID] = docMeta{
			Hash:         doc.ContentHash,
			LastModified: doc.LastModified.UnixNano(),
		}
	}
	return nil
}

// Search performs a BM25 search for query and returns the top topK results.
//
// Each result includes a relevance score, a context snippet, and a count of
// matched query terms.  Results are sorted by score descending.
// Returns an empty slice when no documents match (never returns nil).
func (idx *Index) Search(query string, topK int) ([]SearchResult, error) {
	if query == "" {
		return []SearchResult{}, nil
	}

	ranked := idx.bm25.Search(query, topK)
	if len(ranked) == 0 {
		return []SearchResult{}, nil
	}

	queryTerms := idx.tokenizer.Tokenize(query)

	results := make([]SearchResult, 0, len(ranked))
	for _, r := range ranked {
		// Retrieve plain-text content for snippet + match count computation.
		content := idx.docContent(r.DocID)

		snippet := extractSnippet(content, queryTerms, 200)
		matchCount := countMatchedTerms(content, queryTerms, idx.tokenizer)

		results = append(results, SearchResult{
			DocID:          r.DocID,
			Path:           r.Path,
			RelPath:        r.RelPath,
			Title:          r.Title,
			Score:          r.Score,
			Snippet:        snippet,
			MatchCount:     matchCount,
			HeadingPath:    r.HeadingPath,
			StartLine:      r.StartLine,
			EndLine:        r.EndLine,
			ContentPreview: contentPreview(content, 200),
		})
	}

	return results, nil
}

// Save serialises the index to a JSON file at path.
// The directory containing path must already exist.
func (idx *Index) Save(path string) error {
	p := indexPersisted{
		Params:   idx.params,
		Docs:     make([]persistedDoc, len(idx.bm25.docs)),
		Postings: idx.bm25.postings,
		Stats:    idx.bm25.stats,
	}
	for i, d := range idx.bm25.docs {
		p.Docs[i] = persistedDoc{
			ID:          d.id,
			Path:        d.path,
			RelPath:     d.relPath,
			Title:       d.title,
			Content:     d.content,
			Len:         d.len,
			HeadingPath: d.headingPath,
			StartLine:   d.startLine,
			EndLine:     d.endLine,
		}
	}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("knowledge.Index.Save: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("knowledge.Index.Save: write %q: %w", path, err)
	}
	return nil
}

// Load deserialises a previously saved index from path.
// The current index state is replaced entirely.
func (idx *Index) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("knowledge.Index.Load: read %q: %w", path, err)
	}

	var p indexPersisted
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("knowledge.Index.Load: unmarshal: %w", err)
	}

	// Validate the loaded data minimally.
	if p.Stats.TermDocs == nil {
		p.Stats.TermDocs = make(map[string]int)
	}
	if p.Postings == nil {
		p.Postings = make(map[string][]PostingEntry)
	}

	// Reconstruct the BM25Index from persisted data.
	idx.params = p.Params
	idx.bm25 = NewBM25Index(p.Params, idx.tokenizer)
	idx.bm25.docs = make([]indexedDoc, len(p.Docs))
	for i, pd := range p.Docs {
		idx.bm25.docs[i] = indexedDoc{
			id:          pd.ID,
			path:        pd.Path,
			relPath:     pd.RelPath,
			title:       pd.Title,
			content:     pd.Content,
			len:         pd.Len,
			headingPath: pd.HeadingPath,
			startLine:   pd.StartLine,
			endLine:     pd.EndLine,
		}
	}
	idx.bm25.postings = p.Postings
	idx.bm25.stats = p.Stats

	return nil
}

// DocCount returns the number of documents in the index.
func (idx *Index) DocCount() int { return idx.bm25.DocCount() }

// --- incremental support (Task 6) ---

// IsStale returns true if any of the markdown files currently on disk under
// root have changed since they were last indexed (mtime or content hash
// differs), or if new files have appeared, or if indexed files have been
// deleted.
func (idx *Index) IsStale(root string) (bool, error) {
	// Use default Knowledge configuration for backward compatibility.
	k := DefaultKnowledge()
	docs, err := k.Scan(root)
	if err != nil {
		return false, err
	}

	// Check for new or modified files.
	seen := make(map[string]struct{}, len(docs))
	for i := range docs {
		d := &docs[i]
		seen[d.ID] = struct{}{}
		meta, indexed := idx.docMeta[d.ID]
		if !indexed {
			return true, nil // new file
		}
		if meta.Hash != d.ContentHash {
			return true, nil // content changed
		}
		if meta.LastModified != d.LastModified.UnixNano() {
			return true, nil // mtime changed (may be a no-op content change)
		}
	}

	// Check for deleted files.
	for id := range idx.docMeta {
		if _, ok := seen[id]; !ok {
			return true, nil // file deleted
		}
	}

	return false, nil
}

// UpdateDocuments performs an incremental index update.
//
//   - changed: documents to add or replace in the index.
//   - removed: document IDs to delete from the index.
//
// Documents whose content hash has not changed are skipped automatically.
func (idx *Index) UpdateDocuments(changed []Document, removed []string) error {
	for _, id := range removed {
		// id is the document ID (= RelPath). After chunk-level indexing, each
		// file is stored as multiple index entries keyed by chunk DocID, so we
		// remove by RelPath rather than by exact DocID.
		idx.bm25.RemoveDocumentsByRelPath(id)
		delete(idx.docMeta, id)
	}

	for i := range changed {
		doc := changed[i]

		// Skip if hash is unchanged (no actual content difference).
		if meta, ok := idx.docMeta[doc.ID]; ok && meta.Hash == doc.ContentHash {
			continue
		}

		// Remove old version (all chunks) if it exists.
		idx.bm25.RemoveDocumentsByRelPath(doc.RelPath)

		// Re-index.
		idx.bm25.AddDocument(doc)
		idx.docMeta[doc.ID] = docMeta{
			Hash:         doc.ContentHash,
			LastModified: doc.LastModified.UnixNano(),
		}
	}
	return nil
}

// --- internal helpers --------------------------------------------------------

// docContent retrieves the plain-text content for a document by ID.
// Returns an empty string if the document is not in the index.
func (idx *Index) docContent(docID string) string {
	for _, d := range idx.bm25.docs {
		if d.id == docID {
			return d.content
		}
	}
	return ""
}

// extractSnippet returns up to maxRunes runes of plain-text context.
//
// Strategy: find the first line that contains any of the query terms, then
// return up to maxRunes runes starting from the beginning of that line.
// Falls back to the first maxRunes runes of the document if no term matches.
func extractSnippet(content string, queryTerms []string, maxRunes int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	lowerTerms := make([]string, len(queryTerms))
	for i, t := range queryTerms {
		lowerTerms[i] = strings.ToLower(t)
	}

	// Find the first line containing any query term.
	startLine := 0
	found := false
	for i, line := range lines {
		lowerLine := strings.ToLower(line)
		for _, term := range lowerTerms {
			if strings.Contains(lowerLine, term) {
				startLine = i
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	// Build snippet from startLine onwards.
	var b strings.Builder
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(line)
		if len([]rune(b.String())) >= maxRunes {
			break
		}
	}

	runes := []rune(b.String())
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}

// countMatchedTerms returns the number of distinct query terms that appear at
// least once in content.
func countMatchedTerms(content string, queryTerms []string, tok *Tokenizer) int {
	if content == "" || len(queryTerms) == 0 {
		return 0
	}

	// Build a set of tokens that actually appear in the document.
	docTokens := make(map[string]struct{}, 64)
	for _, t := range tok.Tokenize(content) {
		docTokens[t] = struct{}{}
	}

	count := 0
	for _, qt := range queryTerms {
		if _, ok := docTokens[qt]; ok {
			count++
		}
	}
	return count
}
