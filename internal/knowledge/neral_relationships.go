package knowledge

import (
	"regexp"
	"sort"
	"strings"
)

// NERRelationships extracts directional edges from a collection of documents
// using Named Entity Recognition and Subject-Verb-Object pattern extraction.
//
// The algorithm proceeds in three phases:
//  1. Extract component names and build a registry (NER).
//  2. For each sentence in each document, extract SVO triples and match
//     subjects/objects to known components.
//  3. Aggregate duplicates, keeping the highest confidence per source/target pair.
//
// Returns edges with confidence in the range [0.65, 0.80].
func NERRelationships(documents []Document) []*Edge {
	if len(documents) == 0 {
		return nil
	}

	// Phase 1: Build component registry from NER.
	registry := BuildComponentRegistry(documents)
	if len(registry) == 0 {
		return nil
	}

	// Phase 2: Extract SVO triples and map to edges.
	type edgeKey struct {
		source string
		target string
	}
	bestEdges := make(map[edgeKey]*Edge)

	addEdge := func(edge *Edge) {
		key := edgeKey{source: edge.Source, target: edge.Target}
		if existing, ok := bestEdges[key]; ok {
			if edge.Confidence > existing.Confidence {
				bestEdges[key] = edge
			}
		} else {
			bestEdges[key] = edge
		}
	}

	for i := range documents {
		doc := &documents[i]

		// Phase 2a: SVO triple extraction from sentences.
		sentences := extractSentences(doc.Content)
		for _, sentence := range sentences {
			triples := ExtractSVOTriples(sentence)
			for _, triple := range triples {
				edge := tripleToEdge(triple, doc, registry, documents)
				if edge == nil {
					continue
				}
				addEdge(edge)
			}
		}

		// Phase 2b: Context-based extraction — if this document IS a component,
		// scan for lines that mention other components with relationship verbs.
		docComp := findComponentByFile(doc.ID, registry)
		if docComp == nil {
			continue
		}
		contextEdges := extractContextRelationships(doc, docComp, registry, documents)
		for _, edge := range contextEdges {
			addEdge(edge)
		}
	}

	// Collect and sort results for deterministic output.
	// Sort by source+target+type to ensure consistent ordering across runs.
	type edgeWithKey struct {
		key  string
		edge *Edge
	}
	var sorted []edgeWithKey
	for _, edge := range bestEdges {
		key := edge.Source + "\x00" + edge.Target + "\x00" + string(edge.Type)
		sorted = append(sorted, edgeWithKey{key, edge})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].key < sorted[j].key
	})

	result := make([]*Edge, 0, len(sorted))
	for _, ek := range sorted {
		result = append(result, ek.edge)
	}

	return result
}

// NERRelationshipsToRegistry extracts NER+SVO relationships and adds them to
// a ComponentRegistry as signals.
//
// This is the integration point for the hybrid graph builder pipeline.
func NERRelationshipsToRegistry(documents []Document, reg *ComponentRegistry) {
	if reg == nil || len(documents) == 0 {
		return
	}

	edges := NERRelationships(documents)
	for _, edge := range edges {
		fromID := nodeToRegistryID(edge.Source)
		toID := nodeToRegistryID(edge.Target)

		signal := Signal{
			SourceType: SignalMention,
			Confidence: edge.Confidence,
			Evidence:   "NER+SVO: " + edge.Evidence,
			Weight:     1.0,
		}
		_ = reg.AddSignal(fromID, toID, signal)
	}
	reg.AggregateConfidence()
}

