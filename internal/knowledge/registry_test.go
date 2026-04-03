package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Task 1.1: Core data structure tests ------------------------------------

func TestNewComponentRegistry(t *testing.T) {
	r := NewComponentRegistry()
	if r == nil {
		t.Fatal("NewComponentRegistry returned nil")
	}
	if r.Components == nil {
		t.Error("Components map is nil")
	}
	if r.Relationships == nil {
		t.Error("Relationships slice is nil")
	}
	if r.index == nil {
		t.Error("index map is nil")
	}
	if r.ComponentCount() != 0 {
		t.Errorf("expected 0 components, got %d", r.ComponentCount())
	}
	if r.RelationshipCount() != 0 {
		t.Errorf("expected 0 relationships, got %d", r.RelationshipCount())
	}
}

func TestAddComponent(t *testing.T) {
	r := NewComponentRegistry()

	comp := &RegistryComponent{
		ID:      "auth-service",
		Name:    "Auth Service",
		FileRef: "services/auth.md",
		Type:    ComponentTypeService,
	}

	if err := r.AddComponent(comp); err != nil {
		t.Fatalf("AddComponent failed: %v", err)
	}
	if r.ComponentCount() != 1 {
		t.Errorf("expected 1 component, got %d", r.ComponentCount())
	}

	// Idempotent: re-adding same ID replaces it.
	comp2 := &RegistryComponent{
		ID:      "auth-service",
		Name:    "Auth Service v2",
		FileRef: "services/auth.md",
	}
	if err := r.AddComponent(comp2); err != nil {
		t.Fatalf("AddComponent (replace) failed: %v", err)
	}
	if r.ComponentCount() != 1 {
		t.Errorf("expected 1 component after replace, got %d", r.ComponentCount())
	}
	got := r.GetComponent("auth-service")
	if got.Name != "Auth Service v2" {
		t.Errorf("expected replaced name %q, got %q", "Auth Service v2", got.Name)
	}
}

func TestAddComponent_Validation(t *testing.T) {
	r := NewComponentRegistry()

	if err := r.AddComponent(nil); err == nil {
		t.Error("expected error for nil component")
	}

	if err := r.AddComponent(&RegistryComponent{}); err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestAddComponent_SetsDetectedAt(t *testing.T) {
	r := NewComponentRegistry()
	comp := &RegistryComponent{
		ID:   "svc",
		Name: "Svc",
	}
	before := time.Now()
	_ = r.AddComponent(comp)
	after := time.Now()

	got := r.GetComponent("svc")
	if got.DetectedAt.IsZero() {
		t.Error("DetectedAt should be set automatically")
	}
	if got.DetectedAt.Before(before) || got.DetectedAt.After(after) {
		t.Errorf("DetectedAt %v outside expected range [%v, %v]", got.DetectedAt, before, after)
	}
}

func TestAddSignal_Basic(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddComponent(&RegistryComponent{ID: "auth", Name: "Auth"})
	_ = r.AddComponent(&RegistryComponent{ID: "user", Name: "User"})

	sig := Signal{
		SourceType: SignalLink,
		Confidence: 1.0,
		Evidence:   "[auth](auth.md)",
		Weight:     1.0,
	}

	if err := r.AddSignal("auth", "user", sig); err != nil {
		t.Fatalf("AddSignal failed: %v", err)
	}
	if r.RelationshipCount() != 1 {
		t.Errorf("expected 1 relationship, got %d", r.RelationshipCount())
	}
}

func TestAddSignal_Deduplication(t *testing.T) {
	r := NewComponentRegistry()

	sig := Signal{SourceType: SignalLink, Confidence: 1.0, Evidence: "link", Weight: 1.0}
	_ = r.AddSignal("a", "b", sig)
	// Re-adding same signal should not duplicate.
	_ = r.AddSignal("a", "b", sig)

	rels := r.FindRelationships("a")
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(rels))
	}
	if len(rels[0].Signals) != 1 {
		t.Errorf("expected 1 signal (deduped), got %d", len(rels[0].Signals))
	}
}

