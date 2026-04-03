package knowledge

import (
	"testing"
)

// ─── NewComponentGraph tests ───────────────────────────────────────────────────

func TestNewComponentGraph_InitialisesNodes(t *testing.T) {
	comps := []Component{
		{ID: "api", Name: "API", File: "services/api/README.md"},
		{ID: "db", Name: "Database", File: "services/db/README.md"},
	}
	cg := NewComponentGraph(comps)

	if cg.NodeCount() != 2 {
		t.Fatalf("NodeCount = %d, want 2", cg.NodeCount())
	}
	if _, ok := cg.Nodes["api"]; !ok {
		t.Error("expected node 'api' in graph")
	}
	if _, ok := cg.Nodes["db"]; !ok {
		t.Error("expected node 'db' in graph")
	}
}

func TestNewComponentGraph_InitialisesComponents(t *testing.T) {
	comps := []Component{
		{ID: "svc-a", Name: "Service A", File: "svc-a/README.md"},
	}
	cg := NewComponentGraph(comps)

	if _, ok := cg.Components["svc-a"]; !ok {
		t.Error("expected component 'svc-a' in Components map")
	}
}

func TestNewComponentGraph_NodePathFromFile(t *testing.T) {
	comps := []Component{
		{ID: "auth", Name: "Auth", File: "services/auth/README.md"},
	}
	cg := NewComponentGraph(comps)

	node := cg.Nodes["auth"]
	if node.Path != "services/auth" {
		t.Errorf("node.Path = %q, want 'services/auth'", node.Path)
	}
}

func TestNewComponentGraph_EmptyComponents(t *testing.T) {
	cg := NewComponentGraph(nil)

	if cg.NodeCount() != 0 {
		t.Errorf("NodeCount = %d, want 0 for nil input", cg.NodeCount())
	}
}

// ─── AddEdge tests ─────────────────────────────────────────────────────────────

func TestAddEdge_AddsNewEdge(t *testing.T) {
	comps := []Component{
		{ID: "payment", Name: "Payment", File: "payment.md"},
		{ID: "auth", Name: "Auth", File: "auth.md"},
	}
	cg := NewComponentGraph(comps)

	err := cg.AddEdge("payment", "auth", 0.9, "depends_on", []string{"calls auth"})
	if err != nil {
		t.Fatalf("AddEdge: unexpected error: %v", err)
	}
	if cg.EdgeCount() != 1 {
		t.Errorf("EdgeCount = %d, want 1", cg.EdgeCount())
	}
}

func TestAddEdge_SelfLoopReturnsError(t *testing.T) {
	comps := []Component{
		{ID: "svc", Name: "Svc", File: "svc.md"},
	}
	cg := NewComponentGraph(comps)

	err := cg.AddEdge("svc", "svc", 0.9, "depends_on", nil)
	if err == nil {
		t.Error("expected error for self-loop, got nil")
	}
}

func TestAddEdge_EmptyFromReturnsError(t *testing.T) {
	cg := NewComponentGraph(nil)
	err := cg.AddEdge("", "target", 0.9, "depends_on", nil)
	if err == nil {
		t.Error("expected error for empty from, got nil")
	}
}

func TestAddEdge_EmptyToReturnsError(t *testing.T) {
	cg := NewComponentGraph(nil)
	err := cg.AddEdge("source", "", 0.9, "depends_on", nil)
	if err == nil {
		t.Error("expected error for empty to, got nil")
	}
}

func TestAddEdge_KeepsHigherConfidenceEdge(t *testing.T) {
	comps := []Component{
		{ID: "a", Name: "A", File: "a.md"},
		{ID: "b", Name: "B", File: "b.md"},
	}
	cg := NewComponentGraph(comps)

	_ = cg.AddEdge("a", "b", 0.9, "depends_on", []string{"high confidence"})
	_ = cg.AddEdge("a", "b", 0.5, "depends_on", []string{"low confidence"}) // should be ignored

	edge := cg.Edges["a"]["b"]
	if edge.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9 (higher), got %v", edge.Confidence)
	}
}

func TestAddEdge_ReplacesLowerConfidenceEdge(t *testing.T) {
	comps := []Component{
		{ID: "a", Name: "A", File: "a.md"},
		{ID: "b", Name: "B", File: "b.md"},
	}
	cg := NewComponentGraph(comps)

	_ = cg.AddEdge("a", "b", 0.5, "related", []string{"low"})
	_ = cg.AddEdge("a", "b", 0.95, "depends_on", []string{"high"}) // should replace

	edge := cg.Edges["a"]["b"]
	if edge.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95 (higher replacement), got %v", edge.Confidence)
	}
}

