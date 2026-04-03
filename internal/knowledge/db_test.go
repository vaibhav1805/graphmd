package knowledge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveGraph_DroppedEdgeWarning verifies that SaveGraph prints a warning to
// stderr when edges reference non-existent nodes, while still saving valid edges.
func TestSaveGraph_DroppedEdgeWarning(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a.md", Type: "document", Title: "A"})
	_ = g.AddNode(&Node{ID: "b.md", Type: "document", Title: "B"})

	// Valid edge A -> B
	_ = g.AddEdge(&Edge{ID: "a-b", Source: "a.md", Target: "b.md", Type: EdgeReferences, Confidence: 0.9})
	// Dangling edge A -> C (C doesn't exist)
	_ = g.AddEdge(&Edge{ID: "a-c", Source: "a.md", Target: "c.md", Type: EdgeReferences, Confidence: 0.8})

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Capture stderr output
	var buf bytes.Buffer
	stderrWriter = &buf
	defer func() { stderrWriter = os.Stderr }()

	err = db.SaveGraph(g)
	if err != nil {
		t.Fatalf("SaveGraph returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "dropped") {
		t.Errorf("expected stderr to contain 'dropped', got: %q", output)
	}
	if !strings.Contains(output, "1") {
		t.Errorf("expected stderr to contain count '1', got: %q", output)
	}
	if !strings.Contains(output, "c.md") {
		t.Errorf("expected stderr to mention target 'c.md', got: %q", output)
	}

	// Verify valid edge was still saved
	g2 := NewGraph()
	if err := db.LoadGraph(g2); err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if _, ok := g2.Edges["a-b"]; !ok {
		t.Error("valid edge a-b should have been saved")
	}
	if _, ok := g2.Edges["a-c"]; ok {
		t.Error("dangling edge a-c should NOT have been saved")
	}
}

// TestSaveGraph_NoDroppedEdges verifies no warning when all edges are valid.
func TestSaveGraph_NoDroppedEdges(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a.md", Type: "document", Title: "A"})
	_ = g.AddNode(&Node{ID: "b.md", Type: "document", Title: "B"})
	_ = g.AddEdge(&Edge{ID: "a-b", Source: "a.md", Target: "b.md", Type: EdgeReferences, Confidence: 0.9})

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	var buf bytes.Buffer
	stderrWriter = &buf
	defer func() { stderrWriter = os.Stderr }()

	err = db.SaveGraph(g)
	if err != nil {
		t.Fatalf("SaveGraph returned error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "dropped") {
		t.Errorf("expected no 'dropped' warning for clean graph, got: %q", output)
	}
}

// TestSaveGraph_DroppedEdgeMissingSource verifies warning when edge source doesn't exist.
func TestSaveGraph_DroppedEdgeMissingSource(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "b.md", Type: "document", Title: "B"})

	// Edge from non-existent source X -> B
	_ = g.AddEdge(&Edge{ID: "x-b", Source: "x.md", Target: "b.md", Type: EdgeReferences, Confidence: 0.8})

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	var buf bytes.Buffer
	stderrWriter = &buf
	defer func() { stderrWriter = os.Stderr }()

	err = db.SaveGraph(g)
	if err != nil {
		t.Fatalf("SaveGraph returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "missing source") {
		t.Errorf("expected stderr to mention 'missing source', got: %q", output)
	}
}

// TestSaveGraph_DroppedEdgeMissingTarget verifies warning when edge target doesn't exist.
func TestSaveGraph_DroppedEdgeMissingTarget(t *testing.T) {
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "a.md", Type: "document", Title: "A"})

	// Edge from A -> non-existent target Y
	_ = g.AddEdge(&Edge{ID: "a-y", Source: "a.md", Target: "y.md", Type: EdgeReferences, Confidence: 0.8})

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	var buf bytes.Buffer
	stderrWriter = &buf
	defer func() { stderrWriter = os.Stderr }()

	err = db.SaveGraph(g)
	if err != nil {
		t.Fatalf("SaveGraph returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "missing target") {
		t.Errorf("expected stderr to mention 'missing target', got: %q", output)
	}
}

