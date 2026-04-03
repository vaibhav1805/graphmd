package knowledge

import (
	"testing"
)

func TestStructuralRelationships_DependencySection(t *testing.T) {
	docs := []Document{
		{
			ID:      "services/order-service.md",
			RelPath: "services/order-service.md",
			Title:   "Order Service",
			Content: "# Order Service\n\n## Dependencies\n\n- **User Service**: validates user ownership\n- **Payment Service**: processes payments\n\n## Overview\n\nSome content here.",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
		"Order Service":   "services/order-service.md",
	}

	edges := StructuralRelationships(docs, componentNames)

	if len(edges) == 0 {
		t.Fatal("expected structural edges from Dependencies section, got none")
	}

	foundUser := false
	foundPayment := false
	for _, e := range edges {
		if e.Source == "services/order-service.md" && e.Target == "services/user-service.md" {
			foundUser = true
			if e.Type != EdgeDependsOn {
				t.Errorf("expected EdgeDependsOn for dependency section, got %s", e.Type)
			}
			if e.Confidence < 0.75 {
				t.Errorf("expected confidence >= 0.75, got %.2f", e.Confidence)
			}
		}
		if e.Source == "services/order-service.md" && e.Target == "services/payment-service.md" {
			foundPayment = true
		}
	}

	if !foundUser {
		t.Error("expected edge from Order Service → User Service")
	}
	if !foundPayment {
		t.Error("expected edge from Order Service → Payment Service")
	}
}

func TestStructuralRelationships_IntegrationSection(t *testing.T) {
	docs := []Document{
		{
			ID:      "services/payment-service.md",
			RelPath: "services/payment-service.md",
			Title:   "Payment Service",
			Content: "# Payment Service\n\n## Integration Points\n\n- User Service - Validates customer information\n- Order Service - Processes payment for orders\n",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
		"Order Service":   "services/order-service.md",
	}

	edges := StructuralRelationships(docs, componentNames)

	if len(edges) < 2 {
		t.Errorf("expected at least 2 edges from Integration Points section, got %d", len(edges))
	}

	for _, e := range edges {
		if e.Source != "services/payment-service.md" {
			t.Errorf("all edges should originate from payment-service, got source=%s", e.Source)
		}
		if len(e.Signals) == 0 {
			t.Error("edge has no signals")
		}
		if e.Signals[0].SourceType != SignalStructural {
			t.Errorf("expected SignalStructural, got %s", e.Signals[0].SourceType)
		}
	}
}

func TestStructuralRelationships_RelatedServicesSection(t *testing.T) {
	docs := []Document{
		{
			ID:      "services/user-service.md",
			RelPath: "services/user-service.md",
			Title:   "User Service",
			Content: "# User Service\n\n## Related Services\n\n- Order Service - Requires user authentication\n- Payment Service - Validates user payment methods\n",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
		"Order Service":   "services/order-service.md",
	}

	edges := StructuralRelationships(docs, componentNames)

	if len(edges) < 2 {
		t.Errorf("expected at least 2 edges from Related Services section, got %d", len(edges))
	}
}

func TestStructuralRelationships_SkipsSelfReference(t *testing.T) {
	docs := []Document{
		{
			ID:      "services/user-service.md",
			RelPath: "services/user-service.md",
			Title:   "User Service",
			Content: "# User Service\n\n## Dependencies\n\n- User Service - self reference should be skipped\n",
		},
	}

	componentNames := map[string]string{
		"User Service": "services/user-service.md",
	}

	edges := StructuralRelationships(docs, componentNames)

	for _, e := range edges {
		if e.Source == e.Target {
			t.Errorf("self-loop found: %s → %s", e.Source, e.Target)
		}
	}
}

func TestStructuralRelationships_NonDependencySection(t *testing.T) {
	docs := []Document{
		{
			ID:      "docs/overview.md",
			RelPath: "docs/overview.md",
			Title:   "Overview",
			Content: "# Overview\n\n## Architecture\n\nUser Service handles authentication.\nPayment Service processes payments.\n",
		},
	}

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
	}

	edges := StructuralRelationships(docs, componentNames)

	// Non-dependency sections should not produce structural edges.
	if len(edges) != 0 {
		t.Errorf("expected no edges from non-dependency section, got %d", len(edges))
	}
}

func TestStructuralRelationships_EmptyDocuments(t *testing.T) {
	edges := StructuralRelationships(nil, nil)
	if len(edges) != 0 {
		t.Errorf("expected no edges for nil documents, got %d", len(edges))
	}
}

func TestParseHeadingSections(t *testing.T) {
	content := "# Title\n\nIntro paragraph.\n\n## Dependencies\n\n- Dep A\n- Dep B\n\n## Overview\n\nSome overview."

	sections := parseHeadingSections(content)

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	if sections[0].heading != "Title" {
		t.Errorf("section 0 heading = %q, want %q", sections[0].heading, "Title")
	}
	if sections[0].level != 1 {
		t.Errorf("section 0 level = %d, want 1", sections[0].level)
	}
	if sections[1].heading != "Dependencies" {
		t.Errorf("section 1 heading = %q, want %q", sections[1].heading, "Dependencies")
	}
	if sections[2].heading != "Overview" {
		t.Errorf("section 2 heading = %q, want %q", sections[2].heading, "Overview")
	}
}

func TestClassifyHeadingToEdgeType(t *testing.T) {
	tests := []struct {
		heading  string
		expected EdgeType
	}{
		{"Dependencies", EdgeDependsOn},
		{"Integration Points", EdgeMentions},
		{"Calls", EdgeCalls},
		{"Implements", EdgeImplements},
		{"Related Services", EdgeMentions},
		{"Prerequisites", EdgeDependsOn},
		{"Requires", EdgeDependsOn},
		{"Random Section", EdgeMentions},
	}

	for _, tt := range tests {
		got := classifyHeadingToEdgeType(tt.heading)
		if got != tt.expected {
			t.Errorf("classifyHeadingToEdgeType(%q) = %s, want %s", tt.heading, got, tt.expected)
		}
	}
}

func TestIsDependencySection(t *testing.T) {
	tests := []struct {
		heading  string
		expected bool
	}{
		{"Dependencies", true},
		{"Integration Points", true},
		{"Related Services", true},
		{"Requires", true},
		{"Overview", false},
		{"API Endpoints", false},
		{"Configuration", false},
	}

	for _, tt := range tests {
		got := isDependencySection(tt.heading)
		if got != tt.expected {
			t.Errorf("isDependencySection(%q) = %v, want %v", tt.heading, got, tt.expected)
		}
	}
}

func TestExtractFromTables(t *testing.T) {
	body := `
| Service | Depends On |
|---------|-----------|
| Order Service | User Service |
| Order Service | Payment Service |
`

	componentNames := map[string]string{
		"User Service":    "services/user-service.md",
		"Payment Service": "services/payment-service.md",
		"Order Service":   "services/order-service.md",
	}

	edges := extractFromTables("docs/overview.md", body, componentNames, EdgeDependsOn)

	if len(edges) == 0 {
		t.Fatal("expected edges from table extraction, got none")
	}

	// Should find references to both User Service and Payment Service.
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
		t.Error("expected table edge to User Service")
	}
	if !foundPayment {
		t.Error("expected table edge to Payment Service")
	}
}

func TestStructuralRelationships_HighConfidence(t *testing.T) {
	docs := []Document{
		{
			ID:      "services/order-service.md",
			RelPath: "services/order-service.md",
			Title:   "Order Service",
			Content: "# Order Service\n\n## Dependencies\n\n- User Service: authentication\n",
		},
	}

	componentNames := map[string]string{
		"User Service":  "services/user-service.md",
		"Order Service": "services/order-service.md",
	}

	edges := StructuralRelationships(docs, componentNames)

	for _, e := range edges {
		if e.Confidence < 0.75 {
			t.Errorf("structural edge confidence should be >= 0.75, got %.2f", e.Confidence)
		}
	}
}