func TestAddEdge_UpdatesDegrees(t *testing.T) {
	comps := []Component{
		{ID: "src", Name: "Src", File: "src.md"},
		{ID: "tgt", Name: "Tgt", File: "tgt.md"},
	}
	cg := NewComponentGraph(comps)

	_ = cg.AddEdge("src", "tgt", 0.8, "depends_on", nil)

	if cg.Nodes["src"].OutDegree != 1 {
		t.Errorf("src OutDegree = %d, want 1", cg.Nodes["src"].OutDegree)
	}
	if cg.Nodes["tgt"].InDegree != 1 {
		t.Errorf("tgt InDegree = %d, want 1", cg.Nodes["tgt"].InDegree)
	}
}

// ─── GetOutgoing / GetIncoming tests ──────────────────────────────────────────

func TestGetOutgoing_ReturnsEdges(t *testing.T) {
	comps := []Component{
		{ID: "payment", Name: "Payment", File: "payment.md"},
		{ID: "auth", Name: "Auth", File: "auth.md"},
		{ID: "user", Name: "User", File: "user.md"},
	}
	cg := NewComponentGraph(comps)
	_ = cg.AddEdge("payment", "auth", 0.9, "depends_on", nil)
	_ = cg.AddEdge("payment", "user", 0.8, "depends_on", nil)

	edges := cg.GetOutgoing("payment")
	if len(edges) != 2 {
		t.Errorf("GetOutgoing('payment') = %d edges, want 2", len(edges))
	}
}

func TestGetOutgoing_EmptyForNodeWithNoEdges(t *testing.T) {
	comps := []Component{
		{ID: "isolated", Name: "Isolated", File: "isolated.md"},
	}
	cg := NewComponentGraph(comps)

	edges := cg.GetOutgoing("isolated")
	if len(edges) != 0 {
		t.Errorf("GetOutgoing('isolated') = %d, want 0", len(edges))
	}
}

func TestGetOutgoing_NilForUnknownComponent(t *testing.T) {
	cg := NewComponentGraph(nil)
	edges := cg.GetOutgoing("nonexistent")
	if edges != nil {
		t.Errorf("GetOutgoing for unknown component should return nil, got %v", edges)
	}
}

func TestGetIncoming_ReturnsEdges(t *testing.T) {
	comps := []Component{
		{ID: "payment", Name: "Payment", File: "payment.md"},
		{ID: "auth", Name: "Auth", File: "auth.md"},
		{ID: "order", Name: "Order", File: "order.md"},
	}
	cg := NewComponentGraph(comps)
	_ = cg.AddEdge("payment", "auth", 0.9, "depends_on", nil)
	_ = cg.AddEdge("order", "auth", 0.7, "depends_on", nil)

	edges := cg.GetIncoming("auth")
	if len(edges) != 2 {
		t.Errorf("GetIncoming('auth') = %d edges, want 2", len(edges))
	}
}

func TestGetIncoming_EmptyForNodeWithNoIncoming(t *testing.T) {
	comps := []Component{
		{ID: "root", Name: "Root", File: "root.md"},
		{ID: "leaf", Name: "Leaf", File: "leaf.md"},
	}
	cg := NewComponentGraph(comps)
	_ = cg.AddEdge("root", "leaf", 0.8, "depends_on", nil)

	incoming := cg.GetIncoming("root") // root has no incoming edges
	if len(incoming) != 0 {
		t.Errorf("GetIncoming('root') = %d, want 0", len(incoming))
	}
}

// ─── MapFilesToComponents tests ────────────────────────────────────────────────

func TestMapFilesToComponents_MapsOwnedFiles(t *testing.T) {
	comps := []Component{
		{ID: "auth", Name: "Auth", File: "services/auth/README.md"},
	}
	cg := NewComponentGraph(comps)
	cg.MapFilesToComponents([]string{"services/auth/README.md"})

	if comp, ok := cg.FileToComponent["services/auth/README.md"]; !ok || comp != "auth" {
		t.Errorf("FileToComponent['services/auth/README.md'] = %q, want 'auth'", comp)
	}
}

func TestMapFilesToComponents_AssignsByDirectoryPrefix(t *testing.T) {
	comps := []Component{
		{ID: "auth", Name: "Auth", File: "services/auth/README.md"},
	}
	cg := NewComponentGraph(comps)

	allFiles := []string{
		"services/auth/README.md",
		"services/auth/api.md",      // additional file in same directory
		"services/auth/docs/ref.md", // subdirectory
	}
	cg.MapFilesToComponents(allFiles)

	if comp := cg.FileToComponent["services/auth/api.md"]; comp != "auth" {
		t.Errorf("FileToComponent['services/auth/api.md'] = %q, want 'auth'", comp)
	}
}

