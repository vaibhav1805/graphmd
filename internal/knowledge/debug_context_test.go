package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// buildTestComponentGraph creates a ComponentGraph with 3 components and 2 edges:
//
//	payment → auth (depends_on, 0.9)
//	payment → user  (depends_on, 0.8)
//
// Component files are created under tmpDir if tmpDir != "".
func buildDebugTestGraph(t *testing.T, tmpDir string) *ComponentGraph {
	t.Helper()

	comps := []Component{
		{ID: "payment", Name: "Payment Service", File: "payment/service.md", Confidence: ConfidenceConfigured},
		{ID: "auth", Name: "Auth Service", File: "auth/service.md", Confidence: ConfidenceConfigured},
		{ID: "user", Name: "User Service", File: "user/service.md", Confidence: ConfidenceComponentFilename},
	}

	if tmpDir != "" {
		for _, c := range comps {
			dir := filepath.Join(tmpDir, filepath.FromSlash(filepath.Dir(c.File)))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dir, err)
			}
			content := "# " + c.Name + "\n\nDocumentation for " + c.Name + ".\n"
			if err := os.WriteFile(filepath.Join(tmpDir, filepath.FromSlash(c.File)), []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", c.File, err)
			}
		}
	}

	cg := NewComponentGraph(comps)
	_ = cg.AddEdge("payment", "auth", 0.9, "depends_on", []string{"payment calls auth"})
	_ = cg.AddEdge("payment", "user", 0.8, "depends_on", []string{"payment calls user"})
	return cg
}

// ─── NewBFS tests ─────────────────────────────────────────────────────────────

func TestNewBFS_ValidComponent(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, err := NewBFS(cg, "payment", "")
	if err != nil {
		t.Fatalf("NewBFS: unexpected error: %v", err)
	}
	if bfs == nil {
		t.Fatal("NewBFS returned nil")
	}
}

func TestNewBFS_UnknownComponent(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	_, err := NewBFS(cg, "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for unknown component, got nil")
	}
}

func TestNewBFS_NilGraph(t *testing.T) {
	_, err := NewBFS(nil, "payment", "")
	if err == nil {
		t.Fatal("expected error for nil graph, got nil")
	}
}

// ─── Traverse tests ────────────────────────────────────────────────────────────

func TestTraverse_VisitsTargetAndDependencies(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")

	if err := bfs.Traverse(3, 0); err != nil {
		t.Fatalf("Traverse: unexpected error: %v", err)
	}

	// Target + 2 deps = at least 3 components visited.
	if len(bfs.visited) < 3 {
		t.Errorf("visited %d components, want >= 3", len(bfs.visited))
	}

	// payment must be visited.
	if !bfs.visited["payment"] {
		t.Error("expected 'payment' to be visited")
	}
}

func TestTraverse_RespectsMaxDepth(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")

	// Depth 0 means: visit target only, don't enqueue neighbours.
	if err := bfs.Traverse(0, 0); err != nil {
		t.Fatalf("Traverse: unexpected error: %v", err)
	}

	// Only the target should be visited (depth 0 means stop after first node).
	if len(bfs.visited) != 1 {
		t.Errorf("visited %d components at depth 0, want 1", len(bfs.visited))
	}
}

func TestTraverse_CycleDetection(t *testing.T) {
	// Create a cyclic graph: A → B → A
	comps := []Component{
		{ID: "a", Name: "A", File: "a.md", Confidence: ConfidenceConfigured},
		{ID: "b", Name: "B", File: "b.md", Confidence: ConfidenceConfigured},
	}
	cg := NewComponentGraph(comps)
	_ = cg.AddEdge("a", "b", 0.9, "depends_on", nil)
	_ = cg.AddEdge("b", "a", 0.9, "depends_on", nil)

	bfs, _ := NewBFS(cg, "a", "")
	err := bfs.Traverse(5, 0)
	if err != nil {
		t.Fatalf("Traverse cycle: unexpected error: %v", err)
	}
	// Both should be visited without infinite loop.
	if len(bfs.visited) != 2 {
		t.Errorf("cyclic graph: visited %d components, want 2", len(bfs.visited))
	}
}

func TestTraverse_EmptyGraph(t *testing.T) {
	comps := []Component{
		{ID: "solo", Name: "Solo", File: "solo.md", Confidence: ConfidenceConfigured},
	}
	cg := NewComponentGraph(comps)
	bfs, _ := NewBFS(cg, "solo", "")

	if err := bfs.Traverse(3, 0); err != nil {
		t.Fatalf("Traverse single node: unexpected error: %v", err)
	}
	if len(bfs.visited) != 1 {
		t.Errorf("single-node graph: visited %d, want 1", len(bfs.visited))
	}
}

