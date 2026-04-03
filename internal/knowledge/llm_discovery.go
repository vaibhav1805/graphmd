package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// claudeClient is the interface used for Claude API calls. It is defined as an
// interface so tests can supply a mock without making real network calls.
type claudeClient interface {
	newMessage(ctx context.Context, model, prompt string, maxTokens int) (string, error)
}

// realClaudeClient wraps the official Anthropic SDK client.
type realClaudeClient struct {
	client anthropic.Client
}

// newMessage calls the Claude API and returns the text content of the first
// content block. It returns an error on network failure, API error, or when the
// response contains no text content.
func (r *realClaudeClient) newMessage(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	resp, err := r.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(prompt),
			},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("claude API error: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("claude returned empty content")
	}
	text := resp.Content[0].Text
	if text == "" {
		return "", fmt.Errorf("claude returned no text in first content block")
	}
	return text, nil
}

// newClaudeClient creates a realClaudeClient using the ANTHROPIC_API_KEY
// environment variable (or ANTHROPIC_KEY as fallback). Returns an error when the key is not set.
func newClaudeClient(apiKey string) (*realClaudeClient, error) {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY (or ANTHROPIC_KEY) not set; export one and retry")
	}
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &realClaudeClient{client: c}, nil
}

// GenerateSectionSummary calls Claude to produce a 1-2 sentence technical
// summary for the given TreeNode. The model parameter selects which Claude
// model to use (e.g. "claude-haiku-4-5-20251001"). When client is nil the
// function falls back to creating a new realClaudeClient from the environment.
//
// Returns ("", err) on failure — callers should treat this as a soft error
// (keep the node's existing summary rather than failing the whole discovery).
func GenerateSectionSummary(node *TreeNode, model string, client claudeClient) (string, error) {
	if node == nil {
		return "", fmt.Errorf("GenerateSectionSummary: node is nil")
	}
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	// Build prompt from node heading + content snippet.
	var sb strings.Builder
	sb.WriteString("Summarize this markdown section in 1-2 sentences, focusing on technical purpose.\n\n")
	if node.Heading != "" {
		sb.WriteString("Section: ")
		sb.WriteString(node.Heading)
		sb.WriteString("\n")
	}
	if node.Content != "" {
		preview := node.Content
		if len(preview) > 500 {
			preview = preview[:500]
		}
		sb.WriteString("Content:\n")
		sb.WriteString(preview)
		sb.WriteString("\n")
	} else if node.Summary != "" {
		sb.WriteString("Existing summary: ")
		sb.WriteString(node.Summary)
		sb.WriteString("\n")
	}
	sb.WriteString("\nRespond with the summary only, no preamble.")

	if client == nil {
		var err error
		client, err = newClaudeClient("")
		if err != nil {
			return "", err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return client.newMessage(ctx, model, sb.String(), 256)
}

// DiscoverWithClaude performs LLM-powered relationship discovery by calling
// Claude directly (no subprocess). It builds prompts from the document trees
// and asks Claude which known components each document integrates with.
//
// Parameters:
//
//	docs           — corpus of markdown documents
//	trees          — hierarchical FileTree for each document (built by TreeNodeFromMarkdown)
//	components     — canonical component IDs for reasoning (e.g. ["auth-service", "db"])
//	cfg            — LLMDiscoveryConfig controlling model, concurrency, and cache
//	client         — optional claudeClient for testing; nil → use realClaudeClient from env
//
// Returns a slice of DiscoveredEdge with SignalLLM signals. Requires
// ANTHROPIC_API_KEY in the environment when client is nil.
func DiscoverWithClaude(
	docs []Document,
	trees []FileTree,
	components []string,
	cfg LLMDiscoveryConfig,
	client claudeClient,
) []*DiscoveredEdge {
	if len(docs) == 0 || len(trees) == 0 || len(components) == 0 {
		return nil
	}

	// Lazy-initialize the real client only if caller didn't supply one.
	if client == nil {
		rc, createErr := newClaudeClient("")
		if createErr != nil {
			// No API key: degrade gracefully (no edges).
			return nil
		}
		client = rc
	}

	// Build tree index.
	treeByDocID := make(map[string]*FileTree)
	for i := range trees {
		treeByDocID[trees[i].File] = &trees[i]
	}

	// Load or initialise cache.
	cache := loadLLMDiscoveryCache(cfg.CacheFile)
	if cache == nil {
		cache = &llmDiscoveryCache{
			Entries:     []llmDiscoveryCacheEntry{},
			GeneratedAt: time.Now(),
		}
	}

	cacheByDocID := make(map[string]*llmDiscoveryCacheEntry)
	for i := range cache.Entries {
		cacheByDocID[cache.Entries[i].DocID] = &cache.Entries[i]
	}

	// Determine which docs need new LLM calls.
	type work struct {
		doc  Document
		tree *FileTree
		hash string
	}
	var pending []work
	for _, doc := range docs {
		tree, ok := treeByDocID[doc.RelPath]
		if !ok {
			continue
		}
		hash := hashDocContent(doc.Content)
		if cfg.SkipExisting {
			if cached, hit := cacheByDocID[doc.RelPath]; hit && cached.ContentHash == hash {
				continue
			}
		}
		pending = append(pending, work{doc: doc, tree: tree, hash: hash})
	}

	if len(pending) == 0 {
		return buildEdgesFromCache(cache, components)
	}

	// Process pending docs with concurrency limit.
	sem := make(chan struct{}, max1(cfg.MaxConcurrency))
	type queryResult struct {
		entry llmDiscoveryCacheEntry
	}
	resultCh := make(chan queryResult, len(pending))

	for _, w := range pending {
		sem <- struct{}{}
		go func(w work) {
			defer func() { <-sem }()

			llmResults := queryClaudeForDocument(w.doc, w.tree, components, cfg.Model, client)
			resultCh <- queryResult{
				entry: llmDiscoveryCacheEntry{
					DocID:       w.doc.RelPath,
					ContentHash: w.hash,
					Results:     llmResults,
					GeneratedAt: time.Now(),
				},
			}
		}(w)
	}

	// Collect results (exactly len(pending) items will arrive).
	for range pending {
		r := <-resultCh
		// Replace old cache entry if present.
		newEntries := make([]llmDiscoveryCacheEntry, 0, len(cache.Entries))
		for _, e := range cache.Entries {
			if e.DocID != r.entry.DocID {
				newEntries = append(newEntries, e)
			}
		}
		cache.Entries = newEntries
		cache.Entries = append(cache.Entries, r.entry)
		cacheByDocID[r.entry.DocID] = &cache.Entries[len(cache.Entries)-1]
	}

	_ = saveLLMDiscoveryCache(cfg.CacheFile, cache)
	return buildEdgesFromCache(cache, components)
}

// queryClaudeForDocument sends a discovery prompt for a single document and
// returns the parsed relationships. Returns nil on error (graceful degradation).
func queryClaudeForDocument(
	doc Document,
	tree *FileTree,
	components []string,
	model string,
	client claudeClient,
) []llmDiscoveryResult {
	if tree == nil || tree.Root == nil {
		return nil
	}
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	prompt := buildDiscoveryPrompt(doc.RelPath, tree, components)
	if prompt == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, err := client.newMessage(ctx, model, prompt, 1024)
	if err != nil {
		// Graceful degradation: no relationships for this doc.
		return nil
	}

	// Strip any markdown fences the model may add around JSON.
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var results []llmDiscoveryResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		return nil
	}

	return results
}

// max1 returns n when n >= 1, else 1.
func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
