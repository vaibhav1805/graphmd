package knowledge

import (
	"testing"
)

func TestDiscoveryAggregateSignals_Empty(t *testing.T) {
	result := AggregateSignals(nil)
	if result != nil {
		t.Errorf("nil input: got %d results, want nil", len(result))
	}

	result = AggregateSignals([]DiscoverySignal{})
	if result != nil {
		t.Errorf("empty input: got %d results, want nil", len(result))
	}
}

func TestDiscoveryAggregateSignals_SingleSignal(t *testing.T) {
	signals := []DiscoverySignal{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.6,
			Evidence:   "Semantic overlap: 0.55",
			Algorithm:  "semantic",
		},
	}
	result := AggregateSignals(signals)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	if result[0].Source != "a.md" || result[0].Target != "b.md" {
		t.Errorf("pair: got (%s,%s), want (a.md,b.md)", result[0].Source, result[0].Target)
	}
	if result[0].Confidence != 0.6 {
		t.Errorf("confidence: got %.2f, want 0.6", result[0].Confidence)
	}
	if len(result[0].Signals) != 1 {
		t.Errorf("signals: got %d, want 1", len(result[0].Signals))
	}
}

func TestDiscoveryAggregateSignals_MultipleSignalsSamePair(t *testing.T) {
	signals := []DiscoverySignal{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.5,
			Evidence:   "Semantic overlap: 0.40",
			Algorithm:  "semantic",
		},
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeMentions,
			Confidence: 0.7,
			Evidence:   "Co-occurrence: 5 shared terms",
			Algorithm:  "cooccurrence",
		},
	}
	result := AggregateSignals(signals)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	// Should pick the higher confidence (0.7 from cooccurrence).
	if result[0].Confidence != 0.7 {
		t.Errorf("confidence: got %.2f, want 0.7", result[0].Confidence)
	}
	if result[0].Type != EdgeMentions {
		t.Errorf("type: got %s, want %s", result[0].Type, EdgeMentions)
	}
	if len(result[0].Signals) != 2 {
		t.Errorf("signals: got %d, want 2", len(result[0].Signals))
	}
}

func TestDiscoveryAggregateSignals_MultiplePairs(t *testing.T) {
	signals := []DiscoverySignal{
		{Source: "a.md", Target: "b.md", Type: EdgeRelated, Confidence: 0.6, Algorithm: "semantic"},
		{Source: "b.md", Target: "c.md", Type: EdgeRelated, Confidence: 0.8, Algorithm: "semantic"},
		{Source: "a.md", Target: "c.md", Type: EdgeRelated, Confidence: 0.4, Algorithm: "semantic"},
	}
	result := AggregateSignals(signals)
	if len(result) != 3 {
		t.Fatalf("got %d results, want 3", len(result))
	}
	// Should be sorted by confidence descending.
	if result[0].Confidence < result[1].Confidence || result[1].Confidence < result[2].Confidence {
		t.Errorf("not sorted by confidence: %.2f, %.2f, %.2f",
			result[0].Confidence, result[1].Confidence, result[2].Confidence)
	}
}

func TestDiscoveryAggregateSignals_DirectionalPairsAreDistinct(t *testing.T) {
	// (a->b) and (b->a) are different directed pairs.
	signals := []DiscoverySignal{
		{Source: "a.md", Target: "b.md", Type: EdgeDependsOn, Confidence: 0.7, Algorithm: "structural"},
		{Source: "b.md", Target: "a.md", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "structural"},
	}
	result := AggregateSignals(signals)
	if len(result) != 2 {
		t.Errorf("directional pairs should be distinct: got %d, want 2", len(result))
	}
}

func TestEdgeSignalsFromSemantic(t *testing.T) {
	edges := []*Edge{
		{Source: "a.md", Target: "b.md", Type: EdgeRelated, Confidence: 0.6, Evidence: "Semantic overlap: 0.50"},
	}
	signals := EdgeSignalsFromSemantic(edges)
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if signals[0].Algorithm != "semantic" {
		t.Errorf("algorithm: got %q, want %q", signals[0].Algorithm, "semantic")
	}
	if signals[0].Source != "a.md" {
		t.Errorf("source: got %q, want %q", signals[0].Source, "a.md")
	}
}

func TestEdgeSignalsFromAlgorithm(t *testing.T) {
	edges := []*Edge{
		{Source: "x.md", Target: "y.md", Type: EdgeMentions, Confidence: 0.65, Evidence: "shared terms"},
	}
	signals := EdgeSignalsFromAlgorithm(edges, "cooccurrence")
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if signals[0].Algorithm != "cooccurrence" {
		t.Errorf("algorithm: got %q, want %q", signals[0].Algorithm, "cooccurrence")
	}
}

func TestAggregatedToEdges(t *testing.T) {
	aggregated := []AggregatedEdge{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.65,
			Evidence:   "Semantic overlap: 0.55",
			Signals: []DiscoverySignal{
				{Source: "a.md", Target: "b.md", Algorithm: "semantic"},
				{Source: "a.md", Target: "b.md", Algorithm: "cooccurrence"},
			},
		},
	}
	edges := AggregatedToEdges(aggregated)
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].Source != "a.md" || edges[0].Target != "b.md" {
		t.Errorf("pair: got (%s,%s)", edges[0].Source, edges[0].Target)
	}
	// Evidence should note [2 signals].
	if edges[0].Evidence == "" {
		t.Error("evidence should not be empty")
	}
}

func TestAggregatedToEdges_SingleSignal_NoSuffix(t *testing.T) {
	aggregated := []AggregatedEdge{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.6,
			Evidence:   "Semantic overlap: 0.50",
			Signals:    []DiscoverySignal{{Algorithm: "semantic"}},
		},
	}
	edges := AggregatedToEdges(aggregated)
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	// Single signal should not have "[1 signals]" suffix.
	if edges[0].Evidence != "Semantic overlap: 0.50" {
		t.Errorf("evidence: got %q, want %q", edges[0].Evidence, "Semantic overlap: 0.50")
	}
}

func TestAggregatedToEdges_SkipsInvalid(t *testing.T) {
	aggregated := []AggregatedEdge{
		{
			Source:     "a.md",
			Target:     "a.md", // self-loop, should be skipped
			Type:       EdgeRelated,
			Confidence: 0.6,
			Evidence:   "bad",
			Signals:    []DiscoverySignal{{Algorithm: "test"}},
		},
	}
	edges := AggregatedToEdges(aggregated)
	if len(edges) != 0 {
		t.Errorf("self-loop should be skipped: got %d edges", len(edges))
	}
}
