package knowledge

import (
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverRelationships runs all discovery algorithms (co-occurrence,
// structural patterns, and NER+SVO) on the given documents and returns a
// deduplicated, aggregated set of discovered edges.
//
// componentNames maps display names (e.g. "User Service") to document IDs
// (e.g. "services/user-service.md"). If nil, the function builds a name map
// from the documents' titles and file stems.
//
// The returned edges have their Signals fields populated with all supporting
// evidence, and Confidence set to the maximum signal confidence.
//
// Note: the semantic algorithm requires a BM25 index and is not included here.
// For full 4-algorithm orchestration with quality filtering and explicit-graph
// deduplication, use DiscoverAndIntegrateRelationships instead.
func DiscoverRelationships(documents []Document, componentNames map[string]string) []*DiscoveredEdge {
	if componentNames == nil {
		componentNames = BuildComponentNameMap(documents)
	}

	// Run co-occurrence, structural, and NER+SVO algorithms.
	coOccEdges := CoOccurrenceRelationships(documents, componentNames, DefaultCoOccurrenceConfig())
	structEdges := StructuralRelationships(documents, componentNames)
	nerEdges := convertNERToDiscovered(NERRelationships(documents))

	// Merge and deduplicate.
	return MergeDiscoveredEdges(coOccEdges, structEdges, nerEdges)
}

// MergeDiscoveredEdges deduplicates edges from multiple sources.
//
// For edges with the same source+target+type, signals are aggregated and the
// highest confidence is kept. For edges with the same source+target but
// different types, both are kept (they represent different relationship kinds).
//
// Results are sorted by source+target+type for deterministic output across runs.
func MergeDiscoveredEdges(edgeSets ...[]*DiscoveredEdge) []*DiscoveredEdge {
	// Key: source\x00target\x00type → accumulated DiscoveredEdge.
	merged := make(map[string]*DiscoveredEdge)

	for _, edges := range edgeSets {
		for _, de := range edges {
			key := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)

			if existing, ok := merged[key]; ok {
				// Aggregate signals.
				existing.Signals = append(existing.Signals, de.Signals...)
				// Keep the highest confidence.
				if de.Confidence > existing.Confidence {
					existing.Confidence = de.Confidence
					existing.Evidence = de.Evidence
				}
			} else {
				// Clone to avoid mutation.
				clone := &DiscoveredEdge{
					Edge:    de.Edge,
					Signals: make([]Signal, len(de.Signals)),
				}
				copy(clone.Signals, de.Signals)
				merged[key] = clone
			}
		}
	}

	// Collect and sort results for deterministic output.
	type edgeWithKey struct {
		key string
		de  *DiscoveredEdge
	}
	var sorted []edgeWithKey
	for key, de := range merged {
		sorted = append(sorted, edgeWithKey{key, de})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].key < sorted[j].key
	})

	results := make([]*DiscoveredEdge, 0, len(sorted))
	for _, ek := range sorted {
		results = append(results, ek.de)
	}

	return results
}

// BuildComponentNameMap creates a mapping from display names and filename stems
// to document IDs. Multiple name variants are generated for each document to
// improve matching:
//   - Document title (e.g. "User Service")
//   - Filename stem (e.g. "user-service")
//   - Filename stem with dashes replaced by spaces (e.g. "user service")
func BuildComponentNameMap(documents []Document) map[string]string {
	names := make(map[string]string, len(documents)*3)

	for _, doc := range documents {
		// Title → ID
		if doc.Title != "" {
			names[doc.Title] = doc.ID
		}

		// Filename stem → ID
		stem := filenameStem(doc.RelPath)
		if stem != "" {
			names[stem] = doc.ID
		}

		// Stem with dashes/underscores as spaces → ID
		spacedStem := strings.ReplaceAll(stem, "-", " ")
		spacedStem = strings.ReplaceAll(spacedStem, "_", " ")
		if spacedStem != stem && spacedStem != "" {
			names[spacedStem] = doc.ID
		}

		// Also add directory-qualified stem for disambiguated matching.
		dir := filepath.Dir(doc.RelPath)
		if dir != "." && dir != "" {
			qualifiedName := filepath.Base(dir) + "/" + stem
			names[qualifiedName] = doc.ID
		}
	}

	return names
}

// AddDiscoveredEdgesToRegistry adds discovered edges to a ComponentRegistry,
// converting each DiscoveredEdge into registry signals.
func AddDiscoveredEdgesToRegistry(registry *ComponentRegistry, edges []*DiscoveredEdge) {
	for _, de := range edges {
		fromID := nodeToRegistryID(de.Source)
		toID := nodeToRegistryID(de.Target)

		for _, sig := range de.Signals {
			_ = registry.AddSignal(fromID, toID, sig)
		}
	}
	registry.AggregateConfidence()
}
