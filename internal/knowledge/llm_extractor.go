package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LLMCacheFileName is the name of the cache file for LLM extraction results.
const LLMCacheFileName = ".bmd-llm-extractions.json"

// LLMRelationship is a relationship inferred by PageIndex LLM reasoning.
type LLMRelationship struct {
	// FromFile is the relative path of the source document.
	FromFile string `json:"from_file"`

	// ToComponent is the canonical name of the dependent component.
	ToComponent string `json:"to_component"`

	// Confidence is how confident the LLM is in this relationship (0.0–1.0).
	Confidence float64 `json:"confidence"`

	// Reasoning explains why the LLM identified this relationship.
	Reasoning string `json:"reasoning"`

	// Evidence is the direct quote or summary supporting the relationship.
	Evidence string `json:"evidence"`
}

// llmExtractionCache is the serialized form of the LLM extraction cache.
type llmExtractionCache struct {
	GeneratedAt   time.Time         `json:"generated_at"`
	Relationships []LLMRelationship `json:"relationships"`
}

// QueryLLMConfig holds optional settings for LLM-powered extraction.
type QueryLLMConfig struct {
	// Enabled controls whether LLM extraction runs.
	// Default: auto-detect PageIndex availability.
	Enabled bool

	// CachePath is the path to the .bmd-llm-extractions.json cache file.
	// Empty string defaults to LLMCacheFileName in the current directory.
	CachePath string

	// SkipExisting skips re-querying documents whose results are already cached.
	SkipExisting bool

	// TimeoutSecs is the subprocess timeout in seconds.
	// Zero uses the default (30 seconds).
	TimeoutSecs int

	// PageIndexBin is the path to the pageindex binary.
	// Empty string uses "pageindex" resolved via PATH.
	PageIndexBin string

	// Model is the LLM model name passed to pageindex.
	// Empty string uses "claude-sonnet-4-5".
	Model string
}

// DefaultQueryLLMConfig returns a QueryLLMConfig with sensible defaults.
func DefaultQueryLLMConfig() QueryLLMConfig {
	return QueryLLMConfig{
		Enabled:      true,
		CachePath:    LLMCacheFileName,
		SkipExisting: true,
		TimeoutSecs:  30,
		PageIndexBin: "pageindex",
		Model:        "claude-sonnet-4-5",
	}
}

// pageIndexLLMResponse is the expected JSON shape from a PageIndex reasoning query.
type pageIndexLLMResponse struct {
	Service      string  `json:"service"`
	Relationship string  `json:"relationship"`
	Confidence   float64 `json:"confidence"`
	Evidence     string  `json:"evidence"`
}

// RunLLMExtraction queries PageIndex for each document to extract service
// relationships implied by the document's natural language content.
//
// It runs queries in parallel (one goroutine per document) and merges results.
// Results are cached to cfg.CachePath to avoid re-querying on subsequent runs.
//
// If PageIndex is unavailable, it returns an empty slice and no error
// (graceful degradation).
func RunLLMExtraction(cfg QueryLLMConfig, documents []Document, components []Component) ([]LLMRelationship, error) {
	if len(documents) == 0 || len(components) == 0 {
		return nil, nil
	}

	// Resolve cache path.
	cachePath := cfg.CachePath
	if cachePath == "" {
		cachePath = LLMCacheFileName
	}

	// Load existing cache.
	cachedByFile, err := loadCacheIndex(cachePath)
	if err != nil {
		// Non-fatal: proceed without cache on error.
		cachedByFile = make(map[string][]LLMRelationship)
	}

	// Build known component name set for filtering.
	knownComponents := buildKnownComponentSet(components)

	// Determine which documents need querying.
	var toQuery []Document
	var cached []LLMRelationship

	for _, doc := range documents {
		if cfg.SkipExisting {
			if existing, ok := cachedByFile[doc.ID]; ok {
				cached = append(cached, existing...)
				continue
			}
		}
		toQuery = append(toQuery, doc)
	}

	if len(toQuery) == 0 {
		return cached, nil
	}

	// Run parallel queries.
	type result struct {
		rels []LLMRelationship
		err  error
	}

	resultCh := make(chan result, len(toQuery))
	var wg sync.WaitGroup

	for _, doc := range toQuery {
		wg.Add(1)
		go func(d Document) {
			defer wg.Done()
			rels, qErr := QueryPageIndexForRelationships(cfg, d, knownComponents)
			resultCh <- result{rels: rels, err: qErr}
		}(doc)
	}

	wg.Wait()
	close(resultCh)

	var newRels []LLMRelationship
	var lastErr error

	for r := range resultCh {
		if r.err != nil {
			if errors.Is(r.err, ErrPageIndexNotFound) {
				// PageIndex not installed — graceful degradation.
				return cached, nil
			}
			lastErr = r.err
			continue
		}
		newRels = append(newRels, r.rels...)
	}

	// Merge new results with cache.
	all := append(cached, newRels...)

	// Persist updated cache (best-effort; don't fail on write error).
	_ = CacheLLMResults(all, cachePath)

	if lastErr != nil {
		return all, lastErr
	}
	return all, nil
}

