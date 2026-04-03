package knowledge

import (
	"fmt"
	"sort"
)

// CrawlStats holds computed statistics for a knowledge graph, suitable for
// display in the crawl command output or JSON serialization for agent consumption.
type CrawlStats struct {
	// ComponentCount is the total number of nodes in the graph.
	ComponentCount int `json:"component_count"`

	// RelationshipCount is the total number of edges in the graph.
	RelationshipCount int `json:"relationship_count"`

	// QualityScore is the weighted average of all edge confidence scores,
	// expressed as a percentage (0-100). Zero when there are no edges.
	QualityScore float64 `json:"quality_score"`

	// ComponentsByType groups node IDs by their ComponentType.
	// Each slice is sorted alphabetically for deterministic output.
	ComponentsByType map[ComponentType][]string `json:"components_by_type"`

	// ConfidenceDistribution contains one entry per non-zero confidence tier,
	// ordered from highest confidence (explicit) to lowest (threshold).
	ConfidenceDistribution []TierStats `json:"confidence_distribution"`

	// QualityWarnings lists detected graph quality issues such as orphan nodes,
	// dangling edges, and weak-only components.
	QualityWarnings []QualityWarning `json:"quality_warnings"`
}

// TierStats describes the edge count and percentage for a single confidence tier.
type TierStats struct {
	// Tier is the confidence tier name.
	Tier ConfidenceTier `json:"tier"`

	// RangeLow is the lower bound of the tier's confidence range (inclusive).
	RangeLow float64 `json:"range_low"`

	// RangeHigh is the upper bound of the tier's confidence range (inclusive for
	// explicit, exclusive for all others).
	RangeHigh float64 `json:"range_high"`

	// Count is the number of edges in this tier.
	Count int `json:"count"`

	// Percentage is Count / total edges * 100.
	Percentage float64 `json:"percentage"`
}

// QualityWarning describes a detected quality issue in the graph.
type QualityWarning struct {
	// Type identifies the warning category: "orphan_node", "dangling_edge", or "weak_only".
	Type string `json:"type"`

	// Message is a human-readable description including the count of affected items.
	Message string `json:"message"`

	// Items lists the affected node or edge IDs, sorted alphabetically.
	Items []string `json:"items"`
}

// tierBounds returns the confidence score boundaries for each tier, derived
// from the same thresholds used by ScoreToTier in confidence.go.
func tierBounds() map[ConfidenceTier][2]float64 {
	return map[ConfidenceTier][2]float64{
		TierExplicit:        {0.95, 1.0},
		TierStrongInference: {0.75, 0.95},
		TierModerate:        {0.55, 0.75},
		TierWeak:            {0.45, 0.55},
		TierSemantic:        {0.42, 0.45},
		TierThreshold:       {0.40, 0.42},
	}
}

// ComputeCrawlStats analyzes a Graph and returns comprehensive statistics
// including component counts, quality score, confidence distribution,
// component grouping by type, and quality warnings.
func ComputeCrawlStats(g *Graph) CrawlStats {
	stats := CrawlStats{
		ComponentCount:    len(g.Nodes),
		RelationshipCount: len(g.Edges),
		ComponentsByType:  make(map[ComponentType][]string),
	}

	if len(g.Nodes) == 0 && len(g.Edges) == 0 {
		stats.ComponentsByType = nil
		return stats
	}

	stats.QualityScore = computeQualityScore(g)
	stats.ComponentsByType = groupComponentsByType(g)
	stats.ConfidenceDistribution = computeConfidenceDistribution(g)
	stats.QualityWarnings = detectQualityWarnings(g)

	// Normalize empty maps/slices for clean output
	if len(stats.ComponentsByType) == 0 {
		stats.ComponentsByType = nil
	}

	return stats
}

// computeQualityScore returns the average edge confidence * 100.
// Returns 0 when there are no edges.
func computeQualityScore(g *Graph) float64 {
	if len(g.Edges) == 0 {
		return 0
	}
	var sum float64
	for _, e := range g.Edges {
		sum += e.Confidence
	}
	return sum / float64(len(g.Edges)) * 100
}

