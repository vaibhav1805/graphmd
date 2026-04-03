package knowledge

import (
	"math"
	"sort"
	"strings"
)

// BM25Params holds the tuning constants for the Okapi BM25 algorithm.
//
//   - K1 (default 2.0): controls term-frequency saturation.  Higher values
//     increase the weight of term frequency; diminishing returns after ~10
//     occurrences with the default.
//   - B  (default 0.75): controls the degree to which document length
//     normalises the TF component.  0 = no length normalisation, 1 = full.
type BM25Params struct {
	K1 float64
	B  float64
}

// DefaultBM25Params returns the standard Okapi BM25 parameter values.
func DefaultBM25Params() BM25Params {
	return BM25Params{K1: 2.0, B: 0.75}
}

// PostingEntry records a single document in a term's posting list together
// with the number of times the term occurs in that document.
type PostingEntry struct {
	// DocIndex is the 0-based index into BM25Index.docs.
	DocIndex int
	// TF is the raw term frequency (count of occurrences) in the document.
	TF int
}

// indexedDoc stores the per-document data needed for BM25 scoring.
type indexedDoc struct {
	id      string
	path    string
	relPath string
	title   string
	content string // plain text, for snippet extraction
	len     int    // token count

	// chunk-level fields (zero value = file-level document, backward compatible)
	headingPath string
	startLine   int
	endLine     int
}

// IndexStats holds corpus-level statistics required for IDF computation.
type IndexStats struct {
	// N is the total number of documents in the index.
	N int
	// AvgDocLen is the average document length (token count) across the corpus.
	AvgDocLen float64
	// TermDocs maps each term to the number of documents that contain it.
	// Used to compute IDF = log((N - df + 0.5) / (df + 0.5) + 1).
	TermDocs map[string]int
}

// RankedResult pairs a document identifier with its BM25 relevance score.
type RankedResult struct {
	// DocID is the document identifier (chunk DocID after chunk-level indexing).
	DocID string
	// Path is the absolute filesystem path.
	Path string
	// RelPath is the root-relative path (forward slashes).
	RelPath string
	// Title is the document's title.
	Title string
	// Score is the raw sum of per-term BM25 contributions.
	Score float64
	// chunk-level fields (zero value = file-level document, backward compatible)
	HeadingPath string
	StartLine   int
	EndLine     int
}

// BM25Index provides in-memory BM25 indexing and search.
//
// Zero value is NOT valid; always create via NewBM25Index.
type BM25Index struct {
	params BM25Params

	// docs stores metadata for every indexed document.
	docs []indexedDoc

	// postings maps each term to its posting list.
	postings map[string][]PostingEntry

	// stats holds corpus-level statistics (computed lazily or incrementally).
	stats IndexStats

	// tokenizer is used to normalise query and document text consistently.
	tokenizer *Tokenizer
}

// NewBM25Index returns a ready-to-use BM25Index with the given parameters and
// tokenizer.  Passing nil for tokenizer uses the default tokenizer config.
func NewBM25Index(params BM25Params, tok *Tokenizer) *BM25Index {
	if tok == nil {
		tok = NewTokenizer(DefaultTokenizerConfig())
	}
	return &BM25Index{
		params:    params,
		postings:  make(map[string][]PostingEntry),
		tokenizer: tok,
		stats: IndexStats{
			TermDocs: make(map[string]int),
		},
	}
}

// AddDocument indexes a single document at chunk granularity.
// Each section (bounded by ATX headings) is indexed as a separate entry so
// that search results can identify the precise section containing a query term.
// Corpus statistics are recalculated once after all chunks are added.
func (idx *BM25Index) AddDocument(doc Document) {
	// Use Content (raw markdown) for chunk extraction when available.
	// Fall back to PlainText for documents created without Content (legacy/test data).
	indexContent := doc.Content
	if strings.TrimSpace(indexContent) == "" {
		indexContent = doc.PlainText
	}

	chunks := extractChunks(doc.RelPath, indexContent)
	if len(chunks) == 0 {
		// Fallback: index whole document as a single chunk.
		lineCount := strings.Count(indexContent, "\n") + 1
		chunks = []Chunk{{
			DocID:       doc.ID,
			RelPath:     doc.RelPath,
			HeadingPath: "",
			StartLine:   1,
			EndLine:     lineCount,
			Content:     indexContent,
		}}
	}

	for _, chunk := range chunks {
		plainText := stripMarkdown(chunk.Content)
		tokens := idx.tokenizer.Tokenize(plainText)

		// Build term-frequency map for this chunk.
		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}

		docIndex := len(idx.docs)
		idx.docs = append(idx.docs, indexedDoc{
			id:          chunk.DocID,
			path:        doc.Path,    // parent document path for file opening
			relPath:     chunk.RelPath,
			title:       doc.Title,   // parent document title
			content:     plainText,
			len:         len(tokens),
			headingPath: chunk.HeadingPath,
			startLine:   chunk.StartLine,
			endLine:     chunk.EndLine,
		})

		// Update posting lists and DF counts.
		for term, freq := range tf {
			existing := idx.postings[term]
			idx.postings[term] = append(existing, PostingEntry{
				DocIndex: docIndex,
				TF:       freq,
			})
			idx.stats.TermDocs[term]++
		}
	}

	// Recalculate corpus statistics once after adding all chunks.
	idx.rebuildStats()
}