// tripleToEdge converts an SVO Triple into a graph Edge by resolving the
// subject and object to known components and the source document.
//
// Returns nil if either subject or object cannot be resolved to a component,
// or if the edge would be a self-loop.
func tripleToEdge(triple Triple, doc *Document, registry map[string]*NERComponent, documents []Document) *Edge {
	// Resolve subject to a component.
	subjectComp := FuzzyComponentMatch(triple.Subject, registry)
	objectComp := FuzzyComponentMatch(triple.Object, registry)

	// If either side doesn't resolve, try to use the document itself as the subject.
	if subjectComp == nil && objectComp != nil {
		// The subject might be implicit (the document itself).
		// Check if the document maps to any component.
		subjectComp = findComponentByFile(doc.ID, registry)
	}

	if subjectComp == nil || objectComp == nil {
		return nil
	}

	// Resolve components to file paths.
	sourceFile := ResolveComponentToFile(subjectComp, documents)
	targetFile := ResolveComponentToFile(objectComp, documents)

	if sourceFile == "" || targetFile == "" {
		return nil
	}

	// Validate that both source and target files exist in the document collection
	if !fileExistsInCollection(sourceFile, documents) || !fileExistsInCollection(targetFile, documents) {
		return nil
	}

	// Prevent self-loops.
	if sourceFile == targetFile {
		return nil
	}

	// Classify the verb.
	edgeType, confidence := ClassifyVerb(triple.Verb)

	// Build evidence string.
	evidence := triple.Subject + " " + triple.Verb + " " + triple.Object

	edge, err := NewEdge(sourceFile, targetFile, edgeType, confidence, evidence)
	if err != nil {
		return nil
	}

	return edge
}

// findComponentByFile looks up a component by its file path in the registry.
// Uses sorted-key iteration so that when multiple components share the same
// file path, the alphabetically-first component ID is returned deterministically.
func findComponentByFile(fileID string, registry map[string]*NERComponent) *NERComponent {
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if registry[k].File == fileID {
			return registry[k]
		}
	}
	return nil
}

// extractSentences splits document content into individual sentences for
// SVO analysis.
//
// It handles both line-based and punctuation-based sentence boundaries,
// filtering out code blocks, headings, and list markers.
func extractSentences(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var sentences []string
	inCodeBlock := false

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		// Track code block state.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Skip empty lines.
		if trimmed == "" {
			continue
		}

		// Skip pure heading lines (but include headings with content).
		if isHeadingOnly(trimmed) {
			continue
		}

		// Clean the line for sentence extraction.
		cleaned := cleanForSentences(trimmed)
		if cleaned == "" {
			continue
		}

		// Split on sentence-ending punctuation if the line contains multiple sentences.
		parts := splitSentences(cleaned)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if len(part) >= 10 { // Minimum useful sentence length.
				sentences = append(sentences, part)
			}
		}
	}

	return sentences
}

// isHeadingOnly returns true if the line is a heading without additional content.
func isHeadingOnly(line string) bool {
	stripped := strings.TrimLeft(line, "#")
	if stripped == line {
		return false
	}
	// It's a heading; check if the remaining text is just a title.
	return true
}

// cleanForSentences removes markdown artifacts from a line for sentence parsing.
func cleanForSentences(line string) string {
	// Remove heading markers.
	for strings.HasPrefix(line, "#") {
		line = strings.TrimPrefix(line, "#")
	}

	// Remove list markers.
	line = strings.TrimLeft(line, "- *+0123456789.")

	// Remove bold/italic markers.
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")

	// Remove backticks.
	line = strings.ReplaceAll(line, "`", "")

	return strings.TrimSpace(line)
}

// contextVerbRe matches relationship verbs in lines that mention other components.
// These patterns detect lines like "- Validates customer information" or
// "Requires user authentication" where the subject is implicit (the document's component).
var contextVerbPatterns = []struct {
	re       *regexp.Regexp
	edgeType EdgeType
	conf     float64
}{
	{regexp.MustCompile(`(?i)\b(?:depends?\s+on|relies?\s+on)\b`), EdgeDependsOn, 0.75},
	{regexp.MustCompile(`(?i)\b(?:requires?|needs?)\b`), EdgeDependsOn, 0.70},
	{regexp.MustCompile(`(?i)\b(?:calls?|invokes?|sends?\s+(?:request|data)\s+to)\b`), EdgeCalls, 0.70},
	{regexp.MustCompile(`(?i)\b(?:uses?|consumes?)\b`), EdgeMentions, 0.65},
	{regexp.MustCompile(`(?i)\b(?:validates?\s+(?:via|through|using)|authenticates?\s+(?:via|through|using))\b`), EdgeDependsOn, 0.70},
	{regexp.MustCompile(`(?i)\b(?:integrates?\s+with|connects?\s+to|communicates?\s+with)\b`), EdgeDependsOn, 0.65},
	{regexp.MustCompile(`(?i)\b(?:processes?\s+(?:payment|order|request)s?\s+(?:for|from|via))\b`), EdgeCalls, 0.65},
	{regexp.MustCompile(`(?i)\b(?:stores?|persists?|records?)\s+.*\b(?:in|to)\b`), EdgeDependsOn, 0.65},
}

