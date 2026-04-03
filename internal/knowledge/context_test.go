package knowledge

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── AssembleContextBlock tests ───────────────────────────────────────────────

func TestAssembleContextBlock_Basic(t *testing.T) {
	sections := []ContextSection{
		{File: "auth.md", HeadingPath: "JWT Validation", Content: "JWT tokens are validated here.", Score: 0.9},
		{File: "user.md", HeadingPath: "User Model", Content: "User struct definition.", Score: 0.7},
	}
	out := AssembleContextBlock("how does auth work?", sections)

	if !strings.HasPrefix(out, "# Context for: how does auth work?") {
		t.Errorf("output should start with '# Context for:' header, got: %q", out[:min(60, len(out))])
	}
	if !strings.Contains(out, "## [auth.md § JWT Validation]") {
		t.Errorf("output missing citation for auth.md, got:\n%s", out)
	}
	if !strings.Contains(out, "## [user.md § User Model]") {
		t.Errorf("output missing citation for user.md, got:\n%s", out)
	}
	if !strings.Contains(out, "JWT tokens are validated here.") {
		t.Errorf("output missing auth content, got:\n%s", out)
	}
	if !strings.Contains(out, "User struct definition.") {
		t.Errorf("output missing user content, got:\n%s", out)
	}
}

func TestAssembleContextBlock_EmptySections(t *testing.T) {
	out := AssembleContextBlock("query", nil)
	if out != "" {
		t.Errorf("empty sections should return empty string, got %q", out)
	}

	out2 := AssembleContextBlock("query", []ContextSection{})
	if out2 != "" {
		t.Errorf("empty sections slice should return empty string, got %q", out2)
	}
}

func TestAssembleContextBlock_PreambleSection(t *testing.T) {
	sections := []ContextSection{
		{File: "overview.md", HeadingPath: "", Content: "Overview preamble text.", Score: 0.8},
	}
	out := AssembleContextBlock("overview", sections)

	// Preamble (empty HeadingPath) should use "## [file.md]" without § separator.
	if !strings.Contains(out, "## [overview.md]") {
		t.Errorf("preamble section should use '## [file.md]' format (no §), got:\n%s", out)
	}
	if strings.Contains(out, "§") {
		t.Errorf("preamble section should NOT contain § separator, got:\n%s", out)
	}
}

func TestAssembleContextBlock_MultipleFiles(t *testing.T) {
	sections := []ContextSection{
		{File: "docs/api.md", HeadingPath: "Authentication", Content: "API auth section.", Score: 0.95},
		{File: "docs/db.md", HeadingPath: "Connection Pooling", Content: "DB pool config.", Score: 0.82},
	}
	out := AssembleContextBlock("connection", sections)

	if !strings.Contains(out, "## [docs/api.md § Authentication]") {
		t.Errorf("output missing citation for docs/api.md, got:\n%s", out)
	}
	if !strings.Contains(out, "## [docs/db.md § Connection Pooling]") {
		t.Errorf("output missing citation for docs/db.md, got:\n%s", out)
	}
}

// ─── sectionsFromBM25Results tests ───────────────────────────────────────────

func TestSectionsFromBM25Results(t *testing.T) {
	results := []SearchResult{
		{
			RelPath:        "auth.md",
			HeadingPath:    "JWT",
			ContentPreview: "JWT validation logic.",
			Score:          0.9,
		},
		{
			RelPath:        "user.md",
			HeadingPath:    "UserModel",
			ContentPreview: "User struct.",
			Score:          0.7,
		},
		{
			RelPath:     "readme.md",
			HeadingPath: "",
			Snippet:     "Fallback snippet text.",
			Score:       0.5,
			// ContentPreview is intentionally empty to test fallback to Snippet.
		},
	}

	sections := sectionsFromBM25Results(results)

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	// First section: File, HeadingPath, Content from ContentPreview, Score.
	if sections[0].File != "auth.md" {
		t.Errorf("section[0].File: got %q, want auth.md", sections[0].File)
	}
	if sections[0].HeadingPath != "JWT" {
		t.Errorf("section[0].HeadingPath: got %q, want JWT", sections[0].HeadingPath)
	}
	if sections[0].Content != "JWT validation logic." {
		t.Errorf("section[0].Content: got %q, want 'JWT validation logic.'", sections[0].Content)
	}
	if sections[0].Score != 0.9 {
		t.Errorf("section[0].Score: got %f, want 0.9", sections[0].Score)
	}

	// Second section.
	if sections[1].File != "user.md" {
		t.Errorf("section[1].File: got %q, want user.md", sections[1].File)
	}

	// Third section: ContentPreview empty → fallback to Snippet.
	if sections[2].Content != "Fallback snippet text." {
		t.Errorf("section[2].Content should fall back to Snippet when ContentPreview empty, got %q", sections[2].Content)
	}
}

