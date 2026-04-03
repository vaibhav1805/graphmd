package knowledge

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// canonicalizeEdges produces a stable string representation of a DiscoveredEdge slice
// for byte-for-byte comparison across runs. Sorts edges by source+target+type.
func canonicalizeEdges(edges []*DiscoveredEdge) string {
	var keys []string
	for _, de := range edges {
		if de.Edge != nil {
			keys = append(keys, de.Source+"\x00"+de.Target+"\x00"+string(de.Type))
		}
	}
	sort.Strings(keys)
	return fmt.Sprintf("n=%d edges=%s", len(keys), strings.Join(keys, "|"))
}

// TestDiscoveryDeterministic_10Runs verifies that DiscoverAndIntegrateRelationships
// produces byte-for-byte identical edge sets across 10 consecutive calls on identical input.
// This is a property test for the determinism fix.
func TestDiscoveryDeterministic_10Runs(t *testing.T) {
	docs := buildOrchTestDocs() // 3-doc synthetic corpus with cross-references

	var first string
	for i := 0; i < 10; i++ {
		llmCfg := DefaultLLMDiscoveryConfig()
		llmCfg.ExecutablePath = "/nonexistent/pageindex" // Avoid actual subprocess calls
		filtered, all := DiscoverAndIntegrateRelationships(docs, nil, nil, DefaultDiscoveryFilterConfig(), llmCfg, []FileTree{}, []string{})
		_ = filtered
		got := canonicalizeEdges(all)
		if first == "" {
			first = got
		} else if got != first {
			t.Fatalf("non-deterministic output at run %d:\n  run 0: %s\n  run %d: %s",
				i, first, i, got)
		}
	}
	// Also verify filtered output is deterministic
	var firstFiltered string
	for i := 0; i < 10; i++ {
		llmCfg := DefaultLLMDiscoveryConfig()
		llmCfg.ExecutablePath = "/nonexistent/pageindex"
		filtered, _ := DiscoverAndIntegrateRelationships(docs, nil, nil, DefaultDiscoveryFilterConfig(), llmCfg, []FileTree{}, []string{})
		got := canonicalizeEdges(filtered)
		if firstFiltered == "" {
			firstFiltered = got
		} else if got != firstFiltered {
			t.Fatalf("non-deterministic filtered output at run %d:\n  run 0: %s\n  run %d: %s",
				i, firstFiltered, i, got)
		}
	}
}

// TestNERDeterministic_10Runs verifies that NERRelationships produces identical
// edge content across 10 runs on identical input.
func TestNERDeterministic_10Runs(t *testing.T) {
	docs := buildOrchTestDocs()

	canonicalizeNER := func(edges []*Edge) string {
		var keys []string
		for _, e := range edges {
			keys = append(keys, e.Source+"\x00"+e.Target+"\x00"+string(e.Type))
		}
		sort.Strings(keys)
		return fmt.Sprintf("n=%d edges=%s", len(keys), strings.Join(keys, "|"))
	}

	var first string
	for i := 0; i < 10; i++ {
		edges := NERRelationships(docs)
		got := canonicalizeNER(edges)
		if first == "" {
			first = got
		} else if got != first {
			t.Fatalf("NER non-deterministic at run %d:\n  run 0: %s\n  run %d: %s",
				i, first, i, got)
		}
	}
}

// TestCoOccurrenceDeterministic_10Runs verifies that CoOccurrenceRelationships
// produces identical edge content across 10 runs on identical input.
func TestCoOccurrenceDeterministic_10Runs(t *testing.T) {
	docs := buildOrchTestDocs()
	componentNames := BuildComponentNameMap(docs)

	canonicalizeDE := func(edges []*DiscoveredEdge) string {
		var keys []string
		for _, de := range edges {
			if de.Edge != nil {
				keys = append(keys, de.Source+"\x00"+de.Target+"\x00"+string(de.Type))
			}
		}
		sort.Strings(keys)
		return fmt.Sprintf("n=%d edges=%s", len(keys), strings.Join(keys, "|"))
	}

	var first string
	for i := 0; i < 10; i++ {
		edges := CoOccurrenceRelationships(docs, componentNames, DefaultCoOccurrenceConfig())
		got := canonicalizeDE(edges)
		if first == "" {
			first = got
		} else if got != first {
			t.Fatalf("co-occurrence non-deterministic at run %d:\n  run 0: %s\n  run %d: %s",
				i, first, i, got)
		}
	}
}
