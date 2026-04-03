package knowledge

import (
	"fmt"
	"sort"
)

// DiscoverySignal represents a single relationship signal from one discovery
// algorithm. Multiple signals for the same (source, target) pair are
// aggregated to determine the final edge.
type DiscoverySignal struct {
	Source     string
	Target     string
	Type       EdgeType
	Confidence float64
	Evidence   string
	Algorithm  string // e.g. "semantic", "cooccurrence", "structural"
}

// AggregatedEdge is the result of merging multiple DiscoverySignals for the
// same (source, target) pair.
type AggregatedEdge struct {
	Source     string
	Target     string
	Type       EdgeType
	Confidence float64
	Evidence   string
	Signals    []DiscoverySignal
}

// AggregateSignals merges a flat list of DiscoverySignals by (source, target)
// pair. For each pair, the signal with the highest confidence wins and
// determines the final edge type and evidence. All contributing signals are
// preserved for traceability.
//
// The result is sorted by confidence descending, then by source+target for
// determinism.
func AggregateSignals(signals []DiscoverySignal) []AggregatedEdge {
	if len(signals) == 0 {
		return nil
	}

	// Group signals by canonical (source, target) pair.
	// We use a directional key: source < target to normalize undirected pairs
	// (like semantic similarity) while preserving direction for directed signals.
	type pairKey struct{ source, target string }

	groups := make(map[pairKey][]DiscoverySignal)
	for _, sig := range signals {
		key := pairKey{source: sig.Source, target: sig.Target}
		groups[key] = append(groups[key], sig)
	}

	results := make([]AggregatedEdge, 0, len(groups))
	for key, sigs := range groups {
		// Find the highest-confidence signal.
		best := sigs[0]
		for _, sig := range sigs[1:] {
			if sig.Confidence > best.Confidence {
				best = sig
			}
		}

		results = append(results, AggregatedEdge{
			Source:     key.source,
			Target:     key.target,
			Type:       best.Type,
			Confidence: best.Confidence,
			Evidence:   best.Evidence,
			Signals:    sigs,
		})
	}

	// Sort for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Confidence != results[j].Confidence {
			return results[i].Confidence > results[j].Confidence
		}
		if results[i].Source != results[j].Source {
			return results[i].Source < results[j].Source
		}
		return results[i].Target < results[j].Target
	})

	return results
}

// EdgeSignalsFromSemantic converts semantic edges into DiscoverySignals
// tagged with the "semantic" algorithm name.
func EdgeSignalsFromSemantic(edges []*Edge) []DiscoverySignal {
	signals := make([]DiscoverySignal, 0, len(edges))
	for _, e := range edges {
		signals = append(signals, DiscoverySignal{
			Source:     e.Source,
			Target:     e.Target,
			Type:       e.Type,
			Confidence: e.Confidence,
			Evidence:   e.Evidence,
			Algorithm:  "semantic",
		})
	}
	return signals
}

// EdgeSignalsFromAlgorithm converts edges into DiscoverySignals with the
// given algorithm name.
func EdgeSignalsFromAlgorithm(edges []*Edge, algorithm string) []DiscoverySignal {
	signals := make([]DiscoverySignal, 0, len(edges))
	for _, e := range edges {
		signals = append(signals, DiscoverySignal{
			Source:     e.Source,
			Target:     e.Target,
			Type:       e.Type,
			Confidence: e.Confidence,
			Evidence:   e.Evidence,
			Algorithm:  algorithm,
		})
	}
	return signals
}

// AggregatedToEdges converts aggregated results back to Edge structs suitable
// for adding to a Graph.
func AggregatedToEdges(aggregated []AggregatedEdge) []*Edge {
	edges := make([]*Edge, 0, len(aggregated))
	for _, agg := range aggregated {
		evidence := agg.Evidence
		if len(agg.Signals) > 1 {
			evidence = fmt.Sprintf("%s [%d signals]", evidence, len(agg.Signals))
		}
		edge, err := NewEdge(agg.Source, agg.Target, agg.Type, agg.Confidence, evidence)
		if err != nil {
			continue
		}
		edges = append(edges, edge)
	}
	return edges
}
