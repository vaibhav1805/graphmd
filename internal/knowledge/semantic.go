package knowledge

import (
	"fmt"
	"math"
	"sort"
)

// SemanticRelationships computes pairwise document similarity using TF-IDF
// cosine similarity over the BM25 index postings. Documents whose similarity
// exceeds threshold are returned as EdgeRelated edges. The threshold should
// be in [0.0, 1.0]; a value of 0.35 works well for discovery mode.
//
// Complexity is O(n^2) in the number of unique file-level documents, which is
// acceptable for repositories of up to ~1000 files.
func SemanticRelationships(idx *BM25Index, threshold float64) []*Edge {
	if idx == nil || len(idx.docs) == 0 {
		return nil
	}

	matrix := buildTFIDFMatrix(idx)
	if len(matrix) < 2 {
		return nil
	}

	// Sorted doc IDs for deterministic output.
	docIDs := make([]string, 0, len(matrix))
	for id := range matrix {
		docIDs = append(docIDs, id)
	}
	sort.Strings(docIDs)

	var edges []*Edge

	for i := 0; i < len(docIDs); i++ {
		for j := i + 1; j < len(docIDs); j++ {
			sim := cosineSimilarity(matrix[docIDs[i]], matrix[docIDs[j]])
			if sim < threshold {
				continue
			}

			confidence := mapSimilarityToConfidence(sim)
			evidence := fmt.Sprintf("Semantic overlap: %.2f", sim)

			edge, err := NewEdge(docIDs[i], docIDs[j], EdgeRelated, confidence, evidence)
			if err != nil {
				continue
			}
			edges = append(edges, edge)
		}
	}

	return edges
}

// buildTFIDFMatrix constructs a TF-IDF vector for each unique file-level
// document from the BM25 index postings. Chunks belonging to the same file
// (same relPath) are merged by summing their term frequencies before computing
// TF-IDF, so that each file gets a single vector.
//
// TF = raw term frequency in the document.
// IDF = log((N - df + 0.5) / (df + 0.5) + 1), matching BM25 IDF.
//
// Returns a map from document relPath to its TF-IDF vector (term -> weight).
func buildTFIDFMatrix(idx *BM25Index) map[string]map[string]float64 {
	// Step 1: Merge chunk-level term frequencies per file.
	// fileTF[relPath][term] = total raw frequency across all chunks.
	fileTF := make(map[string]map[string]int)
	for _, doc := range idx.docs {
		relPath := doc.relPath
		if _, ok := fileTF[relPath]; !ok {
			fileTF[relPath] = make(map[string]int)
		}
	}

	for term, postings := range idx.postings {
		for _, pe := range postings {
			relPath := idx.docs[pe.DocIndex].relPath
			fileTF[relPath][term] += pe.TF
		}
	}

	// Step 2: Compute per-file document frequency (how many files contain term).
	fileDF := make(map[string]int)
	for _, tfMap := range fileTF {
		for term := range tfMap {
			fileDF[term]++
		}
	}

	N := float64(len(fileTF))

	// Step 3: Compute TF-IDF vectors.
	matrix := make(map[string]map[string]float64, len(fileTF))
	for relPath, tfMap := range fileTF {
		vec := make(map[string]float64, len(tfMap))
		for term, tf := range tfMap {
			df := float64(fileDF[term])
			idf := math.Log((N-df+0.5)/(df+0.5) + 1)
			vec[term] = float64(tf) * idf
		}
		matrix[relPath] = vec
	}

	return matrix
}

// cosineSimilarity computes the cosine similarity between two sparse TF-IDF
// vectors represented as maps. Returns 0.0 if either vector is zero-length.
//
// Uses sorted-key iteration for both vectors to ensure identical floating-point
// addition order across runs. Go's map iteration is randomized; without sorting,
// the sum order varies and IEEE-754 rounding produces different results for
// borderline pairs near the similarity threshold.
func cosineSimilarity(vec1, vec2 map[string]float64) float64 {
	if len(vec1) == 0 || len(vec2) == 0 {
		return 0.0
	}

	// Sort vec1 keys for deterministic mag1 and dot accumulation.
	keys1 := make([]string, 0, len(vec1))
	for term := range vec1 {
		keys1 = append(keys1, term)
	}
	sort.Strings(keys1)

	// Sort vec2 keys for deterministic mag2 accumulation.
	keys2 := make([]string, 0, len(vec2))
	for term := range vec2 {
		keys2 = append(keys2, term)
	}
	sort.Strings(keys2)

	var dot, mag1, mag2 float64

	for _, term := range keys1 {
		w1 := vec1[term]
		mag1 += w1 * w1
		if w2, ok := vec2[term]; ok {
			dot += w1 * w2
		}
	}

	for _, term := range keys2 {
		w2 := vec2[term]
		mag2 += w2 * w2
	}

	denom := math.Sqrt(mag1) * math.Sqrt(mag2)
	if denom == 0 {
		return 0.0
	}

	return dot / denom
}

// mapSimilarityToConfidence maps a cosine similarity in [0.35, 1.0] to a
// confidence value in [0.5, 0.75]. Values below 0.35 map to 0.5, values
// at or above 1.0 map to 0.75.
func mapSimilarityToConfidence(sim float64) float64 {
	const (
		simMin  = 0.35
		simMax  = 1.0
		confMin = 0.5
		confMax = 0.75
	)

	if sim <= simMin {
		return confMin
	}
	if sim >= simMax {
		return confMax
	}

	// Linear interpolation.
	t := (sim - simMin) / (simMax - simMin)
	return confMin + t*(confMax-confMin)
}
