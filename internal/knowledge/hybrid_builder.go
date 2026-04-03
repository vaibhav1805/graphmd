package knowledge

import (
	"fmt"
	"log"
	"strings"
)

// AggregationStrategy controls how multiple signals are combined into a
// single confidence score.
type AggregationStrategy int

const (
	// AggregationMax returns the highest individual signal confidence.
	// This is the default strategy (conservative, well-behaved).
	AggregationMax AggregationStrategy = iota

	// AggregationWeightedAverage computes a weighted average of all signal
	// confidences using Signal.Weight as the per-signal weight.
	AggregationWeightedAverage
)

// HybridBuilder merges a ComponentRegistry into an existing knowledge Graph,
// creating a hybrid graph where every edge carries aggregated confidence scores
// from multiple signal sources (link, mention, LLM).
//
// The merge is additive and non-destructive:
//   - Existing edges whose confidence is lower than the aggregated signal are
//     updated to the higher value.
//   - New edges discovered in the registry are added to the graph.
//   - Existing edges with no corresponding registry signal are left unchanged.
//
// Zero value is not valid — always use NewHybridBuilder.
type HybridBuilder struct {
	// Strategy controls how multiple signals are merged into one confidence.
	Strategy AggregationStrategy

	// MinConfidence is the threshold below which individual signals are
	// ignored during aggregation.  Default: 0.5.
	MinConfidence float64

	// Verbose enables diagnostic log output on stderr for skipped edges.
	Verbose bool
}

// NewHybridBuilder creates a HybridBuilder with production defaults:
// AggregationMax strategy, MinConfidence 0.5.
func NewHybridBuilder() *HybridBuilder {
	return &HybridBuilder{
		Strategy:      AggregationMax,
		MinConfidence: 0.5,
	}
}

// AggregateSignals merges a slice of Signal values into a single confidence
// score using the builder's configured Strategy.
//
// Signals below the MinConfidence threshold are excluded.
// Returns 0.0 when no signals survive the threshold filter.
func (h *HybridBuilder) AggregateSignals(signals []Signal) float64 {
	if len(signals) == 0 {
		return 0.0
	}

	// Filter by threshold.
	var valid []Signal
	for _, s := range signals {
		if s.Confidence >= h.MinConfidence {
			valid = append(valid, s)
		}
	}
	if len(valid) == 0 {
		return 0.0
	}

	switch h.Strategy {
	case AggregationWeightedAverage:
		return aggregateWeightedAverage(valid)
	default:
		return aggregateMax(valid)
	}
}

// aggregateMax returns max(signal.Confidence * signal.Weight), capped at 1.0.
func aggregateMax(signals []Signal) float64 {
	best := 0.0
	for _, s := range signals {
		w := s.Weight
		if w <= 0 {
			w = 1.0
		}
		score := s.Confidence * w
		if score > best {
			best = score
		}
	}
	if best > 1.0 {
		return 1.0
	}
	return best
}

// aggregateWeightedAverage returns the weighted mean of all signal confidences,
// using Signal.Weight. Signals with weight <= 0 receive weight 1.0.
func aggregateWeightedAverage(signals []Signal) float64 {
	totalWeight := 0.0
	weightedSum := 0.0
	for _, s := range signals {
		w := s.Weight
		if w <= 0 {
			w = 1.0
		}
		weightedSum += s.Confidence * w
		totalWeight += w
	}
	if totalWeight == 0 {
		return 0.0
	}
	avg := weightedSum / totalWeight
	if avg > 1.0 {
		return 1.0
	}
	return avg
}

// MergeEdgeConfidences returns the updated confidence for an existing graph
// edge after incorporating the provided signals.
//
// The existing edge's confidence is treated as one additional signal
// (using SignalLink type, weight 1.0) so that prior evidence is never lost.
// Returns the new aggregated confidence.
func (h *HybridBuilder) MergeEdgeConfidences(existing *Edge, signals []Signal) float64 {
	// Start with the edge's existing confidence as one signal.
	combined := append([]Signal{
		{
			SourceType: SignalLink,
			Confidence: existing.Confidence,
			Weight:     1.0,
		},
	}, signals...)
	return h.AggregateSignals(combined)
}

// BuildHybridGraph merges registry signals into baseGraph, producing an
// augmented graph.
//
// The returned *Graph is the same pointer as baseGraph — the merge is
// performed in place and baseGraph is also returned for convenience.
//
// When registry is nil, baseGraph is returned unmodified.
//
// For each RegistryRelationship in registry:
//  1. Map component IDs to graph node IDs via component FileRef fields.
//  2. If a matching graph edge exists: update its Confidence if the
//     aggregated signal confidence is higher.
//  3. If no matching graph edge exists: add a new EdgeMentions edge with
//     the aggregated confidence.
//  4. Unresolvable targets (no matching graph node) are logged and skipped.
func (h *HybridBuilder) BuildHybridGraph(registry *ComponentRegistry, baseGraph *Graph) *Graph {
	if registry == nil {
		return baseGraph
	}

	// Build a lookup from component ID → node ID (graph node IDs are file paths).
	compToNode := buildComponentToNodeMap(registry, baseGraph)

	for _, rel := range registry.Relationships {
		fromNode, fromOK := compToNode[rel.FromComponent]
		toNode, toOK := compToNode[rel.ToComponent]

		if !fromOK {
			if h.Verbose {
				log.Printf("hybrid: skip relationship %q→%q: source component not found in graph\n",
					rel.FromComponent, rel.ToComponent)
			}
			continue
		}
		if !toOK {
			if h.Verbose {
				log.Printf("hybrid: skip relationship %q→%q: target component not found in graph\n",
					rel.FromComponent, rel.ToComponent)
			}
			continue
		}

		// Skip self-loops (can arise from component-to-component mapping).
		if fromNode == toNode {
			continue
		}

		aggregated := h.AggregateSignals(rel.Signals)
		if aggregated <= 0 {
			continue
		}

		h.mergeIntoGraph(baseGraph, fromNode, toNode, aggregated, rel.Signals)
	}

	return baseGraph
}

