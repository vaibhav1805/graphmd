package knowledge

import (
	"regexp"
	"strings"
)

// ConfidenceMentionCalls is the confidence score for "calls X" patterns.
const ConfidenceMentionCalls float64 = 0.75

// ConfidenceMentionDependsOn is the confidence score for "depends on X" patterns.
const ConfidenceMentionDependsOn float64 = 0.75

// ConfidenceMentionServiceSuffix is the confidence score for "X-service" patterns.
const ConfidenceMentionServiceSuffix float64 = 0.75

// ConfidenceMentionUses is the confidence score for "uses X" patterns.
const ConfidenceMentionUses float64 = 0.7

// ConfidenceMentionAPI is the confidence score for "X API" patterns.
const ConfidenceMentionAPI float64 = 0.7

// ConfidenceMentionPortColon is the confidence score for "X:port" patterns.
const ConfidenceMentionPortColon float64 = 0.65

// ConfidenceMentionAPIPath is the confidence score for "api/X" URL path patterns.
const ConfidenceMentionAPIPath float64 = 0.65

// ConfidenceMentionHostname is the confidence score for "X.example.com" hostname patterns.
const ConfidenceMentionHostname float64 = 0.65

// ConfidenceMentionHTTP is the confidence score for "http.*X" URL patterns.
const ConfidenceMentionHTTP float64 = 0.6

// PatternMatch describes a pattern rule and the confidence score to assign
// when the rule matches a component name in document text.
type PatternMatch struct {
	// Pattern is a compiled regular expression with a single named group "name"
	// that captures the referenced component.
	Pattern *regexp.Regexp

	// Confidence is the score (0.0-1.0) assigned to mentions matched by this
	// pattern.
	Confidence float64

	// PatternType is a human-readable label used for debugging and evidence
	// strings (e.g. "calls", "depends-on", "service-suffix").
	PatternType string
}

// ComponentPatterns holds the built-in pattern sets for mention detection.
//
// ServicePatterns match verb-based prose relationships ("calls X", "uses X").
// ApiPatterns match protocol and endpoint references ("api/X", "X.example.com").
// ConfigPatterns match suffix conventions ("X-service", "X:port").
type ComponentPatterns struct {
	ServicePatterns []PatternMatch
	ApiPatterns     []PatternMatch
	ConfigPatterns  []PatternMatch
}