func TestAddSignal_MultipleSignals(t *testing.T) {
	r := NewComponentRegistry()

	_ = r.AddSignal("a", "b", Signal{SourceType: SignalLink, Confidence: 1.0, Evidence: "link", Weight: 1.0})
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalMention, Confidence: 0.7, Evidence: "mentions", Weight: 1.0})

	rels := r.FindRelationships("a")
	if len(rels[0].Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(rels[0].Signals))
	}
}

func TestAddSignal_Validation(t *testing.T) {
	r := NewComponentRegistry()

	if err := r.AddSignal("", "b", Signal{}); err == nil {
		t.Error("expected error for empty fromID")
	}
	if err := r.AddSignal("a", "", Signal{}); err == nil {
		t.Error("expected error for empty toID")
	}
	if err := r.AddSignal("a", "a", Signal{}); err == nil {
		t.Error("expected error for self-relationship")
	}
}

func TestAggregateConfidence_MaxSignalWins(t *testing.T) {
	r := NewComponentRegistry()

	_ = r.AddSignal("a", "b", Signal{SourceType: SignalMention, Confidence: 0.6, Weight: 1.0})
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalLink, Confidence: 1.0, Weight: 1.0})
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalLLM, Confidence: 0.65, Weight: 1.0})

	r.AggregateConfidence()

	rels := r.FindRelationships("a")
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(rels))
	}
	if rels[0].AggregatedConfidence != 1.0 {
		t.Errorf("expected aggregated confidence 1.0, got %.2f", rels[0].AggregatedConfidence)
	}
}

func TestAggregateConfidence_SingleLowConfidence(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalMention, Confidence: 0.6, Weight: 1.0})

	r.AggregateConfidence()
	rels := r.FindRelationships("a")
	if rels[0].AggregatedConfidence != 0.6 {
		t.Errorf("expected 0.6, got %.2f", rels[0].AggregatedConfidence)
	}
}

func TestAggregateConfidence_CappedAt1(t *testing.T) {
	r := NewComponentRegistry()
	// Weight > 1 should cap at 1.0.
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalLink, Confidence: 1.0, Weight: 2.0})
	r.AggregateConfidence()
	rels := r.FindRelationships("a")
	if rels[0].AggregatedConfidence > 1.0 {
		t.Errorf("confidence should be capped at 1.0, got %.2f", rels[0].AggregatedConfidence)
	}
}

// --- Task 1.1: Component retrieval tests ------------------------------------

func TestGetComponent(t *testing.T) {
	r := NewComponentRegistry()
	comp := &RegistryComponent{ID: "svc", Name: "My Service", Type: ComponentTypeService}
	_ = r.AddComponent(comp)

	got := r.GetComponent("svc")
	if got == nil {
		t.Fatal("GetComponent returned nil for existing component")
	}
	if got.Name != "My Service" {
		t.Errorf("expected %q, got %q", "My Service", got.Name)
	}

	// Missing component returns nil.
	if r.GetComponent("missing") != nil {
		t.Error("expected nil for missing component")
	}
}

func TestFindByName(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddComponent(&RegistryComponent{ID: "auth-svc", Name: "Auth Service"})

	got := r.FindByName("auth service")
	if got == nil {
		t.Fatal("FindByName returned nil for case-insensitive match")
	}
	if got.ID != "auth-svc" {
		t.Errorf("expected ID %q, got %q", "auth-svc", got.ID)
	}

	if r.FindByName("nonexistent") != nil {
		t.Error("expected nil for non-existent name")
	}
}

// --- Task 1.2: InitFromGraph tests ------------------------------------------

func TestInitFromGraph_Basic(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "services/auth.md", Title: "Auth Service"})
	_ = g.AddNode(&Node{ID: "services/user.md", Title: "User Service"})

	edge, _ := NewEdge("services/auth.md", "services/user.md", EdgeReferences, 1.0, "[user](user.md)")
	_ = g.AddEdge(edge)

	docs := []Document{}

	r := NewComponentRegistry()
	r.InitFromGraph(g, docs)

	if r.ComponentCount() != 2 {
		t.Errorf("expected 2 components, got %d", r.ComponentCount())
	}
	if r.RelationshipCount() != 1 {
		t.Errorf("expected 1 relationship, got %d", r.RelationshipCount())
	}
}

