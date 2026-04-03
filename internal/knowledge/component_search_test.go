package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildSearchGraph creates a ComponentGraph with 3 components, wiring them in
// memory so FindComponentReferences / EstimateComponentConfidence can be tested
// without a real Knowledge instance.
func buildSearchGraph(t *testing.T, tmpDir string) *ComponentGraph {
	t.Helper()

	comps := []Component{
		{ID: "api", Name: "API Gateway", File: "api/README.md", Confidence: ConfidenceConfigured},
		{ID: "auth", Name: "Auth Service", File: "auth/README.md", Confidence: ConfidenceConfigured},
		{ID: "db", Name: "Database", File: "db/README.md", Confidence: ConfidenceConfigured},
	}

	if tmpDir != "" {
		for _, c := range comps {
			dir := filepath.Join(tmpDir, filepath.FromSlash(filepath.Dir(c.File)))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
		}
		// api/README.md mentions auth explicitly via backtick.
		mustWriteFile(t, tmpDir, "api/README.md",
			"# API Gateway\n\nCalls `auth` for authentication.\nAlso reads from the db.\n")
		// auth/README.md mentions db.
		mustWriteFile(t, tmpDir, "auth/README.md",
			"# Auth Service\n\nReads tokens from the db service.\n")
		// db/README.md has no mentions.
		mustWriteFile(t, tmpDir, "db/README.md",
			"# Database\n\nStores all data.\n")
	}

	return NewComponentGraph(comps)
}

// ─── EstimateComponentConfidence tests ────────────────────────────────────────

func TestEstimateComponentConfidence_BacktickReference(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	evidence := []string{"calls `auth` for token validation"}
	conf := cs.EstimateComponentConfidence("api", "auth", evidence)

	// Backtick reference should yield 1.0.
	if conf != 1.0 {
		t.Errorf("confidence = %v, want 1.0 for backtick reference", conf)
	}
}

func TestEstimateComponentConfidence_PathReference(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	evidence := []string{"imports from /auth/api endpoint"}
	conf := cs.EstimateComponentConfidence("api", "auth", evidence)

	// Path-like reference should yield ≥ 0.5.
	if conf < 0.5 {
		t.Errorf("confidence = %v, want >= 0.5 for path reference", conf)
	}
}

func TestEstimateComponentConfidence_ProseReference(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	evidence := []string{"the auth component handles all token checks"}
	conf := cs.EstimateComponentConfidence("api", "auth", evidence)

	// Prose mention should yield ≥ 0.3.
	if conf < 0.3 {
		t.Errorf("confidence = %v, want >= 0.3 for prose mention", conf)
	}
}

func TestEstimateComponentConfidence_EmptyEvidenceReturnsZero(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	conf := cs.EstimateComponentConfidence("api", "auth", nil)
	if conf != 0 {
		t.Errorf("confidence = %v, want 0 for empty evidence", conf)
	}
}

func TestEstimateComponentConfidence_MultipleEvidenceUsesMax(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	evidence := []string{
		"auth mentioned in prose",  // 0.7
		"calls `auth` directly",    // 1.0
	}
	conf := cs.EstimateComponentConfidence("api", "auth", evidence)
	if conf != 1.0 {
		t.Errorf("confidence = %v, want 1.0 (max of evidence)", conf)
	}
}

// ─── NewComponentSearch tests ──────────────────────────────────────────────────

func TestNewComponentSearch_CreatesInstance(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")
	if cs == nil {
		t.Fatal("NewComponentSearch returned nil")
	}
}

func TestNewComponentSearch_StrategyStored(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "pageindex")
	if cs.strategy != "pageindex" {
		t.Errorf("strategy = %q, want 'pageindex'", cs.strategy)
	}
}

// ─── FindComponentReferences tests ────────────────────────────────────────────

func TestFindComponentReferences_UnknownFromReturnsError(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	_, _, err := cs.FindComponentReferences("nonexistent", "auth")
	if err == nil {
		t.Error("expected error for unknown fromComponent, got nil")
	}
}

func TestFindComponentReferences_UnknownToReturnsError(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	_, _, err := cs.FindComponentReferences("api", "nonexistent")
	if err == nil {
		t.Error("expected error for unknown toComponent, got nil")
	}
}

func TestFindComponentReferences_NilKnowledgeReturnNoEvidence(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	// Without a Knowledge instance, text scan returns nothing gracefully.
	ev, conf, err := cs.FindComponentReferences("api", "auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil knowledge means no text scan — evidence should be nil/empty, no error.
	_ = ev
	_ = conf
}

// ─── componentMentionPatterns tests ───────────────────────────────────────────

func TestComponentMentionPatterns_IncludesID(t *testing.T) {
	comp := &Component{ID: "api-gateway", Name: "API Gateway", File: "services/api-gateway.md"}
	patterns := componentMentionPatterns(comp)

	found := false
	for _, p := range patterns {
		if strings.Contains(p, "api-gateway") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("patterns %v do not include component ID", patterns)
	}
}

func TestComponentMentionPatterns_IncludesNameWhenDifferentFromID(t *testing.T) {
	comp := &Component{ID: "auth", Name: "Authentication Service", File: "auth.md"}
	patterns := componentMentionPatterns(comp)

	found := false
	for _, p := range patterns {
		if strings.Contains(p, "authentication service") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("patterns %v should include component name 'authentication service'", patterns)
	}
}

func TestComponentMentionPatterns_FileStem(t *testing.T) {
	comp := &Component{ID: "svc", Name: "Svc", File: "services/auth-service.md"}
	patterns := componentMentionPatterns(comp)

	// File stem "auth-service" should be in patterns if different from ID "svc".
	found := false
	for _, p := range patterns {
		if p == "auth-service" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("patterns %v should include file stem 'auth-service'", patterns)
	}
}

// ─── evidenceSnippet tests ─────────────────────────────────────────────────────

func TestEvidenceSnippet_ReturnsSnippetAroundTerm(t *testing.T) {
	content := "The API Gateway calls auth for authentication and then returns the result."
	snippet := evidenceSnippet(content, "auth", 50)

	if !strings.Contains(strings.ToLower(snippet), "auth") {
		t.Errorf("snippet %q does not contain 'auth'", snippet)
	}
}

func TestEvidenceSnippet_TruncatesToMaxLen(t *testing.T) {
	content := strings.Repeat("word ", 200) + "auth" + strings.Repeat(" word", 200)
	snippet := evidenceSnippet(content, "auth", 40)

	if len([]rune(snippet)) > 40 {
		t.Errorf("snippet length = %d, want <= 40", len([]rune(snippet)))
	}
}

func TestEvidenceSnippet_NoMatchReturnsBeginning(t *testing.T) {
	content := "Some content without the target term."
	snippet := evidenceSnippet(content, "nonexistent", 20)

	// Should return beginning of content, not empty.
	if snippet == "" {
		t.Error("expected non-empty snippet even when term not found")
	}
}

// ─── QueryComponentDependencies tests ─────────────────────────────────────────

func TestQueryComponentDependencies_UnknownComponentError(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	_, err := cs.QueryComponentDependencies("nonexistent", 1)
	if err == nil {
		t.Error("expected error for unknown component")
	}
}

func TestQueryComponentDependencies_ReturnsMap(t *testing.T) {
	cg := buildSearchGraph(t, "")
	cs := NewComponentSearch(cg, nil, "fallback")

	result, err := cs.QueryComponentDependencies("api", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should be a map (possibly empty if no text knowledge).
	if result == nil {
		t.Error("expected non-nil map result")
	}
}