// BuiltInPatterns returns a ComponentPatterns value populated with all
// default pattern rules.
//
// The patterns are intentionally simple regex rules using word-boundary and
// whitespace anchors to avoid over-matching common English words.
func BuiltInPatterns() ComponentPatterns {
	return ComponentPatterns{
		ServicePatterns: []PatternMatch{
			{
				Pattern:     regexp.MustCompile(`(?i)\bcalls?\s+(?:the\s+)?(?P<name>[\w][\w.-]*)(?:\s+(?:service|api|server|backend))?`),
				Confidence:  ConfidenceMentionCalls,
				PatternType: "calls",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\bdepends?\s+on\s+(?:the\s+)?(?P<name>[\w][\w.-]*)(?:\s+(?:service|api|server|backend))?`),
				Confidence:  ConfidenceMentionDependsOn,
				PatternType: "depends-on",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\bintegrates?\s+with\s+(?:the\s+)?(?P<name>[\w][\w.-]*)(?:\s+(?:service|api|server|backend))?`),
				Confidence:  ConfidenceMentionDependsOn,
				PatternType: "integrates-with",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\buses?\s+(?:the\s+)?(?P<name>[\w][\w.-]*)(?:\s+(?:service|api|server|backend))?`),
				Confidence:  ConfidenceMentionUses,
				PatternType: "uses",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\brequires?\s+(?:the\s+)?(?P<name>[\w][\w.-]*)(?:\s+(?:service|api|server|backend))?`),
				Confidence:  ConfidenceMentionUses,
				PatternType: "requires",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\bsends?\s+(?:a\s+)?request\s+to\s+(?:the\s+)?(?P<name>[\w][\w.-]*)(?:\s+(?:service|api|server|backend))?`),
				Confidence:  ConfidenceMentionCalls,
				PatternType: "sends-request-to",
			},
		},
		ApiPatterns: []PatternMatch{
			{
				Pattern:     regexp.MustCompile(`(?i)\b(?P<name>[\w][\w-]*)\.(?:example|internal|local|corp|svc)\.com\b`),
				Confidence:  ConfidenceMentionHostname,
				PatternType: "hostname",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\bapi/(?P<name>[\w][\w-]*)\b`),
				Confidence:  ConfidenceMentionAPIPath,
				PatternType: "api-path",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\bhttps?://[^\s/]*(?P<name>[\w][\w-]*)[^\s]*`),
				Confidence:  ConfidenceMentionHTTP,
				PatternType: "http-url",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\b(?P<name>[\w][\w.-]*)\s+API\b`),
				Confidence:  ConfidenceMentionAPI,
				PatternType: "x-api",
			},
		},
		ConfigPatterns: []PatternMatch{
			{
				Pattern:     regexp.MustCompile(`(?i)\b(?P<name>[\w][\w-]*)-service\b`),
				Confidence:  ConfidenceMentionServiceSuffix,
				PatternType: "service-suffix",
			},
			{
				Pattern:     regexp.MustCompile(`(?i)\b(?P<name>[\w][\w.-]*):\d{2,5}\b`),
				Confidence:  ConfidenceMentionPortColon,
				PatternType: "host-port",
			},
		},
	}
}

// AllPatterns returns all patterns from a ComponentPatterns value as a flat
// slice, ordered: ServicePatterns, ApiPatterns, ConfigPatterns.
func (cp ComponentPatterns) AllPatterns() []PatternMatch {
	all := make([]PatternMatch, 0,
		len(cp.ServicePatterns)+len(cp.ApiPatterns)+len(cp.ConfigPatterns))
	all = append(all, cp.ServicePatterns...)
	all = append(all, cp.ApiPatterns...)
	all = append(all, cp.ConfigPatterns...)
	return all
}

// IsComponentMention checks whether text contains a reference to componentName
// using the built-in pattern library.
//
// componentName is matched case-insensitively using whole-word comparison to
// avoid false positives such as "authentication" matching "auth".
//
// Returns (true, confidence) when a match is found, (false, 0) otherwise.
func IsComponentMention(text, componentName string) (bool, float64) {
	if componentName == "" || text == "" {
		return false, 0
	}
	patterns := BuiltInPatterns()
	lowerText := strings.ToLower(text)
	lowerName := strings.ToLower(componentName)

	for _, pm := range patterns.AllPatterns() {
		matches := pm.Pattern.FindAllStringSubmatch(lowerText, -1)
		for _, m := range matches {
			idx := pm.Pattern.SubexpIndex("name")
			if idx < 0 || idx >= len(m) {
				continue
			}
			captured := strings.ToLower(m[idx])
			if isExactMatch(captured, lowerName) {
				return true, pm.Confidence
			}
		}
	}
	return false, 0
}

// ExtractMentionsFromLine scans a single line of text and returns a
// MentionCandidate for each known component that appears in the line.
//
// knownComponents is a map from normalized component name → canonical name.
// Multiple patterns may match the same component; only the highest-confidence
// match is returned per (line, component) pair.
func ExtractMentionsFromLine(line string, knownComponents map[string]string) []MentionCandidate {
	if line == "" || len(knownComponents) == 0 {
		return nil
	}

	patterns := BuiltInPatterns()
	lowerLine := strings.ToLower(line)

	// best[canonicalName] = highest confidence seen so far.
	best := make(map[string]MentionCandidate)

	for _, pm := range patterns.AllPatterns() {
		matches := pm.Pattern.FindAllStringSubmatch(lowerLine, -1)
		for _, m := range matches {
			idx := pm.Pattern.SubexpIndex("name")
			if idx < 0 || idx >= len(m) {
				continue
			}
			captured := strings.ToLower(m[idx])

			// Check if any known component matches this captured name.
			for normName, canonName := range knownComponents {
				if isExactMatch(captured, normName) {
					existing, exists := best[canonName]
					if !exists || pm.Confidence > existing.Confidence {
						best[canonName] = MentionCandidate{
							ComponentName: canonName,
							Confidence:    pm.Confidence,
							PatternType:   pm.PatternType,
							MatchedText:   m[0],
						}
					}
				}
			}
		}
	}

	if len(best) == 0 {
		return nil
	}

	result := make([]MentionCandidate, 0, len(best))
	for _, mc := range best {
		result = append(result, mc)
	}
	return result
}

// MentionCandidate records a single matched mention from a document line.
type MentionCandidate struct {
	// ComponentName is the canonical name of the referenced component.
	ComponentName string

	// Confidence is the score from the matching pattern (0.0-1.0).
	Confidence float64

	// PatternType is the human-readable pattern rule that matched.
	PatternType string

	// MatchedText is the raw text fragment that triggered the match.
	MatchedText string
}

// isExactMatch returns true when captured equals name, or when captured is
// the same as name after common suffix normalization ("-service", "-api", etc.).
//
// The comparison is always case-insensitive (callers must lowercase both
// inputs before calling).
//
// It uses whole-word logic: "auth" must not match "authentication".
func isExactMatch(captured, name string) bool {
	if captured == "" || name == "" {
		return false
	}
	// Direct match.
	if captured == name {
		return true
	}
	// Allow captured to be "name-service", "name-api", "name-server", "name-backend".
	suffixes := []string{"-service", "-api", "-server", "-backend", "-svc"}
	for _, sfx := range suffixes {
		if captured == name+sfx {
			return true
		}
		// Also allow name to include the suffix while captured doesn't.
		if name == captured+sfx {
			return true
		}
	}
	return false
}
