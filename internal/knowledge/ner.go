package knowledge

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// NERComponent represents a named entity extracted from documentation.
// It captures a service, API, database, or other architectural component
// detected through multiple heuristic signals.
type NERComponent struct {
	// ID is a normalised, kebab-case identifier (e.g. "user-service").
	ID string

	// Name is the human-readable label (e.g. "User Service").
	Name string

	// Type is the component category: "service", "api", "database", "config", "unknown".
	Type string

	// File is the relative path of the document where this component was detected.
	File string

	// Aliases holds alternative names that should resolve to this component.
	Aliases []string
}

// componentTypeKeywords maps keyword substrings to component types.
// Order matters: earlier entries take priority when multiple keywords match.
// Most-specific compound keywords appear first to avoid shadowing.
var componentTypeKeywords = []struct {
	keyword string
	cType   string
}{
	{"database", "database"}, // most specific: unambiguous data store
	{"gateway", "api"},       // specific: api-gateway pattern
	{"service", "service"},   // common: microservice files
	{"api", "api"},           // common: api endpoint files
	{"db", "database"},       // abbreviation for database
	{"config", "config"},     // configuration files
	{"setup", "config"},      // setup/init files
	{"component", "service"}, // generic component → treat as service
}

// serviceHeadingRe matches H1/H2 headings that contain service/API/component keywords.
var serviceHeadingRe = regexp.MustCompile(`(?i)^#{1,2}\s+(.+?)(?:\s*\{.*\})?\s*$`)

// providesListRe matches "Provides: X, Y, Z" patterns in text.
var providesListRe = regexp.MustCompile(`(?i)(?:provides|exposes|exports)\s*:\s*(.+)`)

// ExtractComponentNames analyses a collection of documents and returns a map
// of normalised component name to NERComponent.
//
// Detection signals (in priority order):
//  1. Filename: "services/user-service.md" -> "user-service"
//  2. H1 heading containing Service/API/Gateway/Component keywords
//  3. H2 "Service" subsection
//  4. Explicit "Provides: X, Y, Z" patterns
//  5. README section listings referencing services
func ExtractComponentNames(documents []Document) map[string]*NERComponent {
	registry := make(map[string]*NERComponent)

	for i := range documents {
		doc := &documents[i]
		extractFromDocument(doc, registry)
	}

	return registry
}

// extractFromDocument applies all NER heuristics to a single document and
// adds discovered components to the registry.
func extractFromDocument(doc *Document, registry map[string]*NERComponent) {
	// Signal 1: Filename-based detection.
	stem := filenameStem(doc.ID)
	stemLower := strings.ToLower(stem)
	compType := inferNERType(doc.ID, "")

	// If the filename contains a service-like keyword, register it.
	if isServiceFileName(stemLower) {
		comp := &NERComponent{
			ID:      NormalizeNERName(stem),
			Name:    doc.Title,
			Type:    compType,
			File:    doc.ID,
			Aliases: buildAliases(stem, doc.Title),
		}
		mergeNERComponent(registry, comp)
	}

	// Signal 2+3: Heading-based detection.
	lines := strings.Split(doc.Content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}

		m := serviceHeadingRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		headingText := strings.TrimSpace(m[1])
		headingLower := strings.ToLower(headingText)

		// Check if this heading contains a component type keyword.
		for _, kw := range componentTypeKeywords {
			if strings.Contains(headingLower, kw.keyword) {
				// Only register H1 headings as primary components.
				if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
					comp := &NERComponent{
						ID:      NormalizeNERName(headingText),
						Name:    headingText,
						Type:    kw.cType,
						File:    doc.ID,
						Aliases: buildAliases(headingText, stem),
					}
					mergeNERComponent(registry, comp)
				}
				break
			}
		}
	}

	// Signal 4: "Provides: X, Y, Z" patterns.
	for _, line := range lines {
		m := providesListRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		items := strings.Split(m[1], ",")
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			comp := &NERComponent{
				ID:   NormalizeNERName(item),
				Name: item,
				Type: "unknown",
				File: doc.ID,
			}
			mergeNERComponent(registry, comp)
		}
	}

	// Signal 5: README/overview listing patterns.
	// Detect numbered or bulleted service lists like "1. User Service - description".
	extractFromServiceLists(doc, lines, registry)
}

// serviceListRe matches list items that name a service or component.
// Patterns: "- User Service - description" or "1. Order Service - description"
var serviceListRe = regexp.MustCompile(`(?i)^\s*(?:[-*+]|\d+\.)\s+(?:\*{0,2})?([\w][\w\s-]*?(?:Service|API|Gateway|Component|Database))(?:\*{0,2})?\s*(?:[-:]|$)`)