// rebuildStats recalculates N and AvgDocLen from the current doc set.
// Called after every AddDocument and after bulk operations.
func (idx *BM25Index) rebuildStats() {
	idx.stats.N = len(idx.docs)
	if idx.stats.N == 0 {
		idx.stats.AvgDocLen = 0
		return
	}
	total := 0
	for _, d := range idx.docs {
		total += d.len
	}
	idx.stats.AvgDocLen = float64(total) / float64(idx.stats.N)
}

// Search returns the top-K documents ranked by BM25 relevance for query.
//
// The query is tokenised with the same tokenizer used during indexing.
// Terms not present in the index contribute a score of 0 for all documents.
// Returns at most topK results; if topK <= 0 all matching documents are returned.
func (idx *BM25Index) Search(query string, topK int) []RankedResult {
	queryTerms := idx.tokenizer.Tokenize(query)
	if len(queryTerms) == 0 || idx.stats.N == 0 {
		return nil
	}

	// Accumulate BM25 scores in a map keyed by doc index.
	scores := make(map[int]float64, idx.stats.N)

	for _, term := range queryTerms {
		postings, ok := idx.postings[term]
		if !ok {
			continue // term not in corpus — contributes 0
		}

		// IDF = log((N - df + 0.5) / (df + 0.5) + 1)
		// The "+1" inside the log ensures IDF >= 0 for all df values.
		df := float64(idx.stats.TermDocs[term])
		N := float64(idx.stats.N)
		idf := math.Log((N-df+0.5)/(df+0.5) + 1)

		k1 := idx.params.K1
		b := idx.params.B
		avgDocLen := idx.stats.AvgDocLen
		if avgDocLen == 0 {
			avgDocLen = 1 // guard against empty corpus edge case
		}

		for _, pe := range postings {
			d := idx.docs[pe.DocIndex]
			docLen := float64(d.len)
			if docLen == 0 {
				docLen = 1 // guard against zero-length document
			}

			// BM25 TF component with length normalisation.
			// tf_norm = tf * (k1 + 1) / (tf + k1 * (1 - b + b * docLen / avgDocLen))
			tfF := float64(pe.TF)
			tfNorm := (tfF * (k1 + 1)) / (tfF + k1*(1-b+b*docLen/avgDocLen))

			scores[pe.DocIndex] += idf * tfNorm
		}
	}

	if len(scores) == 0 {
		return nil
	}

	// Convert map to slice for sorting.
	results := make([]RankedResult, 0, len(scores))
	for docIndex, score := range scores {
		d := idx.docs[docIndex]
		results = append(results, RankedResult{
			DocID:       d.id,
			Path:        d.path,
			RelPath:     d.relPath,
			Title:       d.title,
			Score:       score,
			HeadingPath: d.headingPath,
			StartLine:   d.startLine,
			EndLine:     d.endLine,
		})
	}

	// Sort by score descending; ties broken by DocID for determinism.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].DocID < results[j].DocID
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

// RemoveDocument removes the document with the given ID from the index.
// It rebuilds the affected posting lists and recalculates corpus statistics.
// Returns false if the document was not found.
func (idx *BM25Index) RemoveDocument(docID string) bool {
	// Find the doc index.
	docIndex := -1
	for i, d := range idx.docs {
		if d.id == docID {
			docIndex = i
			break
		}
	}
	if docIndex < 0 {
		return false
	}

	idx.removeAtIndex(docIndex)
	idx.rebuildStats()
	return true
}

// RemoveDocumentsByRelPath removes all chunks belonging to the document
// identified by relPath. This is required after chunk-level indexing, where a
// single file is represented by multiple index entries (one per section).
// Returns the number of chunks removed.
func (idx *BM25Index) RemoveDocumentsByRelPath(relPath string) int {
	// Collect all doc indices with this relPath (in descending order so
	// removal by index stays correct).
	var indices []int
	for i, d := range idx.docs {
		if d.relPath == relPath {
			indices = append(indices, i)
		}
	}
	if len(indices) == 0 {
		return 0
	}
	// Remove in reverse order so earlier indices remain stable.
	for i := len(indices) - 1; i >= 0; i-- {
		idx.removeAtIndex(indices[i])
	}
	idx.rebuildStats()
	return len(indices)
}

// removeAtIndex removes the indexedDoc at position i and updates posting lists.
// Does NOT call rebuildStats — callers are responsible for calling it once.
func (idx *BM25Index) removeAtIndex(docIndex int) {
	// Remove from posting lists.  We rebuild affected lists rather than
	// compacting indices, which keeps the implementation simple.
	for term, postings := range idx.postings {
		filtered := postings[:0]
		found := false
		for _, pe := range postings {
			if pe.DocIndex == docIndex {
				found = true
				continue
			}
			// Adjust indices for docs that come after the removed one.
			if pe.DocIndex > docIndex {
				pe.DocIndex--
			}
			filtered = append(filtered, pe)
		}
		if found {
			idx.stats.TermDocs[term]--
			if idx.stats.TermDocs[term] <= 0 {
				delete(idx.stats.TermDocs, term)
				delete(idx.postings, term)
			} else {
				idx.postings[term] = filtered
			}
		}
	}

	// Remove the doc from the docs slice.
	idx.docs = append(idx.docs[:docIndex], idx.docs[docIndex+1:]...)
}

// DocCount returns the number of documents currently indexed.
func (idx *BM25Index) DocCount() int { return len(idx.docs) }

// contentPreview returns the first maxRunes runes of content, trimmed of leading whitespace.
// Used for the content_preview field in chunk search results.
func contentPreview(content string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}
