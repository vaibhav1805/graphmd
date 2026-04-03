package knowledge

import (
	"sort"
	"strings"
)

// CoOccurrenceConfig holds parameters for the co-occurrence algorithm.
type CoOccurrenceConfig struct {
	// WindowSize is the number of lines in the sliding window (default: 5).
	WindowSize int
}

// DefaultCoOccurrenceConfig returns a CoOccurrenceConfig with sensible defaults.
func DefaultCoOccurrenceConfig() CoOccurrenceConfig {
	return CoOccurrenceConfig{WindowSize: 5}
}

// CoOccurrenceRelationships analyses documents for co-occurrences of component
// names within a sliding window of text. When two or more known names appear
// in the same paragraph-sized window, a candidate edge is created.
//
// Direction heuristic: the component that appears first in the window is the
// source; the one that appears second is the target.
//
// Confidence heuristic: mentions in early sections of a document receive
// higher confidence (0.55–0.65) than those in later sections (0.40–0.50).
//
// Returns all candidate edges (the caller should apply a confidence threshold).
func CoOccurrenceRelationships(documents []Document, componentNames map[string]string, cfg CoOccurrenceConfig) []*DiscoveredEdge {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 5
	}

	var results []*DiscoveredEdge
	seen := make(map[string]bool)

	for _, doc := range documents {
		edges := coOccurrenceForDocument(&doc, componentNames, cfg)
		for _, de := range edges {
			// Deduplicate across documents: keep the first (highest confidence)
			// occurrence.
			key := de.Source + "\x00" + de.Target
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, de)
		}
	}

	return results
}

// coOccurrenceForDocument applies the sliding window algorithm to a single
// document and returns discovered edges.
func coOccurrenceForDocument(doc *Document, componentNames map[string]string, cfg CoOccurrenceConfig) []*DiscoveredEdge {
	lines := strings.Split(doc.PlainText, "\n")
	totalLines := len(lines)
	if totalLines == 0 {
		return nil
	}

	var results []*DiscoveredEdge
	seen := make(map[string]bool)

	for i := 0; i <= totalLines-1; i++ {
		// Build the window text.
		end := i + cfg.WindowSize
		if end > totalLines {
			end = totalLines
		}
		window := strings.Join(lines[i:end], " ")
		windowLower := strings.ToLower(window)

		// Find all component names that appear in this window.
		// Use sorted component names for deterministic results across runs.
		var hits []coOccHit

		sortedNames := sortedComponentNamesCoOcc(componentNames)
		for _, pair := range sortedNames {
			name, compID := pair.name, pair.docID
			// Skip self-references: don't match the document's own component.
			if compID == doc.ID {
				continue
			}
			nameLower := strings.ToLower(name)
			pos := strings.Index(windowLower, nameLower)
			if pos >= 0 {
				hits = append(hits, coOccHit{compID: compID, pos: pos})
			}
		}

		if len(hits) < 1 {
			continue
		}

		// We need the document itself to also be a component, or we need at
		// least 2 different component hits to form an edge.
		// Strategy: if the document matches a component, pair the document
		// with each hit. Otherwise pair hits with each other.

		// Check if doc is a known component.
		docIsComponent := false
		for _, cID := range componentNames {
			if cID == doc.ID {
				docIsComponent = true
				break
			}
		}

		if docIsComponent && len(hits) >= 1 {
			// Pair doc (as source) with each hit (as target).
			for _, hit := range hits {
				if hit.compID == doc.ID {
					continue
				}
				key := doc.ID + "\x00" + hit.compID
				if seen[key] {
					continue
				}
				seen[key] = true

				confidence := coOccurrenceConfidence(i, totalLines)
				excerpt := truncateEvidence(window, 120)

				edge, err := NewEdge(doc.ID, hit.compID, EdgeMentions, confidence, excerpt)
				if err != nil {
					continue
				}
				results = append(results, &DiscoveredEdge{
					Edge: edge,
					Signals: []Signal{{
						SourceType: SignalCoOccurrence,
						Confidence: confidence,
						Evidence:   excerpt,
						Weight:     1.0,
					}},
				})
			}
		}

		if len(hits) >= 2 {
			// Sort hits by position to establish direction.
			sortHitsByPosition(hits)

			// Create edges between consecutive pairs.
			for j := 0; j < len(hits)-1; j++ {
				for k := j + 1; k < len(hits); k++ {
					src := hits[j].compID
					tgt := hits[k].compID
					if src == tgt {
						continue
					}

					key := src + "\x00" + tgt
					if seen[key] {
						continue
					}
					seen[key] = true

					confidence := coOccurrenceConfidence(i, totalLines)
					excerpt := truncateEvidence(window, 120)

					edge, err := NewEdge(src, tgt, EdgeMentions, confidence, excerpt)
					if err != nil {
						continue
					}
					results = append(results, &DiscoveredEdge{
						Edge: edge,
						Signals: []Signal{{
							SourceType: SignalCoOccurrence,
							Confidence: confidence,
							Evidence:   excerpt,
							Weight:     1.0,
						}},
					})
				}
			}
		}
	}

	return results
}

// coOccurrenceConfidence computes the confidence score based on the position
// of the window in the document. Earlier sections receive higher confidence.
func coOccurrenceConfidence(lineIdx, totalLines int) float64 {
	if totalLines <= 0 {
		return 0.50
	}
	position := float64(lineIdx) / float64(totalLines)

	// Early sections (top 30%) get higher confidence.
	switch {
	case position < 0.3:
		return 0.65
	case position < 0.6:
		return 0.55
	default:
		return 0.45
	}
}

// coOccHit is a component name match at a specific position in a text window.
type coOccHit struct {
	compID string
	pos    int
}

// sortHitsByPosition sorts coOccHit entries by their text position (ascending).
func sortHitsByPosition(hits []coOccHit) {
	// Simple insertion sort for small slices.
	for i := 1; i < len(hits); i++ {
		j := i
		for j > 0 && hits[j].pos < hits[j-1].pos {
			hits[j], hits[j-1] = hits[j-1], hits[j]
			j--
		}
	}
}

// truncateEvidence shortens s to maxLen characters, appending "..." if truncated.
func truncateEvidence(s string, maxLen int) string {
	// Collapse whitespace first.
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// sortedComponentNamesCoOcc returns component names sorted by name string for
// deterministic iteration. This prevents non-deterministic discovery when
// Go's randomized map iteration would pick different matches on each run.
func sortedComponentNamesCoOcc(componentNames map[string]string) []struct {
	name  string
	docID string
} {
	var result []struct {
		name  string
		docID string
	}
	for name, docID := range componentNames {
		result = append(result, struct {
			name  string
			docID string
		}{name, docID})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}