// ─── ParseContextArgs tests ───────────────────────────────────────────────────

func TestParseContextArgs_Basic(t *testing.T) {
	a, err := ParseContextArgs([]string{"how does auth work?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Query != "how does auth work?" {
		t.Errorf("Query: got %q, want %q", a.Query, "how does auth work?")
	}
}

func TestParseContextArgs_Flags(t *testing.T) {
	a, err := ParseContextArgs([]string{"query", "--top", "3", "--format", "json", "--model", "claude-opus-4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Top != 3 {
		t.Errorf("Top: got %d, want 3", a.Top)
	}
	if a.Format != "json" {
		t.Errorf("Format: got %q, want json", a.Format)
	}
	if a.Model != "claude-opus-4" {
		t.Errorf("Model: got %q, want claude-opus-4", a.Model)
	}
}

func TestParseContextArgs_NoQuery(t *testing.T) {
	_, err := ParseContextArgs([]string{})
	if err == nil {
		t.Fatal("expected error for missing QUERY argument")
	}
	if !strings.Contains(err.Error(), "QUERY argument required") {
		t.Errorf("error should mention 'QUERY argument required', got: %v", err)
	}
}

func TestParseContextArgs_Defaults(t *testing.T) {
	a, err := ParseContextArgs([]string{"test query"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Top != 5 {
		t.Errorf("Top default: got %d, want 5", a.Top)
	}
	if a.Format != "markdown" {
		t.Errorf("Format default: got %q, want markdown", a.Format)
	}
	if a.Model != "claude-sonnet-4-5" {
		t.Errorf("Model default: got %q, want claude-sonnet-4-5", a.Model)
	}
	if a.Dir != "." {
		t.Errorf("Dir default: got %q, want .", a.Dir)
	}
}

// ─── CmdContext integration tests ─────────────────────────────────────────────

// captureStdout captures os.Stdout during fn execution and returns the output.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old

	return buf.String()
}

// setupContextTestDocs creates a temp dir with two markdown files containing
// auth-related content suitable for context retrieval tests.
func setupContextTestDocs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"auth.md": `# Auth Service

## JWT Validation

The Auth Service validates JWT tokens using HMAC-SHA256.
Each token contains the user ID and expiration timestamp.

## Token Issuance

Tokens are issued after successful credential verification.
`,
		"user.md": `# User Service

## Authentication Flow

The User Service delegates authentication to the Auth Service.
Users must provide valid credentials to receive a token.

## User Model

Each user has an ID, email, and hashed password.
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	return dir
}

func TestCmdContext_BM25Fallback_MarkdownOutput(t *testing.T) {
	dir := setupContextTestDocs(t)

	out := captureStdout(func() {
		err := CmdContext([]string{"auth", "--dir", dir, "--format", "markdown"})
		if err != nil {
			t.Errorf("CmdContext error: %v", err)
		}
	})

	if !strings.HasPrefix(out, "# Context for: auth") {
		t.Errorf("output should start with '# Context for: auth', got: %q", out[:min(80, len(out))])
	}
	if !strings.Contains(out, "## [") {
		t.Errorf("output should contain at least one citation '## [', got:\n%s", out)
	}
}

func TestCmdContext_BM25Fallback_JSONOutput(t *testing.T) {
	dir := setupContextTestDocs(t)

	out := captureStdout(func() {
		err := CmdContext([]string{"auth", "--dir", dir, "--format", "json"})
		if err != nil {
			t.Errorf("CmdContext error: %v", err)
		}
	})

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &envelope); err != nil {
		t.Fatalf("output not valid JSON: %v\nOutput: %s", err, out)
	}

	status, ok := envelope["status"].(string)
	if !ok {
		t.Fatalf("JSON missing 'status' field, got: %v", envelope)
	}
	if status != "ok" && status != "empty" {
		t.Errorf("status should be 'ok' or 'empty', got %q", status)
	}

	if status == "ok" {
		if _, ok := envelope["data"]; !ok {
			t.Error("JSON 'ok' response missing 'data' field")
		}
	}
}

func TestCmdContext_NoResults_EmptyDir(t *testing.T) {
	// Empty dir — no .md files, no index.
	dir := t.TempDir()

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = CmdContext([]string{"unrelated query xyz abc", "--dir", dir})
	})

	if cmdErr != nil {
		t.Fatalf("CmdContext should return nil error for no-results case, got: %v", cmdErr)
	}

	if !strings.Contains(out, "No relevant context found") {
		t.Errorf("output should contain 'No relevant context found', got: %q", out)
	}
}
