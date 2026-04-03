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
