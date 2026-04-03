package knowledge

import (
	"strings"
	"unicode"
)

// defaultStopWords is a minimal English stop-word list used when no custom
// list is provided. It can be replaced entirely via TokenizerConfig.StopWords.
var defaultStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "been": {}, "being": {}, "but": {}, "by": {},
	"can": {}, "could": {},
	"do": {}, "does": {}, "did": {},
	"for": {}, "from": {},
	"had": {}, "has": {}, "have": {}, "he": {}, "her": {}, "here": {},
	"him": {}, "his": {}, "how": {},
	"i": {}, "if": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {},
	"just": {},
	"me": {}, "my": {},
	"no": {}, "not": {}, "now": {},
	"of": {}, "on": {}, "or": {}, "our": {},
	"s": {}, "she": {}, "should": {}, "so": {}, "some": {},
	"than": {}, "that": {}, "the": {}, "their": {}, "them": {},
	"then": {}, "there": {}, "these": {}, "they": {}, "this": {},
	"those": {}, "through": {}, "to": {}, "too": {},
	"up": {}, "use": {},
	"was": {}, "we": {}, "were": {}, "what": {}, "when": {},
	"where": {}, "which": {}, "while": {}, "who": {}, "will": {},
	"with": {}, "would": {},
	"you": {}, "your": {},
}

// TokenizerConfig controls optional tokenisation behaviour.
type TokenizerConfig struct {
	// RemoveStopWords enables stop-word filtering.  When true, any token that
	// appears in StopWords is dropped from the output.
	RemoveStopWords bool

	// StopWords is the set of words to remove.  If nil the default English
	// stop-word list is used.  Pass an empty map to disable stop-word removal
	// even when RemoveStopWords is true.
	StopWords map[string]struct{}

	// MinTokenLen drops tokens shorter than this value.  0 means no minimum.
	MinTokenLen int
}

// DefaultTokenizerConfig returns a sensible default configuration that enables
// stop-word removal using the built-in English word list.
func DefaultTokenizerConfig() TokenizerConfig {
	return TokenizerConfig{
		RemoveStopWords: true,
		StopWords:       defaultStopWords,
		MinTokenLen:     2,
	}
}

// Tokenizer provides text normalisation and term extraction for BM25 indexing.
type Tokenizer struct {
	cfg TokenizerConfig
}

// NewTokenizer creates a Tokenizer with the given configuration.
func NewTokenizer(cfg TokenizerConfig) *Tokenizer {
	if cfg.StopWords == nil && cfg.RemoveStopWords {
		cfg.StopWords = defaultStopWords
	}
	return &Tokenizer{cfg: cfg}
}

// Tokenize splits text into normalised terms ready for indexing.
//
// Algorithm:
//  1. Lowercase the entire input.
//  2. Iterate rune-by-rune; accumulate runs of "term characters".
//     A term character is: a letter, a digit, or a hyphen that is surrounded
//     by at least one letter/digit on both sides (compound words like
//     "api-gateway" stay intact).
//  3. Flush an accumulated token when a non-term character is encountered.
//  4. Optionally drop stop words and short tokens.
//
// The function handles arbitrary Unicode input correctly because it operates
// on rune slices rather than bytes.
func (t *Tokenizer) Tokenize(text string) []string {
	lower := strings.ToLower(text)
	runes := []rune(lower)
	n := len(runes)

	var tokens []string
	var buf []rune

	flush := func() {
		if len(buf) == 0 {
			return
		}
		token := string(buf)
		buf = buf[:0]

		// Trim leading/trailing hyphens that may have been accumulated at
		// word boundaries.
		token = strings.Trim(token, "-")
		if token == "" {
			return
		}

		// Minimum length filter.
		if t.cfg.MinTokenLen > 0 && len([]rune(token)) < t.cfg.MinTokenLen {
			return
		}

		// Stop-word filter.
		if t.cfg.RemoveStopWords {
			if _, isStop := t.cfg.StopWords[token]; isStop {
				return
			}
		}

		tokens = append(tokens, token)
	}

	for i := 0; i < n; i++ {
		r := runes[i]
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf = append(buf, r)
			continue
		}

		// A hyphen is kept only when it sits between two letter/digit runes
		// (compound word boundary).  Otherwise it acts as a delimiter.
		if r == '-' {
			prev := i > 0 && (unicode.IsLetter(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			next := i+1 < n && (unicode.IsLetter(runes[i+1]) || unicode.IsDigit(runes[i+1]))
			if prev && next {
				buf = append(buf, r)
				continue
			}
		}

		// All other runes are delimiters.
		flush()
	}
	flush()

	return tokens
}

// TokenizeWithDefaults is a convenience function that tokenises text using the
// default configuration (stop-word removal enabled, min length 2).
func TokenizeWithDefaults(text string) []string {
	return NewTokenizer(DefaultTokenizerConfig()).Tokenize(text)
}