// ─── NodeCount / EdgeCount tests ──────────────────────────────────────────────

func TestNodeCount_Correct(t *testing.T) {
	comps := make([]Component, 5)
	for i := range comps {
		comps[i] = Component{ID: string(rune('a' + i)), File: "x.md"}
	}
	cg := NewComponentGraph(comps)
	if cg.NodeCount() != 5 {
		t.Errorf("NodeCount = %d, want 5", cg.NodeCount())
	}
}

func TestEdgeCount_Correct(t *testing.T) {
	comps := []Component{
		{ID: "a", File: "a.md"},
		{ID: "b", File: "b.md"},
		{ID: "c", File: "c.md"},
	}
	cg := NewComponentGraph(comps)
	_ = cg.AddEdge("a", "b", 0.9, "depends_on", nil)
	_ = cg.AddEdge("b", "c", 0.8, "depends_on", nil)
	_ = cg.AddEdge("a", "c", 0.7, "depends_on", nil)

	if cg.EdgeCount() != 3 {
		t.Errorf("EdgeCount = %d, want 3", cg.EdgeCount())
	}
}

// ─── BuildComponentGraph tests ────────────────────────────────────────────────

func TestBuildComponentGraph_NoComponentsError(t *testing.T) {
	fileGraph := NewGraph()
	_, err := BuildComponentGraph(nil, fileGraph, nil)
	if err == nil {
		t.Fatal("expected error for nil components, got nil")
	}
}

func TestBuildComponentGraph_BuiltFromComponents(t *testing.T) {
	comps := []Component{
		{ID: "api", Name: "API", File: "api.md", Confidence: ConfidenceConfigured},
	}
	fileGraph := NewGraph()
	// Add a node to file graph so MapFilesToComponents has something to work with.
	fileGraph.Nodes["api.md"] = &Node{ID: "api.md", Title: "API"}

	cg, err := BuildComponentGraph(comps, fileGraph, nil)
	if err != nil {
		t.Fatalf("BuildComponentGraph: unexpected error: %v", err)
	}
	if cg.NodeCount() != 1 {
		t.Errorf("NodeCount = %d, want 1", cg.NodeCount())
	}
}

func TestBuildComponentGraph_ExtractsEdgesFromFileGraph(t *testing.T) {
	comps := []Component{
		{ID: "svc-a", Name: "Svc A", File: "svc-a/README.md", Confidence: ConfidenceConfigured},
		{ID: "svc-b", Name: "Svc B", File: "svc-b/README.md", Confidence: ConfidenceConfigured},
	}
	fileGraph := NewGraph()
	fileGraph.Nodes["svc-a/README.md"] = &Node{ID: "svc-a/README.md"}
	fileGraph.Nodes["svc-b/README.md"] = &Node{ID: "svc-b/README.md"}
	_ = fileGraph.AddEdge(&Edge{
		ID:         "svc-a/README.md->svc-b/README.md",
		Source:     "svc-a/README.md",
		Target:     "svc-b/README.md",
		Type:       EdgeDependsOn,
		Confidence: 0.85,
		Evidence:   "svc-a calls svc-b",
	})

	cg, err := BuildComponentGraph(comps, fileGraph, nil)
	if err != nil {
		t.Fatalf("BuildComponentGraph: %v", err)
	}
	if cg.EdgeCount() != 1 {
		t.Errorf("EdgeCount = %d, want 1", cg.EdgeCount())
	}
}

// ─── directoryOf / appendIfMissing helpers ────────────────────────────────────

func TestDirectoryOf_ReturnsParentDir(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"services/auth/README.md", "services/auth"},
		{"README.md", ""},
		{"a/b/c/file.md", "a/b/c"},
	}
	for _, tc := range cases {
		got := directoryOf(tc.input)
		if got != tc.want {
			t.Errorf("directoryOf(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestAppendIfMissing_NoDuplicates(t *testing.T) {
	s := []string{"a", "b"}
	s = appendIfMissing(s, "b") // already present
	if len(s) != 2 {
		t.Errorf("expected len=2, got %d", len(s))
	}
	s = appendIfMissing(s, "c") // new
	if len(s) != 3 {
		t.Errorf("expected len=3, got %d", len(s))
	}
}