// mergeIntoGraph updates or creates an edge between fromNode and toNode in g.
//
// If an edge already exists (any EdgeType), the confidence is updated when
// the aggregated value is higher. Otherwise a new EdgeMentions edge is created.
func (h *HybridBuilder) mergeIntoGraph(g *Graph, fromNode, toNode string, aggregated float64, signals []Signal) {
	// Check all existing edges between fromNode and toNode.
	// Pick any edge that connects these two nodes to update.
	var existingEdge *Edge
	for _, e := range g.BySource[fromNode] {
		if e.Target == toNode {
			existingEdge = e
			break
		}
	}

	if existingEdge != nil {
		// Update confidence if aggregated signals give a higher value.
		updated := h.MergeEdgeConfidences(existingEdge, signals)
		if updated > existingEdge.Confidence {
			existingEdge.Confidence = updated
		}
		return
	}

	// No existing edge — create a new one from registry signals.
	evidence := buildEvidenceSummary(signals)
	newEdge, err := NewEdge(fromNode, toNode, EdgeMentions, aggregated, evidence)
	if err != nil {
		if h.Verbose {
			log.Printf("hybrid: cannot create edge %q→%q: %v\n", fromNode, toNode, err)
		}
		return
	}
	if err := g.AddEdge(newEdge); err != nil {
		if h.Verbose {
			log.Printf("hybrid: AddEdge %q→%q: %v\n", fromNode, toNode, err)
		}
	}
}

// UpdateEdgeConfidence updates the Confidence of the edge connecting fromNode
// to toNode (any EdgeType) to the given confidence value.
//
// Returns an error when no edge between the two nodes exists or the confidence
// is outside [0.0, 1.0].
func (g *Graph) UpdateEdgeConfidence(fromNode, toNode string, confidence float64) error {
	if confidence < 0.0 || confidence > 1.0 {
		return fmt.Errorf("knowledge.Graph.UpdateEdgeConfidence: confidence %.4f out of [0.0, 1.0]", confidence)
	}
	for _, e := range g.BySource[fromNode] {
		if e.Target == toNode {
			e.Confidence = confidence
			return nil
		}
	}
	return fmt.Errorf("knowledge.Graph.UpdateEdgeConfidence: no edge from %q to %q", fromNode, toNode)
}

// MergeRegistry augments the graph with signals from registry using the
// default HybridBuilder (AggregationMax, MinConfidence 0.5).
//
// This is a convenience method for callers that do not need custom aggregation
// settings.  When registry is nil, the method is a no-op.
func (g *Graph) MergeRegistry(registry *ComponentRegistry) error {
	if registry == nil {
		return nil
	}
	builder := NewHybridBuilder()
	builder.BuildHybridGraph(registry, g)
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// buildComponentToNodeMap builds a lookup from registry component ID to the
// corresponding graph node ID.
//
// Primary strategy: match component.FileRef directly to a graph node ID.
// Fallback strategy: match by filename stem (component ID == stem of node ID).
func buildComponentToNodeMap(registry *ComponentRegistry, g *Graph) map[string]string {
	m := make(map[string]string, len(registry.Components))

	for compID, comp := range registry.Components {
		// Primary: exact FileRef match.
		if _, ok := g.Nodes[comp.FileRef]; ok {
			m[compID] = comp.FileRef
			continue
		}

		// Fallback: match component ID against node ID stem.
		matched := findNodeByStem(g, compID)
		if matched != "" {
			m[compID] = matched
		}
	}
	return m
}

// findNodeByStem scans graph nodes for one whose filename stem (without
// extension) matches compID (case-insensitive).
func findNodeByStem(g *Graph, compID string) string {
	lower := strings.ToLower(compID)
	for nodeID := range g.Nodes {
		// nodeID is a relative path like "services/auth.md"
		// Extract stem: "auth"
		base := nodeID
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		if idx := strings.LastIndex(base, "."); idx >= 0 {
			base = base[:idx]
		}
		if strings.ToLower(base) == lower {
			return nodeID
		}
	}
	return ""
}

// buildEvidenceSummary constructs a brief human-readable evidence string from
// the signals backing a relationship.
func buildEvidenceSummary(signals []Signal) string {
	if len(signals) == 0 {
		return "registry signal"
	}
	types := make([]string, 0, len(signals))
	seen := make(map[string]bool)
	for _, s := range signals {
		t := string(s.SourceType)
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	if len(signals) == 1 && signals[0].Evidence != "" {
		return fmt.Sprintf("[%s] %s", types[0], signals[0].Evidence)
	}
	return fmt.Sprintf("[%s] %d signal(s)", strings.Join(types, "+"), len(signals))
}
