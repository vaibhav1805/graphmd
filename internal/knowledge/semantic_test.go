package knowledge

import (
	"math"
	"testing"
)

// --- cosineSimilarity tests --------------------------------------------------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	vec := map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}
	sim := cosineSimilarity(vec, vec)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("identical vectors: got %.6f, want 1.0", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	v1 := map[string]float64{"a": 1.0, "b": 0.0}
	v2 := map[string]float64{"c": 1.0, "d": 0.0}
	sim := cosineSimilarity(v1, v2)
	if sim != 0.0 {
		t.Errorf("orthogonal vectors: got %.6f, want 0.0", sim)
	}
}

func TestCosineSimilarity_KnownValue(t *testing.T) {
	// vec1 = (1, 2, 3), vec2 = (4, 5, 6)
	// dot = 4+10+18 = 32
	// |v1| = sqrt(14), |v2| = sqrt(77)
	// cos = 32 / sqrt(14*77) = 32 / sqrt(1078) ~= 0.9746
	v1 := map[string]float64{"a": 1.0, "b": 2.0, "c": 3.0}
	v2 := map[string]float64{"a": 4.0, "b": 5.0, "c": 6.0}
	sim := cosineSimilarity(v1, v2)
	expected := 32.0 / math.Sqrt(14.0*77.0)
	if math.Abs(sim-expected) > 1e-6 {
		t.Errorf("known value: got %.6f, want %.6f", sim, expected)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	empty := map[string]float64{}
	nonempty := map[string]float64{"a": 1.0}

	if sim := cosineSimilarity(empty, nonempty); sim != 0.0 {
		t.Errorf("empty v1: got %.6f, want 0.0", sim)
	}
	if sim := cosineSimilarity(nonempty, empty); sim != 0.0 {
		t.Errorf("empty v2: got %.6f, want 0.0", sim)
	}
	if sim := cosineSimilarity(empty, empty); sim != 0.0 {
		t.Errorf("both empty: got %.6f, want 0.0", sim)
	}
}

func TestCosineSimilarity_PartialOverlap(t *testing.T) {
	v1 := map[string]float64{"a": 1.0, "b": 2.0}
	v2 := map[string]float64{"b": 3.0, "c": 4.0}
	// dot = 0 + 6 + 0 = 6
	// |v1| = sqrt(1+4) = sqrt(5)
	// |v2| = sqrt(9+16) = sqrt(25) = 5
	// cos = 6 / (sqrt(5) * 5) = 6 / (5*sqrt(5)) ~= 0.5367
	sim := cosineSimilarity(v1, v2)
	expected := 6.0 / (math.Sqrt(5.0) * 5.0)
	if math.Abs(sim-expected) > 1e-6 {
		t.Errorf("partial overlap: got %.6f, want %.6f", sim, expected)
	}
}

// --- mapSimilarityToConfidence tests -----------------------------------------

func TestMapSimilarityToConfidence_BelowThreshold(t *testing.T) {
	conf := mapSimilarityToConfidence(0.1)
	if conf != 0.5 {
		t.Errorf("below threshold: got %.4f, want 0.5", conf)
	}
}

func TestMapSimilarityToConfidence_AtThreshold(t *testing.T) {
	conf := mapSimilarityToConfidence(0.35)
	if conf != 0.5 {
		t.Errorf("at threshold: got %.4f, want 0.5", conf)
	}
}

func TestMapSimilarityToConfidence_AtMax(t *testing.T) {
	conf := mapSimilarityToConfidence(1.0)
	if conf != 0.75 {
		t.Errorf("at max: got %.4f, want 0.75", conf)
	}
}

func TestMapSimilarityToConfidence_Midpoint(t *testing.T) {
	// sim=0.675 is midpoint of [0.35, 1.0]
	conf := mapSimilarityToConfidence(0.675)
	expected := 0.625 // midpoint of [0.5, 0.75]
	if math.Abs(conf-expected) > 1e-6 {
		t.Errorf("midpoint: got %.4f, want %.4f", conf, expected)
	}
}

// --- buildTFIDFMatrix tests --------------------------------------------------

func TestBuildTFIDFMatrix_EmptyIndex(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	matrix := buildTFIDFMatrix(idx)
	if len(matrix) != 0 {
		t.Errorf("empty index: got %d docs, want 0", len(matrix))
	}
}

func TestBuildTFIDFMatrix_SingleDoc(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	idx.AddDocument(Document{
		ID:      "doc1.md",
		RelPath: "doc1.md",
		Content: "hello world hello",
	})
	matrix := buildTFIDFMatrix(idx)
	if len(matrix) != 1 {
		t.Fatalf("single doc: got %d docs, want 1", len(matrix))
	}
	vec, ok := matrix["doc1.md"]
	if !ok {
		t.Fatal("doc1.md not in matrix")
	}
	if len(vec) == 0 {
		t.Error("vector is empty")
	}
}

func TestBuildTFIDFMatrix_MergesChunks(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	// A document with headings will produce multiple chunks with the same relPath.
	idx.AddDocument(Document{
		ID:      "doc1.md",
		RelPath: "doc1.md",
		Content: "# Section A\nfoo bar baz\n# Section B\nfoo qux",
	})
	matrix := buildTFIDFMatrix(idx)
	// Even though there are multiple chunks, they should merge into one relPath.
	if len(matrix) != 1 {
		t.Errorf("merged chunks: got %d docs, want 1", len(matrix))
	}
}

func TestBuildTFIDFMatrix_TwoDocs_IDFDiffers(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	idx.AddDocument(Document{
		ID:      "a.md",
		RelPath: "a.md",
		Content: "alpha beta gamma",
	})
	idx.AddDocument(Document{
		ID:      "b.md",
		RelPath: "b.md",
		Content: "alpha delta epsilon",
	})

	matrix := buildTFIDFMatrix(idx)
	if len(matrix) != 2 {
		t.Fatalf("two docs: got %d, want 2", len(matrix))
	}

	// "alpha" appears in both docs, so its IDF should be lower than "beta"
	// which appears only in doc a.
	aVec := matrix["a.md"]
	alphaWeight := aVec["alpha"]
	betaWeight := aVec["beta"]
	if betaWeight <= alphaWeight {
		t.Errorf("expected unique term 'beta' (%.4f) to have higher TF-IDF than shared 'alpha' (%.4f)",
			betaWeight, alphaWeight)
	}
}

// --- SemanticRelationships tests ---------------------------------------------

func TestSemanticRelationships_NilIndex(t *testing.T) {
	edges := SemanticRelationships(nil, 0.35)
	if edges != nil {
		t.Errorf("nil index: got %d edges, want nil", len(edges))
	}
}

func TestSemanticRelationships_SingleDoc(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	idx.AddDocument(Document{
		ID:      "doc1.md",
		RelPath: "doc1.md",
		Content: "hello world",
	})
	edges := SemanticRelationships(idx, 0.35)
	if len(edges) != 0 {
		t.Errorf("single doc: got %d edges, want 0", len(edges))
	}
}

func TestSemanticRelationships_SimilarDocs(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	// Two documents sharing many terms. Use longer content so tokenization
	// preserves enough overlapping terms after stop-word removal.
	idx.AddDocument(Document{
		ID:      "a.md",
		RelPath: "a.md",
		Content: "microservice architecture deployment kubernetes docker container orchestration cluster scaling monitoring observability",
	})
	idx.AddDocument(Document{
		ID:      "b.md",
		RelPath: "b.md",
		Content: "microservice deployment kubernetes container orchestration cluster scaling monitoring logging tracing",
	})
	// Also add a third unrelated doc to increase IDF discrimination.
	idx.AddDocument(Document{
		ID:      "c.md",
		RelPath: "c.md",
		Content: "cooking recipe pasta sauce tomato garlic olive pepper salt basil parsley oregano",
	})

	edges := SemanticRelationships(idx, 0.15)

	// a.md and b.md should have a semantic relationship.
	foundAB := false
	for _, e := range edges {
		if (e.Source == "a.md" && e.Target == "b.md") || (e.Source == "b.md" && e.Target == "a.md") {
			foundAB = true
			if e.Type != EdgeRelated {
				t.Errorf("edge type: got %q, want %q", e.Type, EdgeRelated)
			}
			if e.Confidence < 0.5 || e.Confidence > 0.75 {
				t.Errorf("confidence %.4f outside expected [0.5, 0.75]", e.Confidence)
			}
		}
	}
	if !foundAB {
		t.Error("similar docs a.md and b.md: expected a semantic edge, got none")
	}
}

func TestSemanticRelationships_DissimilarDocs(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	// Two completely different documents should not produce edges at high threshold.
	idx.AddDocument(Document{
		ID:      "a.md",
		RelPath: "a.md",
		Content: "mathematics algebra geometry calculus",
	})
	idx.AddDocument(Document{
		ID:      "b.md",
		RelPath: "b.md",
		Content: "cooking recipe pasta sauce tomato",
	})
	edges := SemanticRelationships(idx, 0.5)
	if len(edges) != 0 {
		t.Errorf("dissimilar docs: got %d edges, want 0", len(edges))
	}
}

func TestSemanticRelationships_ThresholdFiltering(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	idx.AddDocument(Document{
		ID:      "a.md",
		RelPath: "a.md",
		Content: "database query optimization index performance",
	})
	idx.AddDocument(Document{
		ID:      "b.md",
		RelPath: "b.md",
		Content: "database schema migration index upgrade",
	})
	idx.AddDocument(Document{
		ID:      "c.md",
		RelPath: "c.md",
		Content: "frontend react component styling layout",
	})

	lowThreshold := SemanticRelationships(idx, 0.1)
	highThreshold := SemanticRelationships(idx, 0.9)

	if len(highThreshold) > len(lowThreshold) {
		t.Errorf("higher threshold should produce fewer edges: low=%d high=%d",
			len(lowThreshold), len(highThreshold))
	}
}

func TestSemanticRelationships_EdgeEvidence(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	idx.AddDocument(Document{
		ID:      "a.md",
		RelPath: "a.md",
		Content: "service authentication authorization security",
	})
	idx.AddDocument(Document{
		ID:      "b.md",
		RelPath: "b.md",
		Content: "service authentication token security oauth",
	})
	edges := SemanticRelationships(idx, 0.2)
	for _, e := range edges {
		if e.Evidence == "" {
			t.Error("edge evidence should not be empty")
		}
		if len(e.Evidence) < 10 {
			t.Errorf("evidence too short: %q", e.Evidence)
		}
	}
}

func TestSemanticRelationships_NoSelfLoops(t *testing.T) {
	idx := NewBM25Index(DefaultBM25Params(), nil)
	idx.AddDocument(Document{
		ID:      "a.md",
		RelPath: "a.md",
		Content: "hello world foo bar",
	})
	idx.AddDocument(Document{
		ID:      "b.md",
		RelPath: "b.md",
		Content: "hello world baz qux",
	})

	edges := SemanticRelationships(idx, 0.0)
	for _, e := range edges {
		if e.Source == e.Target {
			t.Errorf("self-loop found: %s -> %s", e.Source, e.Target)
		}
	}
}

func TestSemanticRelationships_Deterministic(t *testing.T) {
	makeIndex := func() *BM25Index {
		idx := NewBM25Index(DefaultBM25Params(), nil)
		idx.AddDocument(Document{ID: "a.md", RelPath: "a.md", Content: "alpha beta gamma"})
		idx.AddDocument(Document{ID: "b.md", RelPath: "b.md", Content: "alpha beta delta"})
		idx.AddDocument(Document{ID: "c.md", RelPath: "c.md", Content: "epsilon zeta eta"})
		return idx
	}

	edges1 := SemanticRelationships(makeIndex(), 0.2)
	edges2 := SemanticRelationships(makeIndex(), 0.2)

	if len(edges1) != len(edges2) {
		t.Fatalf("non-deterministic count: %d vs %d", len(edges1), len(edges2))
	}
	for i := range edges1 {
		if edges1[i].Source != edges2[i].Source || edges1[i].Target != edges2[i].Target {
			t.Errorf("non-deterministic at %d: (%s,%s) vs (%s,%s)",
				i, edges1[i].Source, edges1[i].Target, edges2[i].Source, edges2[i].Target)
		}
	}
}
