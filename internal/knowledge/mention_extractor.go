package knowledge

import (
	"strings"
)

// Mention records a detected textual reference from a source document to a
// known component, discovered by the pattern matching engine.
type Mention struct {
	// FromFile is the relative path of the document that mentions the component.
	FromFile string

	// ToComponent is the canonical component identifier that was mentioned.
	ToComponent string

	// Confidence is the pattern-based confidence score (0.6–0.75).
	Confidence float64

	// EvidenceCount is the number of distinct lines that mention the component.
	EvidenceCount int

	// ExampleEvidence is the first matching line (trimmed) for human inspection.
	ExampleEvidence string
}

// ExtractMentionsFromDocument scans a Document's content and returns a
// Mention for each known component referenced in the document text.
//
// components is a slice of known components; only their IDs and Names are
// used for matching. The document's own component (if any) is excluded from
// results to avoid self-referential mentions.
//
// Algorithm:
//  1. Build a lookup map of normalized component names.
//  2. Split the document into lines.
//  3. For each line, call ExtractMentionsFromLine to find matching components.
//  4. Aggregate per (fromFile, toComponent) pair — keep highest confidence
//     seen and count distinct matching lines.
func ExtractMentionsFromDocument(doc Document, components []Component) []Mention {
	if doc.ID == "" || len(components) == 0 {
		return nil
	}

	// Build normalized name → canonical name lookup.
	// We include both the component ID and a cleaned version of the Name.
	knownComponents := buildComponentLookup(components, doc.ID)
	if len(knownComponents) == 0 {
		return nil
	}

	lines := strings.Split(doc.Content, "\n")

	// Aggregate: key = toComponent canonical name
	type agg struct {
		confidence      float64
		evidenceCount   int
		exampleEvidence string
	}
	best := make(map[string]*agg)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		candidates := ExtractMentionsFromLine(line, knownComponents)
		for _, mc := range candidates {
			if existing, ok := best[mc.ComponentName]; ok {
				existing.evidenceCount++
				if mc.Confidence > existing.confidence {
					existing.confidence = mc.Confidence
					existing.exampleEvidence = truncateLine(line, 200)
				}
			} else {
				best[mc.ComponentName] = &agg{
					confidence:      mc.Confidence,
					evidenceCount:   1,
					exampleEvidence: truncateLine(line, 200),
				}
			}
		}
	}

	if len(best) == 0 {
		return nil
	}

	result := make([]Mention, 0, len(best))
	for compName, a := range best {
		result = append(result, Mention{
			FromFile:        doc.ID,
			ToComponent:     compName,
			Confidence:      a.confidence,
			EvidenceCount:   a.evidenceCount,
			ExampleEvidence: a.exampleEvidence,
		})
	}
	return result
}

// ExtractMentionsFromDocuments runs ExtractMentionsFromDocument over a
// collection of documents and returns the merged results.
//
// This is the batch entry-point for registry integration.
func ExtractMentionsFromDocuments(docs []Document, components []Component) []Mention {
	if len(docs) == 0 || len(components) == 0 {
		return nil
	}
	var all []Mention
	for _, doc := range docs {
		mentions := ExtractMentionsFromDocument(doc, components)
		all = append(all, mentions...)
	}
	return all
}

// buildComponentLookup converts a slice of Component into the normalized
// lookup map expected by ExtractMentionsFromLine:
//
//	normalizedName → canonicalName
//
// selfDocID is the source document's ID; the component whose FileRef matches
// selfDocID is excluded to prevent self-referential mentions.
func buildComponentLookup(components []Component, selfDocID string) map[string]string {
	m := make(map[string]string, len(components)*2)
	for _, c := range components {
		// Skip the component that belongs to this document.
		if c.File == selfDocID {
			continue
		}
		normID := strings.ToLower(c.ID)
		if normID != "" {
			m[normID] = c.ID
		}
		// Also index by normalized name (first word, lowercase, kebab-safe).
		normName := normalizeComponentName(c.Name)
		if normName != "" && normName != normID {
			m[normName] = c.ID
		}
	}
	return m
}

// normalizeComponentName converts a human-readable component name to a
// lowercase, kebab-compatible identifier suitable for pattern matching.
//
// Examples:
//
//	"Auth Service"   → "auth"
//	"API Gateway"    → "api-gateway"
//	"UserDB"         → "userdb"
func normalizeComponentName(name string) string {
	// Split on whitespace, lower each word, join with dash.
	words := strings.Fields(strings.ToLower(name))
	if len(words) == 0 {
		return ""
	}
	// Remove trailing "service", "api", "server" from single-word names to
	// allow matching "auth-service" as "auth".
	if len(words) == 1 {
		return strings.TrimSuffix(strings.TrimSuffix(
			strings.TrimSuffix(words[0], "-service"),
			"-api"), "-server")
	}
	return strings.Join(words, "-")
}

// truncateLine returns the first maxChars UTF-8 characters of s, appending
// "..." when truncation occurs.
func truncateLine(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "..."
}