// extractFromServiceLists extracts component names from bulleted/numbered lists
// in documents that appear to be README or overview files.
func extractFromServiceLists(doc *Document, lines []string, registry map[string]*NERComponent) {
	lowerID := strings.ToLower(doc.ID)
	isOverview := strings.Contains(lowerID, "readme") ||
		strings.Contains(lowerID, "overview") ||
		strings.Contains(lowerID, "architecture") ||
		strings.Contains(lowerID, "index")

	if !isOverview {
		return
	}

	for _, line := range lines {
		m := serviceListRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		comp := &NERComponent{
			ID:      NormalizeNERName(name),
			Name:    name,
			Type:    inferNERType("", name),
			File:    "", // Not the primary doc for this component.
			Aliases: buildAliases(name, ""),
		}
		mergeNERComponent(registry, comp)
	}
}

// BuildComponentRegistry creates a unified component registry from documents.
// It calls ExtractComponentNames and resolves duplicates and aliases.
func BuildComponentRegistry(docs []Document) map[string]*NERComponent {
	registry := ExtractComponentNames(docs)

	// Resolve file references: for components found in lists without a file,
	// try to find the matching document by name.
	// Collect and sort registry keys for deterministic iteration.
	regKeys := make([]string, 0, len(registry))
	for k := range registry {
		regKeys = append(regKeys, k)
	}
	sort.Strings(regKeys)

	for _, k := range regKeys {
		comp := registry[k]
		if comp.File != "" {
			continue
		}
		// Try to find a document whose title or filename matches.
		for i := range docs {
			docStem := strings.ToLower(filenameStem(docs[i].ID))
			compNorm := strings.ToLower(comp.ID)
			if docStem == compNorm || strings.Contains(docStem, compNorm) {
				comp.File = docs[i].ID
				break
			}
		}
	}

	return registry
}

// NormalizeNERName converts a human-readable name to a lowercase kebab-case
// identifier suitable for registry lookup.
//
// Examples:
//
//	"User Service"     -> "user-service"
//	"API Gateway"      -> "api-gateway"
//	"user-service.md"  -> "user-service"
//	"Order Svc"        -> "order-svc"
func NormalizeNERName(name string) string {
	// Remove file extension if present.
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Replace underscores with dashes.
	name = strings.ReplaceAll(name, "_", "-")

	// Split on whitespace and join with dashes.
	words := strings.Fields(strings.ToLower(name))
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, "-")
}

// FuzzyComponentMatch finds the best matching component in the registry for
// a given text string. It tries exact match first, then alias matching,
// then substring matching.
//
// Returns nil if no match is found.
func FuzzyComponentMatch(text string, registry map[string]*NERComponent) *NERComponent {
	if text == "" || len(registry) == 0 {
		return nil
	}

	normalized := NormalizeNERName(text)
	if normalized == "" {
		return nil
	}

	// 1. Exact match on ID.
	if comp, ok := registry[normalized]; ok {
		return comp
	}

	// 2. Check with common suffix variations.
	suffixes := []string{"-service", "-api", "-server", "-svc", "-backend", "-gateway"}
	for _, suffix := range suffixes {
		// Try adding the suffix.
		if comp, ok := registry[normalized+suffix]; ok {
			return comp
		}
		// Try removing the suffix.
		trimmed := strings.TrimSuffix(normalized, suffix)
		if trimmed != normalized {
			if comp, ok := registry[trimmed]; ok {
				return comp
			}
		}
	}

	// 3. Alias matching — use sorted-key iteration for determinism.
	// When multiple components share an alias, alphabetically-first ID wins.
	{
		aliasKeys := make([]string, 0, len(registry))
		for k := range registry {
			aliasKeys = append(aliasKeys, k)
		}
		sort.Strings(aliasKeys)
		for _, k := range aliasKeys {
			comp := registry[k]
			for _, alias := range comp.Aliases {
				if strings.ToLower(alias) == normalized {
					return comp
				}
			}
		}
	}

	// 4. Substring matching (last resort, requires minimum 4 chars to avoid false positives).
	if len(normalized) >= 4 {
		var candidates []*NERComponent
		for _, comp := range registry {
			compLower := strings.ToLower(comp.ID)
			if strings.Contains(compLower, normalized) || strings.Contains(normalized, compLower) {
				candidates = append(candidates, comp)
			}
		}
		if len(candidates) > 0 {
			// Sort by ID for determinism: prefer longer (more specific) ID.
			// When lengths are equal, alphabetically-first ID wins.
			sort.Slice(candidates, func(i, j int) bool {
				if len(candidates[i].ID) != len(candidates[j].ID) {
					return len(candidates[i].ID) > len(candidates[j].ID) // longest first
				}
				return candidates[i].ID < candidates[j].ID // alpha tiebreak
			})
			return candidates[0]
		}
	}

	return nil
}

