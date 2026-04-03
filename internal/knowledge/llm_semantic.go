package knowledge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LLMDiscoveryConfig holds configuration for LLM-powered semantic discovery.
type LLMDiscoveryConfig struct {
	// ExecutablePath is the path to the pageindex CLI binary.
	// Defaults to "pageindex" (resolved via PATH).
	ExecutablePath string

	// Model is the LLM model name passed to pageindex.
	// Defaults to "claude-haiku-4-5-20251001" (fast, cheap for discovery).
	Model string

	// MaxConcurrency is the max number of parallel LLM calls.
	// Defaults to 4.
	MaxConcurrency int

	// CacheFile is the path to the discovery cache JSON file.
	// Defaults to ".bmd-llm-discovery-cache.json".
	CacheFile string

	// SkipExisting skips re-querying docs that are already cached.
	// Defaults to true.
	SkipExisting bool
}

// DefaultLLMDiscoveryConfig returns a LLMDiscoveryConfig with sensible defaults.
func DefaultLLMDiscoveryConfig() LLMDiscoveryConfig {
	return LLMDiscoveryConfig{
		ExecutablePath: "pageindex",
		Model:          "claude-haiku-4-5-20251001",
		MaxConcurrency: 4,
		CacheFile:      ".bmd-llm-discovery-cache.json",
		SkipExisting:   true,
	}
}

// llmDiscoveryResult holds the parsed response from an LLM query.
type llmDiscoveryResult struct {
	Target        string  `json:"target"`
	Relationship  string  `json:"relationship"`
	Confidence    float64 `json:"confidence"`
	Evidence      string  `json:"evidence"`
}

// llmDiscoveryCacheEntry stores cached LLM discovery results per document.
type llmDiscoveryCacheEntry struct {
	DocID       string                 `json:"doc_id"`
	ContentHash string                 `json:"content_hash"`
	Results     []llmDiscoveryResult   `json:"results"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// llmDiscoveryCache is the serialized form of the LLM discovery cache.
type llmDiscoveryCache struct {
	Entries   []llmDiscoveryCacheEntry `json:"entries"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// LLMSemanticRelationships discovers semantic relationships between documents
// using LLM reasoning over PageIndex tree summaries.
//
// For each document with a tree summary, it builds a prompt asking the LLM
// to identify which known services this document integrates with, based on
// the document's purpose (from the tree summary) and the list of known components.
//
// Results are cached by content hash to avoid re-querying unchanged documents.
//
// Returns a slice of DiscoveredEdge with SignalLLM signals.
func LLMSemanticRelationships(
	docs []Document,
	trees []FileTree,
	knownComponents []string,
	cfg LLMDiscoveryConfig,
) []*DiscoveredEdge {
	if len(docs) == 0 || len(trees) == 0 || len(knownComponents) == 0 {
		return nil
	}

	// Build tree index by document ID (relPath).
	treeByDocID := make(map[string]*FileTree)
	for i := range trees {
		treeByDocID[trees[i].File] = &trees[i]
	}

	// Load or initialize cache.
	cache := loadLLMDiscoveryCache(cfg.CacheFile)
	if cache == nil {
		cache = &llmDiscoveryCache{
			Entries:     []llmDiscoveryCacheEntry{},
			GeneratedAt: time.Now(),
		}
	}

	// Build cache map for fast lookup.
	cacheByDocID := make(map[string]*llmDiscoveryCacheEntry)
	for i := range cache.Entries {
		cacheByDocID[cache.Entries[i].DocID] = &cache.Entries[i]
	}

	// Collect docs that need querying (have trees, not cached or content changed).
	type docToQuery struct {
		doc  Document
		tree *FileTree
		hash string
	}
	var toQuery []docToQuery

	for _, doc := range docs {
		tree, hasTree := treeByDocID[doc.RelPath]
		if !hasTree {
			continue // No tree for this doc, skip
		}

		// Compute content hash.
		hash := hashDocContent(doc.Content)

		// Check cache.
		if cfg.SkipExisting {
			if cached, ok := cacheByDocID[doc.RelPath]; ok && cached.ContentHash == hash {
				continue // Cached and unchanged
			}
		}

		toQuery = append(toQuery, docToQuery{doc: doc, tree: tree, hash: hash})
	}

	if len(toQuery) == 0 {
		// All docs cached; use cache to build edges.
		return buildEdgesFromCache(cache, knownComponents)
	}

	// Query LLM for uncached documents (with concurrency limit).
	sem := make(chan struct{}, cfg.MaxConcurrency)
	results := make(chan llmDiscoveryCacheEntry, len(toQuery))

	var wg sync.WaitGroup
	for _, dtq := range toQuery {
		wg.Add(1)
		go func(doc Document, tree *FileTree, hash string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			llmResults := queryLLMForDocument(
				doc,
				tree,
				knownComponents,
				cfg,
			)

			results <- llmDiscoveryCacheEntry{
				DocID:       doc.RelPath,
				ContentHash: hash,
				Results:     llmResults,
				GeneratedAt: time.Now(),
			}
		}(dtq.doc, dtq.tree, dtq.hash)
	}

	wg.Wait()
	close(results)

	// Collect new results and update cache.
	for entry := range results {
		// Remove old cached entry if exists.
		filtered := make([]llmDiscoveryCacheEntry, 0, len(cache.Entries))
		for _, e := range cache.Entries {
			if e.DocID != entry.DocID {
				filtered = append(filtered, e)
			}
		}
		cache.Entries = filtered
		cache.Entries = append(cache.Entries, entry)
		cacheByDocID[entry.DocID] = &cache.Entries[len(cache.Entries)-1]
	}

	// Save updated cache.
	_ = saveLLMDiscoveryCache(cfg.CacheFile, cache)

	// Build and return edges from full cache.
	return buildEdgesFromCache(cache, knownComponents)
}

// queryLLMForDocument queries PageIndex for a single document.
func queryLLMForDocument(
	doc Document,
	tree *FileTree,
	knownComponents []string,
	cfg LLMDiscoveryConfig,
) []llmDiscoveryResult {
	if tree == nil || tree.Root == nil {
		return nil
	}

	// Build prompt.
	prompt := buildDiscoveryPrompt(doc.RelPath, tree, knownComponents)

	// Call PageIndex via subprocess.
	cmd := exec.Command(
		cfg.ExecutablePath,
		"query",
		"--query", prompt,
		"--format", "json",
		"--model", cfg.Model,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// If pageindex fails, return empty (graceful degradation).
		return nil
	}

	// Parse JSON response.
	var results []llmDiscoveryResult
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil
	}

	return results
}

