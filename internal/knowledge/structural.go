package knowledge

import (
	"regexp"
	"sort"
	"strings"
)

// StructuralRelationships extracts relationships from the structural patterns
// in documents: heading hierarchy, dependency sections, and tables.
//
// Detection strategy:
//  1. Parse heading hierarchy to find sections whose names indicate
//     dependencies (e.g. "Dependencies", "Integration Points", "Requires").
//  2. Within those sections, extract component names from list items,
//     paragraphs, and table rows.
//  3. Create directed edges from the document to the mentioned components
//     with high confidence (0.75+).
//
// componentNames maps display name (e.g. "User Service") → document ID
// (e.g. "services/user-service.md").
func StructuralRelationships(documents []Document, componentNames map[string]string) []*DiscoveredEdge {
	var results []*DiscoveredEdge
	seen := make(map[string]bool)

	for _, doc := range documents {
		edges := structuralForDocument(&doc, componentNames)
		for _, de := range edges {
			key := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, de)
		}
	}

	return results
}

// structuralForDocument extracts structural relationships from a single document.
func structuralForDocument(doc *Document, componentNames map[string]string) []*DiscoveredEdge {
	sections := parseHeadingSections(doc.Content)
	var results []*DiscoveredEdge
	seen := make(map[string]bool)

	for _, section := range sections {
		if !isDependencySection(section.heading) {
			continue
		}

		edgeType := classifyHeadingToEdgeType(section.heading)

		// Extract component mentions from the section body.
		mentions := extractComponentMentions(section.body, componentNames)
		for _, mention := range mentions {
			if mention.docID == doc.ID {
				continue // skip self-references
			}

			key := doc.ID + "\x00" + mention.docID + "\x00" + string(edgeType)
			if seen[key] {
				continue
			}
			seen[key] = true

			confidence := 0.80
			// Boost confidence for explicit dependency sections.
			if edgeType == EdgeDependsOn {
				confidence = 0.85
			}

			edge, err := NewEdge(doc.ID, mention.docID, edgeType, confidence, mention.evidence)
			if err != nil {
				continue
			}
			results = append(results, &DiscoveredEdge{
				Edge: edge,
				Signals: []Signal{{
					SourceType: SignalStructural,
					Confidence: confidence,
					Evidence:   mention.evidence,
					Weight:     1.0,
				}},
			})
		}

		// Also try table-based extraction.
		tableEdges := extractFromTables(doc.ID, section.body, componentNames, edgeType)
		for _, de := range tableEdges {
			key := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, de)
		}
	}

	return results
}

// headingSection represents a section of a document bounded by a heading.
type headingSection struct {
	heading string // the heading text (without # prefix)
	level   int    // heading level (1-6)
	body    string // all content until the next heading of same or higher level
}

// headingRegex matches markdown heading lines.
var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// parseHeadingSections splits a markdown document into sections by heading.
func parseHeadingSections(content string) []headingSection {
	lines := strings.Split(content, "\n")
	var sections []headingSection
	var current *headingSection
	var bodyLines []string

	for _, line := range lines {
		m := headingRegex.FindStringSubmatch(line)
		if m != nil {
			// Save previous section.
			if current != nil {
				current.body = strings.Join(bodyLines, "\n")
				sections = append(sections, *current)
			}
			current = &headingSection{
				heading: strings.TrimSpace(m[2]),
				level:   len(m[1]),
			}
			bodyLines = nil
			continue
		}

		if current != nil {
			bodyLines = append(bodyLines, line)
		}
	}

	// Save the last section.
	if current != nil {
		current.body = strings.Join(bodyLines, "\n")
		sections = append(sections, *current)
	}

	return sections
}

// componentMention represents a component name found in section content.
type componentMention struct {
	name     string
	docID    string
	evidence string
}

// componentNamePair represents a name/docID pair for deterministic iteration.
type componentNamePair struct {
	name  string
	docID string
}

// sortedComponentNames returns component names sorted by name string for
// deterministic iteration. This prevents non-deterministic discovery when
// Go's randomized map iteration would pick different matches on each run.
func sortedComponentNames(componentNames map[string]string) []componentNamePair {
	var result []componentNamePair
	for name, docID := range componentNames {
		result = append(result, componentNamePair{name, docID})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}

// extractComponentMentions finds known component names in body text.
// Uses sorted component names for deterministic discovery across runs.
func extractComponentMentions(body string, componentNames map[string]string) []componentMention {
	var mentions []componentMention
	seen := make(map[string]bool)

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Iterate in sorted order for deterministic results
		for _, pair := range sortedComponentNames(componentNames) {
			name, docID := pair.name, pair.docID
			if seen[docID] {
				continue
			}
			// Case-insensitive search for component name in the line.
			if containsIgnoreCase(trimmed, name) {
				seen[docID] = true
				mentions = append(mentions, componentMention{
					name:     name,
					docID:    docID,
					evidence: truncateEvidence(trimmed, 120),
				})
			}
		}
	}

	return mentions
}

// containsIgnoreCase returns true if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// tableRowRegex matches markdown table rows: | cell1 | cell2 | ...
var tableRowRegex = regexp.MustCompile(`^\|(.+)\|$`)

// extractFromTables extracts relationships from markdown tables in dependency
// sections. Supports "Service | Depends On" format.
func extractFromTables(docID, body string, componentNames map[string]string, defaultEdgeType EdgeType) []*DiscoveredEdge {
	var results []*DiscoveredEdge
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := tableRowRegex.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		cells := strings.Split(m[1], "|")
		if len(cells) < 2 {
			continue
		}

		// Skip separator rows (---).
		isSeparator := true
		for _, cell := range cells {
			cleaned := strings.TrimSpace(cell)
			cleaned = strings.Trim(cleaned, "-: ")
			if cleaned != "" {
				isSeparator = false
				break
			}
		}
		if isSeparator {
			continue
		}

		// Check each cell for component name matches.
		for _, cell := range cells {
			cellText := strings.TrimSpace(cell)
			// Iterate in sorted order for deterministic results
			for _, pair := range sortedComponentNames(componentNames) {
				name, compDocID := pair.name, pair.docID
				if compDocID == docID {
					continue
				}
				if containsIgnoreCase(cellText, name) {
					evidence := truncateEvidence(trimmed, 120)
					edge, err := NewEdge(docID, compDocID, defaultEdgeType, 0.75, evidence)
					if err != nil {
						continue
					}
					results = append(results, &DiscoveredEdge{
						Edge: edge,
						Signals: []Signal{{
							SourceType: SignalStructural,
							Confidence: 0.75,
							Evidence:   evidence,
							Weight:     1.0,
						}},
					})
				}
			}
		}
	}

	return results
}