// FindComponentsInLine scans a single line and returns all NERComponents
// whose names or aliases appear in the text.
func FindComponentsInLine(line string, registry map[string]*NERComponent) []*NERComponent {
	if line == "" || len(registry) == 0 {
		return nil
	}

	lowerLine := strings.ToLower(line)
	seen := make(map[string]bool)
	var result []*NERComponent

	for _, comp := range registry {
		if seen[comp.ID] {
			continue
		}

		// Check component name.
		nameLower := strings.ToLower(comp.Name)
		if nameLower != "" && strings.Contains(lowerLine, nameLower) {
			seen[comp.ID] = true
			result = append(result, comp)
			continue
		}

		// Check aliases.
		for _, alias := range comp.Aliases {
			aliasLower := strings.ToLower(alias)
			if aliasLower != "" && len(aliasLower) >= 3 && strings.Contains(lowerLine, aliasLower) {
				seen[comp.ID] = true
				result = append(result, comp)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ResolveComponentToFile maps a component to its primary documentation file.
// Returns the File field if set, or attempts to find a matching document
// from the provided document list.
func ResolveComponentToFile(comp *NERComponent, documents []Document) string {
	if comp.File != "" {
		return comp.File
	}

	compNorm := strings.ToLower(comp.ID)
	for _, doc := range documents {
		docStem := strings.ToLower(filenameStem(doc.ID))
		if docStem == compNorm || strings.Contains(docStem, compNorm) {
			return doc.ID
		}
	}
	return ""
}

// --- internal helpers -------------------------------------------------------

// isServiceFileName returns true if the filename stem indicates a service or
// component (e.g., "user-service", "api-gateway", "database").
func isServiceFileName(stemLower string) bool {
	keywords := []string{"service", "api", "gateway", "database", "db", "config", "setup"}
	for _, kw := range keywords {
		if strings.Contains(stemLower, kw) {
			return true
		}
	}
	return false
}

// inferNERType infers the component type from a file path or component name.
func inferNERType(filePath, name string) string {
	candidates := strings.ToLower(filePath + " " + name)
	for _, kw := range componentTypeKeywords {
		if strings.Contains(candidates, kw.keyword) {
			return kw.cType
		}
	}
	return "unknown"
}

// buildAliases generates alias names for a component from its various name forms.
func buildAliases(names ...string) []string {
	seen := make(map[string]bool)
	var aliases []string

	for _, name := range names {
		if name == "" {
			continue
		}

		// Add the raw name (lowercased).
		lower := strings.ToLower(name)
		if !seen[lower] {
			seen[lower] = true
			aliases = append(aliases, lower)
		}

		// Add kebab-case form.
		kebab := NormalizeNERName(name)
		if kebab != "" && !seen[kebab] {
			seen[kebab] = true
			aliases = append(aliases, kebab)
		}

		// Add form without common suffixes.
		for _, suffix := range []string{" service", " api", " gateway", "-service", "-api", "-gateway"} {
			trimmed := strings.TrimSuffix(lower, suffix)
			if trimmed != lower && trimmed != "" && !seen[trimmed] {
				seen[trimmed] = true
				aliases = append(aliases, trimmed)
			}
		}
	}

	return aliases
}

// mergeNERComponent adds comp to registry, merging aliases if a component
// with the same ID already exists.
func mergeNERComponent(registry map[string]*NERComponent, comp *NERComponent) {
	if comp.ID == "" {
		return
	}

	existing, ok := registry[comp.ID]
	if !ok {
		registry[comp.ID] = comp
		return
	}

	// Merge: prefer the one with a file reference.
	if existing.File == "" && comp.File != "" {
		existing.File = comp.File
	}
	if existing.Type == "unknown" && comp.Type != "unknown" {
		existing.Type = comp.Type
	}

	// Merge aliases.
	aliasSet := make(map[string]bool)
	for _, a := range existing.Aliases {
		aliasSet[a] = true
	}
	for _, a := range comp.Aliases {
		if !aliasSet[a] {
			existing.Aliases = append(existing.Aliases, a)
		}
	}
}