// buildDiscoveryPrompt constructs the LLM prompt for discovery.
func buildDiscoveryPrompt(docID string, tree *FileTree, knownComponents []string) string {
	root := tree.Root
	if root == nil {
		return ""
	}

	// Collect top child summaries.
	var childSummaries []string
	for i, child := range root.Children {
		if i >= 3 {
			break // Top 3 only
		}
		if child != nil && child.Summary != "" {
			childSummaries = append(childSummaries, fmt.Sprintf("  - %s: %s", child.Heading, child.Summary))
		}
	}

	// Sort known components for determinism.
	sorted := make([]string, len(knownComponents))
	copy(sorted, knownComponents)
	sort.Strings(sorted)

	sections := []string{
		"You are a software architecture expert analyzing service dependencies.",
		"",
		fmt.Sprintf("Service: %s", docID),
		fmt.Sprintf("Purpose: %s", root.Summary),
	}

	if len(childSummaries) > 0 {
		sections = append(sections, "Key sections:")
		sections = append(sections, childSummaries...)
	}

	sections = append(sections, "")
	sections = append(sections, "Known services in this codebase:")
	for _, comp := range sorted {
		sections = append(sections, fmt.Sprintf("  - %s", comp))
	}

	sections = append(sections, "")
	sections = append(sections, fmt.Sprintf("Which of the above services does \"%s\" directly integrate with?", docID))
	sections = append(sections, "Only list services with EXPLICIT integration evidence in the descriptions above.")
	sections = append(sections, "Generic concepts (APIs, databases, caches) do NOT count unless a specific named service is mentioned.")
	sections = append(sections, "")
	sections = append(sections, "Return JSON only (no markdown, no explanation):")
	sections = append(sections, "[{\"target\": \"service-name\", \"relationship\": \"calls|depends-on|integrates-with\", \"confidence\": 0.0-1.0, \"evidence\": \"exact quote or reasoning\"}]")
	sections = append(sections, "If no integrations found, return: []")

	return strings.Join(sections, "\n")
}

