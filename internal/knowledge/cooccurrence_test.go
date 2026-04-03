package knowledge

import (
	"testing"
	"time"
)

func TestCoOccurrenceRelationships_BasicDetection(t *testing.T) {
	docs := []Document{
		{
			ID:        "services/order-service.md",
			RelPath:   "services/order-service.md",
			Title:     "Order Service",
			PlainText: "The Order Service coordinates with User Service for authentication.\nPayment Service processes the payments.\nThis creates a seamless workflow.",
		},
	}

	componentNames := map[string]string{
		"Order Service":   "services/order-service.md",
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
	}

	cfg := DefaultCoOccurrenceConfig()
	edges := CoOccurrenceRelationships(docs, componentNames, cfg)

	if len(edges) == 0 {
		t.Fatal("expected at least one co-occurrence edge, got none")
	}

	// Verify that doc→User Service and doc→Payment Service edges exist.
	foundUser := false
	foundPayment := false
	for _, e := range edges {
		if e.Target == "services/user-service.md" {
			foundUser = true
		}
		if e.Target == "services/payment-service.md" {
			foundPayment = true
		}
	}

	if !foundUser {
		t.Error("expected edge to User Service, not found")
	}
	if !foundPayment {
		t.Error("expected edge to Payment Service, not found")
	}
}

func TestCoOccurrenceRelationships_SkipsSelfReference(t *testing.T) {
	docs := []Document{
		{
			ID:        "services/user-service.md",
			RelPath:   "services/user-service.md",
			Title:     "User Service",
			PlainText: "The User Service manages authentication.\nUser Service handles JWT tokens.",
		},
	}

	componentNames := map[string]string{
		"User Service": "services/user-service.md",
	}

	cfg := DefaultCoOccurrenceConfig()
	edges := CoOccurrenceRelationships(docs, componentNames, cfg)

	for _, e := range edges {
		if e.Source == e.Target {
			t.Errorf("self-loop found: %s → %s", e.Source, e.Target)
		}
	}
}

func TestCoOccurrenceRelationships_EmptyDocuments(t *testing.T) {
	edges := CoOccurrenceRelationships(nil, nil, DefaultCoOccurrenceConfig())
	if len(edges) != 0 {
		t.Errorf("expected no edges for nil documents, got %d", len(edges))
	}
}

func TestCoOccurrenceConfidence_EarlySection(t *testing.T) {
	c := coOccurrenceConfidence(0, 100)
	if c < 0.60 {
		t.Errorf("expected high confidence for early section, got %.2f", c)
	}
}

func TestCoOccurrenceConfidence_LateSection(t *testing.T) {
	c := coOccurrenceConfidence(80, 100)
	if c > 0.50 {
		t.Errorf("expected lower confidence for late section, got %.2f", c)
	}
}

func TestCoOccurrenceRelationships_MultipleComponentsInWindow(t *testing.T) {
	docs := []Document{
		{
			ID:        "README.md",
			RelPath:   "README.md",
			Title:     "README",
			PlainText: "The system uses User Service and Payment Service together.\nOrder Service handles the workflow.",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
		"Order Service":   "services/order-service.md",
	}

	cfg := CoOccurrenceConfig{WindowSize: 3}
	edges := CoOccurrenceRelationships(docs, componentNames, cfg)

	if len(edges) < 2 {
		t.Errorf("expected at least 2 edges from multiple co-occurrences, got %d", len(edges))
	}

	// All edges should have signals.
	for _, e := range edges {
		if len(e.Signals) == 0 {
			t.Errorf("edge %s → %s has no signals", e.Source, e.Target)
		}
		if e.Signals[0].SourceType != SignalCoOccurrence {
			t.Errorf("expected SignalCoOccurrence, got %s", e.Signals[0].SourceType)
		}
	}
}

func TestCoOccurrenceRelationships_CustomWindowSize(t *testing.T) {
	docs := []Document{
		{
			ID:      "doc.md",
			RelPath: "doc.md",
			Title:   "Doc",
			PlainText: "Line 1 mentions User Service.\n" +
				"Line 2.\n" +
				"Line 3.\n" +
				"Line 4.\n" +
				"Line 5.\n" +
				"Line 6.\n" +
				"Line 7.\n" +
				"Line 8.\n" +
				"Line 9.\n" +
				"Line 10 mentions Payment Service.",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
	}

	// Small window should not catch both.
	smallCfg := CoOccurrenceConfig{WindowSize: 3}
	edgesSmall := CoOccurrenceRelationships(docs, componentNames, smallCfg)

	// Large window should catch both.
	largeCfg := CoOccurrenceConfig{WindowSize: 12}
	edgesLarge := CoOccurrenceRelationships(docs, componentNames, largeCfg)

	if len(edgesLarge) < len(edgesSmall) {
		t.Errorf("larger window should produce at least as many edges: small=%d, large=%d",
			len(edgesSmall), len(edgesLarge))
	}
}

func TestTruncateEvidence(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 100, "short"},
		{"a very long string that should be truncated at some point", 20, "a very long strin..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncateEvidence(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateEvidence(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestCoOccurrenceRelationships_DeduplicatesAcrossDocuments(t *testing.T) {
	docs := []Document{
		{
			ID:        "doc1.md",
			RelPath:   "doc1.md",
			Title:     "Doc 1",
			PlainText: "Uses User Service and Payment Service together.",
		},
		{
			ID:        "doc2.md",
			RelPath:   "doc2.md",
			Title:     "Doc 2",
			PlainText: "Also uses User Service and Payment Service together.",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
	}

	cfg := DefaultCoOccurrenceConfig()
	edges := CoOccurrenceRelationships(docs, componentNames, cfg)

	// The cross-component edge (User→Payment) should only appear once.
	crossCount := 0
	for _, e := range edges {
		if e.Source == "services/user-service.md" && e.Target == "services/payment-service.md" {
			crossCount++
		}
	}
	if crossCount > 1 {
		t.Errorf("expected deduplicated cross-component edge, got %d occurrences", crossCount)
	}
}

func makeDiscoveryDoc(id, title, plainText string) Document {
	return Document{
		ID:           id,
		Path:         "/test/" + id,
		RelPath:      id,
		Title:        title,
		Content:      plainText,
		PlainText:    plainText,
		LastModified: time.Now(),
	}
}
