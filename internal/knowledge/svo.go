package knowledge

import (
	"regexp"
	"strings"
)

// Triple represents a Subject-Verb-Object extraction from a sentence.
type Triple struct {
	// Subject is the entity performing the action.
	Subject string

	// Verb is the relationship verb (e.g. "depends on", "calls").
	Verb string

	// Object is the entity being acted upon.
	Object string
}

// SVOPattern defines a compiled regex and the canonical verb it matches.
type SVOPattern struct {
	// Pattern is a compiled regex with named groups "subject" and "object".
	Pattern *regexp.Regexp

	// Verb is the canonical verb form (e.g. "depends on", "calls").
	Verb string
}

// svoPatterns is the built-in library of SVO extraction patterns.
// Each pattern captures subject and object components around a relationship verb.
//
// Patterns are ordered by specificity (most specific first) to avoid
// over-matching with greedy patterns.
var svoPatterns = []SVOPattern{
	// "X depends on Y" / "X depend on Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+(?:depends?|depended)\s+on\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "depends on",
	},
	// "X requires Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+requires?\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "requires",
	},
	// "X calls Y" / "X invokes Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+(?:calls?|invokes?)\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?\s]|to\s|$)`),
		Verb:    "calls",
	},
	// "X uses Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+uses?\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "uses",
	},
	// "X integrates with Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+integrates?\s+with\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "integrates with",
	},
	// "X connects to Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+connects?\s+to\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "connects to",
	},
	// "X provides Y" / "X exposes Y" / "X exports Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+(?:provides?|exposes?|exports?)\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component|endpoint))?\s*(?:[.,;:!?]|$)`),
		Verb:    "provides",
	},
	// "X communicates with Y" / "X talks to Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+(?:communicates?\s+with|talks?\s+to)\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "communicates with",
	},
	// "X sends request to Y" / "X sends data to Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+sends?\s+(?:a\s+)?(?:request|data|message|event)s?\s+to\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "calls",
	},
	// "X validates via Y" / "X authenticates via Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+(?:validates?|authenticates?|verifies?)\s+(?:via|through|using)\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "depends on",
	},
	// "X is submitted to Y"
	{
		Pattern: regexp.MustCompile(`(?i)\b(?P<subject>[\w][\w\s-]*?)\s+is\s+(?:submitted|sent|forwarded|passed)\s+to\s+(?:the\s+)?(?P<object>[\w][\w\s-]*?)(?:\s+(?:service|api|server|backend|component))?\s*(?:[.,;:!?]|$)`),
		Verb:    "calls",
	},
}

// VerbClassification holds the edge type and confidence for a classified verb.
type VerbClassification struct {
	// EdgeType is the relationship type mapped from the verb.
	EdgeType EdgeType

	// Confidence is the score assigned to this verb class.
	Confidence float64
}

// verbClassifications maps canonical verbs to their edge type and confidence.
var verbClassifications = map[string]VerbClassification{
	"depends on":        {EdgeType: EdgeDependsOn, Confidence: 0.80},
	"requires":          {EdgeType: EdgeDependsOn, Confidence: 0.80},
	"calls":             {EdgeType: EdgeCalls, Confidence: 0.75},
	"uses":              {EdgeType: EdgeMentions, Confidence: 0.70},
	"integrates with":   {EdgeType: EdgeDependsOn, Confidence: 0.65},
	"connects to":       {EdgeType: EdgeDependsOn, Confidence: 0.65},
	"provides":          {EdgeType: EdgeImplements, Confidence: 0.70},
	"communicates with": {EdgeType: EdgeDependsOn, Confidence: 0.65},
}

// ExtractSVOTriples extracts Subject-Verb-Object triples from a sentence.
//
// The function applies pattern-based extraction (no heavy NLP) to find
// component-to-component relationships expressed in natural language.
//
// Returns all matched triples. Multiple patterns may match the same sentence.
func ExtractSVOTriples(sentence string) []Triple {
	if sentence == "" {
		return nil
	}

	// Clean up the sentence for better matching.
	cleaned := cleanSentence(sentence)
	if cleaned == "" {
		return nil
	}

	var triples []Triple
	seen := make(map[string]bool)

	for _, sp := range svoPatterns {
		matches := sp.Pattern.FindAllStringSubmatch(cleaned, -1)
		for _, match := range matches {
			subjectIdx := sp.Pattern.SubexpIndex("subject")
			objectIdx := sp.Pattern.SubexpIndex("object")

			if subjectIdx < 0 || objectIdx < 0 || subjectIdx >= len(match) || objectIdx >= len(match) {
				continue
			}

			subject := strings.TrimSpace(match[subjectIdx])
			object := strings.TrimSpace(match[objectIdx])

			if subject == "" || object == "" {
				continue
			}

			// Skip if subject and object are the same.
			if strings.EqualFold(subject, object) {
				continue
			}

			// Skip common false positives.
			if isStopWord(subject) || isStopWord(object) {
				continue
			}

			// Deduplicate.
			key := strings.ToLower(subject) + "|" + sp.Verb + "|" + strings.ToLower(object)
			if seen[key] {
				continue
			}
			seen[key] = true

			triples = append(triples, Triple{
				Subject: subject,
				Verb:    sp.Verb,
				Object:  object,
			})
		}
	}

	return triples
}

// ClassifyVerb maps a verb string to an EdgeType and confidence score.
//
// Returns the classification for known verbs, or (EdgeMentions, 0.60) for
// unrecognised verbs.
func ClassifyVerb(verb string) (EdgeType, float64) {
	lower := strings.ToLower(strings.TrimSpace(verb))
	if vc, ok := verbClassifications[lower]; ok {
		return vc.EdgeType, vc.Confidence
	}
	return EdgeMentions, 0.60
}

// --- internal helpers -------------------------------------------------------

// cleanSentence prepares a sentence for SVO extraction by removing markdown
// artifacts and normalising whitespace.
func cleanSentence(s string) string {
	// Remove markdown formatting.
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "`", "")

	// Remove markdown link syntax: [text](url) -> text
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	s = linkRe.ReplaceAllString(s, "$1")

	// Remove list markers.
	s = strings.TrimLeft(s, "- *+")
	s = strings.TrimSpace(s)

	// Remove heading markers.
	for strings.HasPrefix(s, "#") {
		s = strings.TrimPrefix(s, "#")
	}
	s = strings.TrimSpace(s)

	// Collapse whitespace.
	spaceRe := regexp.MustCompile(`\s+`)
	s = spaceRe.ReplaceAllString(s, " ")

	return s
}

// svoStopWords are common words that should not be treated as subjects or objects.
var svoStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "it": true, "this": true,
	"that": true, "these": true, "those": true, "they": true,
	"we": true, "you": true, "i": true, "he": true, "she": true,
	"all": true, "each": true, "every": true, "some": true,
	"any": true, "no": true, "not": true, "also": true,
	"see": true, "note": true, "then": true, "which": true,
}

// isStopWord returns true if the word is a common stop word that should not
// be treated as a subject or object in SVO extraction.
func isStopWord(word string) bool {
	return svoStopWords[strings.ToLower(strings.TrimSpace(word))]
}