func TestTraverse_RecordsRelationships(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")
	_ = bfs.Traverse(3, 0)

	// payment should have relationships pointing to auth and user.
	rels, ok := bfs.relationships["payment"]
	if !ok {
		t.Fatal("no relationships recorded for 'payment'")
	}

	found := make(map[string]bool)
	for _, r := range rels {
		found[r.To] = true
	}
	if !found["auth"] {
		t.Error("expected 'auth' in payment relationships")
	}
	if !found["user"] {
		t.Error("expected 'user' in payment relationships")
	}
}

func TestTraverse_RecordsUsedByRelationships(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	// Start from 'auth' — it should discover 'payment' as a dependent (used_by).
	bfs, _ := NewBFS(cg, "auth", "")
	_ = bfs.Traverse(3, 0)

	rels, ok := bfs.relationships["auth"]
	if !ok {
		t.Fatal("no relationships recorded for 'auth'")
	}

	found := false
	for _, r := range rels {
		if r.To == "payment" && r.Type == "used_by" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'payment' as used_by for 'auth'")
	}
}

// ─── AggregateDocumentation tests ─────────────────────────────────────────────

func TestAggregateDocumentation_LoadsFile(t *testing.T) {
	tmpDir := t.TempDir()
	cg := buildDebugTestGraph(t, tmpDir)
	bfs, _ := NewBFS(cg, "payment", tmpDir)

	doc, err := bfs.AggregateDocumentation("payment", 0)
	if err != nil {
		t.Fatalf("AggregateDocumentation: unexpected error: %v", err)
	}
	if doc == "" {
		t.Error("expected non-empty documentation for 'payment'")
	}
	if doc == "" || len(doc) < 10 {
		t.Errorf("documentation too short: %q", doc)
	}
}

func TestAggregateDocumentation_TruncatesAtMaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	cg := buildDebugTestGraph(t, tmpDir)
	bfs, _ := NewBFS(cg, "payment", tmpDir)

	// Limit to 30 bytes.
	doc, err := bfs.AggregateDocumentation("payment", 30)
	if err != nil {
		t.Fatalf("AggregateDocumentation: unexpected error: %v", err)
	}
	if len(doc) > 50 { // allow some slack for truncation suffix
		t.Errorf("documentation not truncated: len=%d", len(doc))
	}
}

func TestAggregateDocumentation_MissingFile(t *testing.T) {
	cg := buildDebugTestGraph(t, "") // no tmpDir, files don't exist
	bfs, _ := NewBFS(cg, "payment", "/nonexistent/root")

	doc, err := bfs.AggregateDocumentation("payment", 0)
	if err != nil {
		t.Fatalf("AggregateDocumentation: expected nil error for missing file, got %v", err)
	}
	// Should return empty gracefully.
	if doc != "" {
		t.Errorf("expected empty doc for unreadable file, got %q", doc)
	}
}

func TestAggregateDocumentation_UnknownComponent(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")

	doc, err := bfs.AggregateDocumentation("nonexistent", 0)
	if err != nil {
		t.Fatalf("AggregateDocumentation: unexpected error: %v", err)
	}
	if doc != "" {
		t.Errorf("expected empty doc for unknown component, got %q", doc)
	}
}

// ─── BuildDebugContext tests ───────────────────────────────────────────────────

func TestBuildDebugContext_PopulatesAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	cg := buildDebugTestGraph(t, tmpDir)
	bfs, _ := NewBFS(cg, "payment", tmpDir)
	_ = bfs.Traverse(3, 1024*1024)

	dc := bfs.BuildDebugContext("payment", "Why are payments failing?")

	if dc.TargetComponent != "payment" {
		t.Errorf("TargetComponent = %q, want %q", dc.TargetComponent, "payment")
	}
	if dc.QueryDescription != "Why are payments failing?" {
		t.Errorf("QueryDescription = %q, want %q", dc.QueryDescription, "Why are payments failing?")
	}
	if len(dc.Components) == 0 {
		t.Error("Components slice is empty")
	}
	if dc.Stats.ComponentsVisited == 0 {
		t.Error("Stats.ComponentsVisited is 0")
	}
	if dc.Stats.DocumentationSize == 0 {
		t.Error("Stats.DocumentationSize is 0")
	}
	if dc.Stats.StartTime.IsZero() {
		t.Error("Stats.StartTime is zero")
	}
	if dc.Stats.EndTime.IsZero() {
		t.Error("Stats.EndTime is zero")
	}
}

func TestBuildDebugContext_TargetIsFirst(t *testing.T) {
	tmpDir := t.TempDir()
	cg := buildDebugTestGraph(t, tmpDir)
	bfs, _ := NewBFS(cg, "payment", tmpDir)
	_ = bfs.Traverse(3, 1024*1024)

	dc := bfs.BuildDebugContext("payment", "test query")

	if len(dc.Components) == 0 {
		t.Fatal("no components")
	}
	// After sort, target should be first.
	if dc.Components[0].Role != "target" {
		t.Errorf("first component role = %q, want 'target'", dc.Components[0].Role)
	}
}