// buildEdgesFromCache converts cached LLM results into DiscoveredEdge values.
func buildEdgesFromCache(cache *llmDiscoveryCache, knownComponents []string) []*DiscoveredEdge {
	if cache == nil || len(cache.Entries) == 0 {
		return nil
	}

	// Build component ID map (normalize: lowercase component name to component ID).
	compMap := make(map[string]string)
	for _, comp := range knownComponents {
		lower := strings.ToLower(comp)
		compMap[lower] = comp
		// Also map hyphenated versions to original.
		compMap[strings.ReplaceAll(lower, "_", "-")] = comp
	}

	var edges []*DiscoveredEdge
	seenKeys := make(map[string]bool) // Deduplicate: source+target+type

	for _, entry := range cache.Entries {
		for _, result := range entry.Results {
			// Map target name to component ID.
			targetID := resolveComponentID(result.Target, compMap)
			if targetID == "" {
				continue // Target not recognized, skip
			}

			// Normalize relationship type.
			edgeType := normalizeRelationshipType(result.Relationship)

			// Use Claude-generated confidence directly — no artificial capping.
			// Claude already assigns appropriate confidence scores; capping at 0.65
			// would prevent high-confidence LLM discoveries from passing the filter.
			conf := result.Confidence

			// Create edge.
			edge, err := NewEdge(entry.DocID, targetID, edgeType, conf, result.Evidence)
			if err != nil {
				continue
			}

			// Deduplicate.
			key := entry.DocID + "\x00" + targetID + "\x00" + string(edgeType)
			if seenKeys[key] {
				continue
			}
			seenKeys[key] = true

			// Wrap as DiscoveredEdge with SignalLLM.
			de := &DiscoveredEdge{
				Edge: edge,
				Signals: []Signal{{
					SourceType: SignalLLM,
					Confidence: conf,
					Evidence:   "llm: " + result.Evidence,
					Weight:     1.0,
				}},
			}
			edges = append(edges, de)
		}
	}

	return edges
}

// resolveComponentID maps a target name (from LLM) to a canonical component ID.
// Returns "" if target is not found.
func resolveComponentID(target string, compMap map[string]string) string {
	if target == "" {
		return ""
	}

	lower := strings.ToLower(strings.TrimSpace(target))
	if id, ok := compMap[lower]; ok {
		return id
	}

	// Try with hyphens/underscores normalized.
	normalized := strings.ReplaceAll(lower, "_", "-")
	if id, ok := compMap[normalized]; ok {
		return id
	}

	return ""
}

// normalizeRelationshipType maps LLM relationship strings to EdgeType.
func normalizeRelationshipType(rel string) EdgeType {
	lower := strings.ToLower(strings.TrimSpace(rel))
	switch {
	case strings.Contains(lower, "calls"):
		return EdgeCalls
	case strings.Contains(lower, "depends"):
		return EdgeDependsOn
	case strings.Contains(lower, "integrates"):
		return EdgeMentions // Use EdgeMentions for generic integrations
	default:
		return EdgeMentions
	}
}

// hashDocContent computes a SHA256 hash of the document content.
func hashDocContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// loadLLMDiscoveryCache loads the cache from disk, returning nil if not found or invalid.
func loadLLMDiscoveryCache(cacheFile string) *llmDiscoveryCache {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil // File doesn't exist or can't be read
	}

	var cache llmDiscoveryCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil // Corrupted cache, start fresh
	}

	return &cache
}

// saveLLMDiscoveryCache persists the cache to disk.
func saveLLMDiscoveryCache(cacheFile string, cache *llmDiscoveryCache) error {
	if cache == nil {
		return nil
	}

	// Ensure directory exists.
	dir := filepath.Dir(cacheFile)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// checkPageIndexAvailable verifies the pageindex binary is available.
// Callers should only invoke this when LLM discovery is explicitly requested via --llm-discovery flag.
func checkPageIndexAvailable(execPath string) error {
	cmd := exec.Command(execPath, "--version")
	if err := cmd.Run(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("pageindex not found: install via 'pip install pageindex': %w", ErrPageIndexNotFound)
		}
		// If it's an exec.ExitError, the binary exists but --version failed; that's OK.
		return nil
	}
	return nil
}