// extractContextRelationships scans a document for lines that mention other
// components in the context of relationship verbs. The document's own component
// is used as the implicit subject.
//
// This captures relationships expressed in list items and bullet points like:
//   - "User Service - Validates customer information"
//   - "Requires user authentication from User Service"
func extractContextRelationships(doc *Document, docComp *NERComponent, registry map[string]*NERComponent, documents []Document) []*Edge {
	lines := strings.Split(doc.Content, "\n")
	inCodeBlock := false

	type edgeKey struct{ source, target string }
	bestEdges := make(map[edgeKey]*Edge)

	sourceFile := ResolveComponentToFile(docComp, documents)
	if sourceFile == "" {
		return nil
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock || trimmed == "" {
			continue
		}

		// Clean the line.
		cleaned := cleanForSentences(trimmed)
		if cleaned == "" || len(cleaned) < 10 {
			continue
		}

		// Find all other components mentioned in this line.
		mentionedComps := FindComponentsInLine(cleaned, registry)
		if len(mentionedComps) == 0 {
			continue
		}

		// Check if any relationship verb appears in this line.
		for _, cvp := range contextVerbPatterns {
			if !cvp.re.MatchString(cleaned) {
				continue
			}

			// Create edges from document component to each mentioned component.
			for _, targetComp := range mentionedComps {
				if targetComp.ID == docComp.ID {
					continue // Skip self-references.
				}

				targetFile := ResolveComponentToFile(targetComp, documents)
				if targetFile == "" || targetFile == sourceFile {
					continue
				}

				// Validate that target file exists in the document collection
				if !fileExistsInCollection(targetFile, documents) {
					continue
				}

				evidence := docComp.Name + " " + string(cvp.edgeType) + " " + targetComp.Name + " (context: " + truncateLine(cleaned, 60) + ")"

				edge, err := NewEdge(sourceFile, targetFile, cvp.edgeType, cvp.conf, evidence)
				if err != nil {
					continue
				}

				key := edgeKey{source: sourceFile, target: targetFile}
				if existing, ok := bestEdges[key]; ok {
					if edge.Confidence > existing.Confidence {
						bestEdges[key] = edge
					}
				} else {
					bestEdges[key] = edge
				}
			}
			break // Only use the highest-priority verb match per line.
		}
	}

	result := make([]*Edge, 0, len(bestEdges))
	for _, edge := range bestEdges {
		result = append(result, edge)
	}
	sort.Slice(result, func(i, j int) bool {
		ki := result[i].Source + "\x00" + result[i].Target
		kj := result[j].Source + "\x00" + result[j].Target
		return ki < kj
	})
	return result
}

// splitSentences splits a string into sentences at period, question mark,
// or exclamation boundaries.
func splitSentences(text string) []string {
	if !strings.ContainsAny(text, ".?!") {
		return []string{text}
	}

	var sentences []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '.' || text[i] == '?' || text[i] == '!' {
			// Check it's followed by a space or end of string (not a decimal or abbreviation).
			if i+1 >= len(text) || text[i+1] == ' ' {
				sentence := strings.TrimSpace(text[start : i+1])
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				start = i + 1
			}
		}
	}

	// Remainder after last punctuation.
	if start < len(text) {
		remainder := strings.TrimSpace(text[start:])
		if remainder != "" {
			sentences = append(sentences, remainder)
		}
	}

	return sentences
}

// fileExistsInCollection checks if a file ID exists in the documents collection.
func fileExistsInCollection(fileID string, documents []Document) bool {
	for _, doc := range documents {
		if doc.ID == fileID {
			return true
		}
	}
	return false
}