func TestBuildDebugContext_HasRelationships(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")
	_ = bfs.Traverse(3, 0)

	dc := bfs.BuildDebugContext("payment", "test query")

	if len(dc.Relationships) == 0 {
		t.Error("Relationships map is empty")
	}
}

func TestBuildDebugContext_StatsEdgeCount(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")
	_ = bfs.Traverse(3, 0)

	dc := bfs.BuildDebugContext("payment", "test")

	if dc.Stats.EdgesTraversed == 0 {
		t.Error("Stats.EdgesTraversed is 0, expected > 0")
	}
}

// ─── ToJSON tests ──────────────────────────────────────────────────────────────

func TestToJSON_ValidEnvelope(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")
	_ = bfs.Traverse(3, 0)
	dc := bfs.BuildDebugContext("payment", "test query")

	raw, err := dc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: unexpected error: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("ToJSON returned empty bytes")
	}

	// Validate STATUS-01 envelope structure.
	var resp ContractResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("ToJSON: invalid JSON: %v", err)
	}
	if resp.Status == "" {
		t.Error("envelope missing status field")
	}
	if resp.Message == "" {
		t.Error("envelope missing message field")
	}
}

func TestToJSON_StatusOKWhenComponentsFound(t *testing.T) {
	cg := buildDebugTestGraph(t, "")
	bfs, _ := NewBFS(cg, "payment", "")
	_ = bfs.Traverse(3, 0)
	dc := bfs.BuildDebugContext("payment", "test")

	raw, _ := dc.ToJSON()
	var resp ContractResponse
	_ = json.Unmarshal(raw, &resp)

	if resp.Status != "ok" {
		t.Errorf("status = %q, want 'ok'", resp.Status)
	}
}

func TestToJSON_StatusEmptyWhenNoComponents(t *testing.T) {
	dc := &DebugContext{
		TargetComponent: "missing",
		Components:      nil,
	}

	raw, err := dc.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: unexpected error: %v", err)
	}

	var resp ContractResponse
	_ = json.Unmarshal(raw, &resp)

	if resp.Status != "empty" {
		t.Errorf("status = %q, want 'empty'", resp.Status)
	}
}

// ─── discoveryMethodFromComponent tests ───────────────────────────────────────

func TestDiscoveryMethod_Yaml(t *testing.T) {
	comp := &Component{Confidence: ConfidenceConfigured}
	if discoveryMethodFromComponent(comp) != "yaml" {
		t.Errorf("expected 'yaml' for configured confidence")
	}
}

func TestDiscoveryMethod_Marker(t *testing.T) {
	comp := &Component{Confidence: ConfidenceComponentFilename}
	if discoveryMethodFromComponent(comp) != "marker" {
		t.Errorf("expected 'marker' for filename confidence")
	}
}

func TestDiscoveryMethod_Conventional(t *testing.T) {
	comp := &Component{Confidence: ConfidenceComponentHeading}
	if discoveryMethodFromComponent(comp) != "conventional" {
		t.Errorf("expected 'conventional' for heading confidence")
	}
}

func TestDiscoveryMethod_DepthFallback(t *testing.T) {
	comp := &Component{Confidence: 0.1}
	if discoveryMethodFromComponent(comp) != "depth_fallback" {
		t.Errorf("expected 'depth_fallback' for low confidence")
	}
}

func TestDiscoveryMethod_Nil(t *testing.T) {
	if discoveryMethodFromComponent(nil) != "depth_fallback" {
		t.Errorf("expected 'depth_fallback' for nil component")
	}
}

// ─── sortComponentInfos tests ─────────────────────────────────────────────────

func TestSortComponentInfos_TargetFirst(t *testing.T) {
	infos := []ComponentInfo{
		{Name: "auth", Role: "dependency", DiscoveryDistance: 1},
		{Name: "payment", Role: "target", DiscoveryDistance: 0},
		{Name: "user", Role: "dependency", DiscoveryDistance: 1},
	}
	sortComponentInfos(infos)
	if infos[0].Role != "target" {
		t.Errorf("first item role = %q, want 'target'", infos[0].Role)
	}
}

func TestSortComponentInfos_ByDistanceThenName(t *testing.T) {
	infos := []ComponentInfo{
		{Name: "z-svc", Role: "dependency", DiscoveryDistance: 2},
		{Name: "a-svc", Role: "dependency", DiscoveryDistance: 2},
		{Name: "m-svc", Role: "dependency", DiscoveryDistance: 1},
	}
	sortComponentInfos(infos)

	// Depth 1 before depth 2.
	if infos[0].DiscoveryDistance != 1 {
		t.Errorf("first non-target item distance = %d, want 1", infos[0].DiscoveryDistance)
	}
	// Within same depth, alphabetical.
	if infos[1].Name >= infos[2].Name {
		t.Errorf("expected alphabetical order within same depth; got %q then %q", infos[1].Name, infos[2].Name)
	}
}