// groupComponentsByType groups node IDs by their ComponentType, with each
// slice sorted alphabetically for deterministic output.
func groupComponentsByType(g *Graph) map[ComponentType][]string {
	groups := make(map[ComponentType][]string)
	for _, node := range g.Nodes {
		groups[node.ComponentType] = append(groups[node.ComponentType], node.ID)
	}
	for ct := range groups {
		sort.Strings(groups[ct])
	}
	return groups
}

// computeConfidenceDistribution buckets edges by confidence tier using
// ScoreToTier, skipping zero-count tiers. Returns entries ordered from
// highest confidence (explicit) to lowest (threshold).
func computeConfidenceDistribution(g *Graph) []TierStats {
	if len(g.Edges) == 0 {
		return nil
	}

	counts := make(map[ConfidenceTier]int)
	for _, e := range g.Edges {
		tier := ScoreToTier(e.Confidence)
		counts[tier]++
	}

	bounds := tierBounds()
	total := float64(len(g.Edges))

	var result []TierStats
	// Iterate in canonical descending order
	for _, tier := range allConfidenceTiers {
		count, ok := counts[tier]
		if !ok || count == 0 {
			continue
		}
		b := bounds[tier]
		result = append(result, TierStats{
			Tier:       tier,
			RangeLow:   b[0],
			RangeHigh:  b[1],
			Count:      count,
			Percentage: float64(count) / total * 100,
		})
	}

	return result
}

// detectQualityWarnings inspects the graph for quality issues:
// - orphan_node: nodes with zero incoming AND zero outgoing edges
// - dangling_edge: edges referencing non-existent nodes
// - weak_only: nodes where ALL connected edges have confidence < 0.55
func detectQualityWarnings(g *Graph) []QualityWarning {
	var warnings []QualityWarning

	// Detect orphan nodes
	var orphans []string
	for id := range g.Nodes {
		incoming := g.ByTarget[id]
		outgoing := g.BySource[id]
		if len(incoming) == 0 && len(outgoing) == 0 {
			orphans = append(orphans, id)
		}
	}
	if len(orphans) > 0 {
		sort.Strings(orphans)
		warnings = append(warnings, QualityWarning{
			Type:    "orphan_node",
			Message: fmt.Sprintf("%d orphan node(s) with no connections", len(orphans)),
			Items:   orphans,
		})
	}

	// Detect dangling edges
	var dangling []string
	for _, e := range g.Edges {
		_, srcOK := g.Nodes[e.Source]
		_, tgtOK := g.Nodes[e.Target]
		if !srcOK || !tgtOK {
			dangling = append(dangling, e.ID)
		}
	}
	if len(dangling) > 0 {
		sort.Strings(dangling)
		warnings = append(warnings, QualityWarning{
			Type:    "dangling_edge",
			Message: fmt.Sprintf("%d edge(s) referencing non-existent nodes", len(dangling)),
			Items:   dangling,
		})
	}

	// Detect weak-only components: nodes where ALL connected edges have confidence < 0.55
	var weakOnly []string
	for id := range g.Nodes {
		incoming := g.ByTarget[id]
		outgoing := g.BySource[id]
		allEdges := append(incoming, outgoing...)
		if len(allEdges) == 0 {
			continue // orphans are already reported separately
		}
		allWeak := true
		for _, e := range allEdges {
			if e.Confidence >= 0.55 {
				allWeak = false
				break
			}
		}
		if allWeak {
			weakOnly = append(weakOnly, id)
		}
	}
	if len(weakOnly) > 0 {
		sort.Strings(weakOnly)
		warnings = append(warnings, QualityWarning{
			Type:    "weak_only",
			Message: fmt.Sprintf("%d component(s) with only weak connections (confidence < 0.55)", len(weakOnly)),
			Items:   weakOnly,
		})
	}

	if len(warnings) == 0 {
		return nil
	}
	return warnings
}