func TestInitFromGraph_EdgeConfidences(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a.md", Title: "A"})
	_ = g.AddNode(&Node{ID: "b.md", Title: "B"})

	edge, _ := NewEdge("a.md", "b.md", EdgeReferences, ConfidenceLink, "")
	_ = g.AddEdge(edge)

	r := NewComponentRegistry()
	r.InitFromGraph(g, nil)

	rels := r.FindRelationships("a")
	if len(rels) == 0 {
		t.Fatal("expected at least 1 relationship for a → b")
	}
	if rels[0].AggregatedConfidence != ConfidenceLink {
		t.Errorf("expected confidence %.2f, got %.2f", ConfidenceLink, rels[0].AggregatedConfidence)
	}
}

func TestInitFromGraph_ComponentTypes(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "services/auth-service.md", Title: "Auth Service"})
	_ = g.AddNode(&Node{ID: "api/gateway.md", Title: "Gateway"})
	_ = g.AddNode(&Node{ID: "config/settings.md", Title: "Settings"})
	_ = g.AddNode(&Node{ID: "docs/readme.md", Title: "ReadMe"})

	r := NewComponentRegistry()
	r.InitFromGraph(g, nil)

	tests := []struct {
		id       string
		wantType ComponentType
	}{
		{"auth-service", ComponentTypeService},
		{"gateway", ComponentTypeAPI},
		{"settings", ComponentTypeConfig},
		{"readme", ComponentTypeUnknown},
	}
	for _, tt := range tests {
		comp := r.GetComponent(tt.id)
		if comp == nil {
			t.Errorf("component %q not found", tt.id)
			continue
		}
		if comp.Type != tt.wantType {
			t.Errorf("component %q: expected type %q, got %q", tt.id, tt.wantType, comp.Type)
		}
	}
}

// --- Task 1.3: Query and serialization tests --------------------------------

func TestQueryByConfidence(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalLink, Confidence: 1.0, Weight: 1.0})
	_ = r.AddSignal("a", "c", Signal{SourceType: SignalMention, Confidence: 0.6, Weight: 1.0})
	_ = r.AddSignal("a", "d", Signal{SourceType: SignalLLM, Confidence: 0.3, Weight: 1.0})

	// Threshold 0.5 should return 2 relationships.
	results := r.QueryByConfidence(0.5)
	if len(results) != 2 {
		t.Errorf("expected 2 results for threshold 0.5, got %d", len(results))
	}

	// Threshold 1.0 should return only the link relationship.
	results = r.QueryByConfidence(1.0)
	if len(results) != 1 {
		t.Errorf("expected 1 result for threshold 1.0, got %d", len(results))
	}

	// Threshold 0.0 should return all.
	results = r.QueryByConfidence(0.0)
	if len(results) != 3 {
		t.Errorf("expected 3 results for threshold 0.0, got %d", len(results))
	}
}

func TestQueryByConfidence_SortedDescending(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddSignal("a", "b", Signal{SourceType: SignalMention, Confidence: 0.6, Weight: 1.0})
	_ = r.AddSignal("a", "c", Signal{SourceType: SignalLink, Confidence: 1.0, Weight: 1.0})
	_ = r.AddSignal("a", "d", Signal{SourceType: SignalLLM, Confidence: 0.65, Weight: 1.0})

	results := r.QueryByConfidence(0.0)
	for i := 1; i < len(results); i++ {
		if results[i].AggregatedConfidence > results[i-1].AggregatedConfidence {
			t.Errorf("results not sorted descending at index %d: %.2f > %.2f",
				i, results[i].AggregatedConfidence, results[i-1].AggregatedConfidence)
		}
	}
}