// QueryPageIndexForRelationships invokes PageIndex as a subprocess to extract
// service dependencies from a single document.
//
// It builds a structured prompt requesting JSON output, then filters the
// response against the set of known components to reduce false positives.
func QueryPageIndexForRelationships(cfg QueryLLMConfig, doc Document, knownComponents map[string]bool) ([]LLMRelationship, error) {
	bin := cfg.PageIndexBin
	if bin == "" {
		bin = "pageindex"
	}
	model := cfg.Model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	prompt := buildExtractionPrompt(doc.Content)

	// Invoke pageindex query subcommand.
	cmd := exec.Command(bin, "query",
		"--query", prompt,
		"--model", model,
		"--format", "json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		errMsg := runErr.Error()
		if strings.Contains(errMsg, "executable file not found") ||
			strings.Contains(errMsg, "no such file or directory") {
			return nil, fmt.Errorf("pageindex not found: %w", ErrPageIndexNotFound)
		}
		var execErr *exec.Error
		if errors.As(runErr, &execErr) {
			return nil, fmt.Errorf("pageindex not found: %w", ErrPageIndexNotFound)
		}
		// Other subprocess errors — return empty, not fatal.
		return nil, nil
	}

	return parseAndFilterLLMResponse(stdout.Bytes(), doc.ID, knownComponents), nil
}

// buildExtractionPrompt constructs the prompt sent to PageIndex for relationship
// extraction. The prompt requests structured JSON output.
func buildExtractionPrompt(content string) string {
	// Truncate very large documents to avoid token limits.
	if len(content) > 4000 {
		content = content[:4000] + "\n...[truncated]"
	}

	return fmt.Sprintf(`Analyze this document and identify all mentioned or implied service dependencies.
For each dependency:
1. Service name
2. Relationship type (calls, depends on, integrates with, etc.)
3. Confidence (0.0-1.0 based on explicitness)
4. Evidence (exact quote)

Document:
%s

Return as JSON array only, no explanation: [{"service": "...", "relationship": "...", "confidence": 0.7, "evidence": "..."}]`, content)
}

// parseAndFilterLLMResponse parses the JSON array returned by PageIndex and
// filters results to known components.
func parseAndFilterLLMResponse(raw []byte, fromFile string, knownComponents map[string]bool) []LLMRelationship {
	// The response may be wrapped in text; find the JSON array.
	start := bytes.IndexByte(raw, '[')
	end := bytes.LastIndexByte(raw, ']')
	if start < 0 || end <= start {
		return nil
	}
	jsonPart := raw[start : end+1]

	var parsed []pageIndexLLMResponse
	if err := json.Unmarshal(jsonPart, &parsed); err != nil {
		return nil
	}

	var result []LLMRelationship
	for _, p := range parsed {
		svcName := strings.TrimSpace(strings.ToLower(p.Service))
		if svcName == "" {
			continue
		}

		// Filter to known components to reduce false positives.
		if !isKnownComponent(svcName, knownComponents) {
			continue
		}

		conf := p.Confidence
		if conf <= 0 {
			conf = 0.65 // default LLM confidence
		}
		if conf > 1.0 {
			conf = 1.0
		}

		result = append(result, LLMRelationship{
			FromFile:    fromFile,
			ToComponent: svcName,
			Confidence:  conf,
			Reasoning:   p.Relationship,
			Evidence:    p.Evidence,
		})
	}
	return result
}

// isKnownComponent returns true when svcName matches any known component.
// Matching is fuzzy: exact name, or name with -service/-api suffix.
func isKnownComponent(svcName string, known map[string]bool) bool {
	if known[svcName] {
		return true
	}
	// Also check common suffix variants.
	variants := []string{
		svcName + "-service",
		svcName + "-api",
		svcName + "-server",
		strings.TrimSuffix(svcName, "-service"),
		strings.TrimSuffix(svcName, "-api"),
	}
	for _, v := range variants {
		if known[v] {
			return true
		}
	}
	return false
}

// buildKnownComponentSet builds a lowercase name set from a component slice.
func buildKnownComponentSet(components []Component) map[string]bool {
	m := make(map[string]bool, len(components)*2)
	for _, c := range components {
		m[strings.ToLower(c.ID)] = true
		if c.Name != "" {
			m[strings.ToLower(normalizeForLLM(c.Name))] = true
		}
	}
	return m
}

// normalizeForLLM converts a component name to lowercase kebab for LLM matching.
func normalizeForLLM(name string) string {
	words := strings.Fields(strings.ToLower(name))
	return strings.Join(words, "-")
}

// CacheLLMResults persists the LLM relationship results to path as JSON.
func CacheLLMResults(results []LLMRelationship, path string) error {
	cache := llmExtractionCache{
		GeneratedAt:   time.Now(),
		Relationships: results,
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("CacheLLMResults: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Clean(path), data, 0o600); err != nil {
		return fmt.Errorf("CacheLLMResults: write %q: %w", path, err)
	}
	return nil
}

// LoadLLMCache loads cached LLM extraction results from path.
// Returns nil, nil when the file does not exist (graceful fallback).
func LoadLLMCache(path string) ([]LLMRelationship, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("LoadLLMCache: read %q: %w", path, err)
	}
	var cache llmExtractionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("LoadLLMCache: parse %q: %w", path, err)
	}
	return cache.Relationships, nil
}

// loadCacheIndex loads the cache file and returns a map from fromFile → []LLMRelationship
// for fast lookup during RunLLMExtraction.
func loadCacheIndex(path string) (map[string][]LLMRelationship, error) {
	rels, err := LoadLLMCache(path)
	if err != nil || len(rels) == 0 {
		return make(map[string][]LLMRelationship), err
	}
	m := make(map[string][]LLMRelationship, len(rels))
	for _, r := range rels {
		m[r.FromFile] = append(m[r.FromFile], r)
	}
	return m, nil
}