// --- LoadComponentMentions tests ---------------------------------------------

func TestLoadComponentMentions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Save a graph with components first (for FK references).
	g := NewGraph()
	_ = g.AddNode(&Node{ID: "svc-a", Type: "document", Title: "Service A", ComponentType: ComponentTypeService})
	_ = g.AddNode(&Node{ID: "svc-b", Type: "document", Title: "Service B", ComponentType: ComponentTypeService})
	if err := db.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph: %v", err)
	}

	// Insert mentions for both components.
	mentions := []ComponentMention{
		{ComponentID: "svc-a", FilePath: "docs/arch.md", HeadingHierarchy: "Architecture > Services", DetectedBy: "explicit-link", Confidence: 0.95},
		{ComponentID: "svc-a", FilePath: "docs/deploy.md", HeadingHierarchy: "Deployment", DetectedBy: "co-occurrence", Confidence: 0.70},
		{ComponentID: "svc-a", FilePath: "docs/overview.md", HeadingHierarchy: "Overview", DetectedBy: "structural", Confidence: 0.85},
		{ComponentID: "svc-b", FilePath: "docs/arch.md", HeadingHierarchy: "Architecture > Databases", DetectedBy: "explicit-link", Confidence: 0.90},
		{ComponentID: "svc-b", FilePath: "docs/config.md", HeadingHierarchy: "Configuration", DetectedBy: "semantic", Confidence: 0.60},
	}
	if err := db.SaveComponentMentions(mentions); err != nil {
		t.Fatalf("SaveComponentMentions: %v", err)
	}

	// Load mentions via the new method.
	result, err := db.LoadComponentMentions()
	if err != nil {
		t.Fatalf("LoadComponentMentions: %v", err)
	}

	// Verify both components are present.
	if len(result) != 2 {
		t.Fatalf("expected 2 components in result, got %d", len(result))
	}

	// Verify svc-a has 3 mentions ordered by confidence DESC.
	svcA := result["svc-a"]
	if len(svcA) != 3 {
		t.Fatalf("expected 3 mentions for svc-a, got %d", len(svcA))
	}
	if svcA[0].Confidence != 0.95 {
		t.Errorf("expected first mention confidence 0.95, got %f", svcA[0].Confidence)
	}
	if svcA[1].Confidence != 0.85 {
		t.Errorf("expected second mention confidence 0.85, got %f", svcA[1].Confidence)
	}
	if svcA[2].Confidence != 0.70 {
		t.Errorf("expected third mention confidence 0.70, got %f", svcA[2].Confidence)
	}

	// Verify svc-b has 2 mentions ordered by confidence DESC.
	svcB := result["svc-b"]
	if len(svcB) != 2 {
		t.Fatalf("expected 2 mentions for svc-b, got %d", len(svcB))
	}
	if svcB[0].Confidence != 0.90 {
		t.Errorf("expected first mention confidence 0.90, got %f", svcB[0].Confidence)
	}

	// Verify fields are populated.
	if svcA[0].FilePath != "docs/arch.md" {
		t.Errorf("expected file_path 'docs/arch.md', got %q", svcA[0].FilePath)
	}
	if svcA[0].DetectedBy != "explicit-link" {
		t.Errorf("expected detected_by 'explicit-link', got %q", svcA[0].DetectedBy)
	}
	if svcA[0].HeadingHierarchy != "Architecture > Services" {
		t.Errorf("expected heading_hierarchy 'Architecture > Services', got %q", svcA[0].HeadingHierarchy)
	}
}

func TestLoadComponentMentions_EmptyTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	result, err := db.LoadComponentMentions()
	if err != nil {
		t.Fatalf("LoadComponentMentions: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil empty map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}