func TestFindRelationships(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddSignal("auth", "user", Signal{SourceType: SignalLink, Confidence: 1.0})
	_ = r.AddSignal("auth", "billing", Signal{SourceType: SignalMention, Confidence: 0.7})
	_ = r.AddSignal("user", "billing", Signal{SourceType: SignalLink, Confidence: 1.0})

	authRels := r.FindRelationships("auth")
	if len(authRels) != 2 {
		t.Errorf("expected 2 relationships from auth, got %d", len(authRels))
	}

	userRels := r.FindRelationships("user")
	if len(userRels) != 1 {
		t.Errorf("expected 1 relationship from user, got %d", len(userRels))
	}

	// No outgoing relationships.
	billingRels := r.FindRelationships("billing")
	if len(billingRels) != 0 {
		t.Errorf("expected 0 relationships from billing, got %d", len(billingRels))
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	r := NewComponentRegistry()
	_ = r.AddComponent(&RegistryComponent{
		ID:      "auth",
		Name:    "Auth Service",
		FileRef: "services/auth.md",
		Type:    ComponentTypeService,
	})
	_ = r.AddComponent(&RegistryComponent{
		ID:      "user",
		Name:    "User Service",
		FileRef: "services/user.md",
		Type:    ComponentTypeService,
	})
	_ = r.AddSignal("auth", "user", Signal{
		SourceType: SignalLink,
		Confidence: 1.0,
		Evidence:   "[user](user.md)",
		Weight:     1.0,
	})

	// Serialize.
	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize into new registry.
	r2 := NewComponentRegistry()
	if err := r2.FromJSON(data); err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify components.
	if r2.ComponentCount() != r.ComponentCount() {
		t.Errorf("component count mismatch: want %d, got %d", r.ComponentCount(), r2.ComponentCount())
	}
	if r2.RelationshipCount() != r.RelationshipCount() {
		t.Errorf("relationship count mismatch: want %d, got %d", r.RelationshipCount(), r2.RelationshipCount())
	}

	// Verify index was rebuilt.
	rels := r2.FindRelationships("auth")
	if len(rels) != 1 {
		t.Errorf("expected 1 relationship after deserialization, got %d", len(rels))
	}
}

func TestSaveLoadRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bmd-registry.json")

	r := NewComponentRegistry()
	_ = r.AddComponent(&RegistryComponent{ID: "svc", Name: "Service", Type: ComponentTypeService})
	_ = r.AddSignal("svc", "dep", Signal{SourceType: SignalLink, Confidence: 1.0})

	if err := SaveRegistry(r, path); err != nil {
		t.Fatalf("SaveRegistry failed: %v", err)
	}

	loaded, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadRegistry returned nil")
	}

	if loaded.ComponentCount() != r.ComponentCount() {
		t.Errorf("expected %d components, got %d", r.ComponentCount(), loaded.ComponentCount())
	}
	if loaded.RelationshipCount() != r.RelationshipCount() {
		t.Errorf("expected %d relationships, got %d", r.RelationshipCount(), loaded.RelationshipCount())
	}
}

func TestLoadRegistry_MissingFile(t *testing.T) {
	loaded, err := LoadRegistry("/nonexistent/path/.bmd-registry.json")
	if err != nil {
		t.Fatalf("LoadRegistry should return nil, nil for missing file, got err: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for missing file")
	}
}

func TestLoadRegistry_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bmd-registry.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRegistry(path)
	if err == nil {
		t.Error("expected error for corrupt JSON")
	}
}

// --- nodeToRegistryID helper tests ------------------------------------------

func TestNodeToRegistryID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"services/auth-service.md", "auth-service"},
		{"api/gateway.md", "gateway"},
		{"README.md", "readme"},
		{"docs/overview.md", "overview"},
	}
	for _, tt := range tests {
		got := nodeToRegistryID(tt.input)
		if got != tt.want {
			t.Errorf("nodeToRegistryID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- inferComponentType helper tests ----------------------------------------

func TestInferComponentType(t *testing.T) {
	tests := []struct {
		input string
		want  ComponentType
	}{
		{"services/auth-service.md", ComponentTypeService},
		{"api/gateway.md", ComponentTypeAPI},
		{"config/settings.md", ComponentTypeConfig},
		{"store/orders.md", ComponentTypeDatabase},
		{"db/schema.md", ComponentTypeDatabase},
		{"docs/readme.md", ComponentTypeUnknown},
	}
	for _, tt := range tests {
		got := inferComponentType(tt.input)
		if got != tt.want {
			t.Errorf("inferComponentType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
