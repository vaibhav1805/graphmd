package knowledge

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// buildTestMonorepo creates a temporary monorepo directory layout and returns
// its root path.  The caller is responsible for cleanup via t.Cleanup or os.RemoveAll.
func buildTestMonorepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// services/payment — has package.json
	mustMkdir(t, root, "services/payment/handlers")
	mustWriteFile(t, root, "services/payment/package.json", `{"name":"payment"}`)
	mustWriteFile(t, root, "services/payment/README.md", "# Payment Service")

	// services/auth — has go.mod
	mustMkdir(t, root, "services/auth")
	mustWriteFile(t, root, "services/auth/go.mod", "module auth")
	mustWriteFile(t, root, "services/auth/README.md", "# Auth Service")

	// packages/common — has package.json
	mustMkdir(t, root, "packages/common")
	mustWriteFile(t, root, "packages/common/package.json", `{"name":"common"}`)
	mustWriteFile(t, root, "packages/common/README.md", "# Common Package")

	// packages/utils — has go.mod
	mustMkdir(t, root, "packages/utils")
	mustWriteFile(t, root, "packages/utils/go.mod", "module utils")
	mustWriteFile(t, root, "packages/utils/README.md", "# Utils Package")

	// apps/web — has package.json
	mustMkdir(t, root, "apps/web")
	mustWriteFile(t, root, "apps/web/package.json", `{"name":"web"}`)
	mustWriteFile(t, root, "apps/web/README.md", "# Web App")

	return root
}

func mustMkdir(t *testing.T, root string, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
}

func mustWriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func componentNames(components []DiscoveredComponent) []string {
	names := make([]string, len(components))
	for i, c := range components {
		names[i] = c.Name
	}
	sort.Strings(names)
	return names
}

// --- Test 1: Pure YAML discovery ---

func TestDiscoverComponents_Yaml(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, root, "svc/alpha")
	mustMkdir(t, root, "svc/beta")
	mustMkdir(t, root, ".bmd")
	mustWriteFile(t, root, ".bmd/components.yaml", `
components:
  - name: alpha
    path: svc/alpha
  - name: beta
    path: svc/beta
`)

	components, err := DiscoverComponents("", root, false)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}
	if len(components) != 2 {
		t.Fatalf("want 2 components, got %d: %v", len(components), components)
	}
	names := componentNames(components)
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("want [alpha, beta], got %v", names)
	}
	for _, c := range components {
		if c.DiscoveryMethod != "yaml" {
			t.Errorf("component %q: want method=yaml, got %q", c.Name, c.DiscoveryMethod)
		}
	}
}

// --- Test 2: Package marker fallback ---

func TestDiscoverComponents_PackageMarkers(t *testing.T) {
	root := buildTestMonorepo(t)

	components, err := DiscoverComponents("", root, false)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}
	names := componentNames(components)

	// Expect at least auth, common, payment, utils, web.
	expected := []string{"auth", "common", "payment", "utils", "web"}
	for _, want := range expected {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected component %q not found in %v", want, names)
		}
	}

	for _, c := range components {
		if c.DiscoveryMethod != "marker" {
			t.Errorf("component %q: want method=marker, got %q", c.Name, c.DiscoveryMethod)
		}
	}
}

func TestDiscoverComponents_PackageMarkers_MarkerField(t *testing.T) {
	root := buildTestMonorepo(t)

	components, err := DiscoverComponents("", root, false)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}
	for _, c := range components {
		if c.PackageMarker == "" {
			t.Errorf("component %q: expected non-empty PackageMarker", c.Name)
		}
	}
}

// --- Test 3: Conventional pattern fallback ---

func TestDiscoverComponents_ConventionalPatterns(t *testing.T) {
	root := t.TempDir()
	// Only directories, no package markers, conventional layout.
	mustMkdir(t, root, "services/payments")
	mustMkdir(t, root, "services/notifications")
	mustMkdir(t, root, "packages/ui")

	components, err := DiscoverComponents("", root, false)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}
	names := componentNames(components)

	expected := []string{"notifications", "payments", "ui"}
	for _, want := range expected {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected component %q not found in %v", want, names)
		}
	}
	for _, c := range components {
		if c.DiscoveryMethod != "conventional" {
			t.Errorf("component %q: want method=conventional, got %q", c.Name, c.DiscoveryMethod)
		}
	}
}

// --- Test 4: Depth-based fallback ---

func TestDiscoverComponents_DepthFallback(t *testing.T) {
	root := t.TempDir()
	// No markers, no conventional dirs, just top-level directories.
	mustMkdir(t, root, "frontend")
	mustMkdir(t, root, "backend")
	mustMkdir(t, root, "infra")

	cd := NewComponentDiscovery("", DefaultPackageMarkers, []string{}, false)
	components, err := cd.discover(root)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	names := componentNames(components)

	expected := []string{"backend", "frontend", "infra"}
	for _, want := range expected {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected component %q not found in %v", want, names)
		}
	}
	for _, c := range components {
		if c.DiscoveryMethod != "depth_fallback" {
			t.Errorf("component %q: want method=depth_fallback, got %q", c.Name, c.DiscoveryMethod)
		}
	}
}

// --- Test 5: Cascading preference (yaml preferred over markers) ---

func TestDiscoverComponents_CascadeYamlPreferred(t *testing.T) {
	root := t.TempDir()
	// Has both yaml config and package markers — yaml should win.
	mustMkdir(t, root, ".bmd")
	mustMkdir(t, root, "services/payment")
	mustWriteFile(t, root, "services/payment/package.json", `{"name":"payment"}`)
	mustWriteFile(t, root, ".bmd/components.yaml", `
components:
  - name: explicit-payment
    path: services/payment
`)

	components, err := DiscoverComponents("", root, false)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}

	if len(components) != 1 {
		t.Fatalf("want 1 component (yaml wins), got %d: %v", len(components), components)
	}
	if components[0].Name != "explicit-payment" {
		t.Errorf("want name=explicit-payment, got %q", components[0].Name)
	}
	if components[0].DiscoveryMethod != "yaml" {
		t.Errorf("want method=yaml, got %q", components[0].DiscoveryMethod)
	}
}

// --- Test 6: Deduplication (same component from multiple detections) ---

func TestDiscoverComponents_Deduplication(t *testing.T) {
	root := t.TempDir()
	// services/payment matches both marker AND conventional patterns.
	mustMkdir(t, root, "services/payment")
	mustWriteFile(t, root, "services/payment/package.json", `{"name":"payment"}`)

	cd := NewComponentDiscovery("", DefaultPackageMarkers, DefaultConventionalDirs, false)
	components, err := cd.discover(root)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	// Should find "payment" exactly once.
	count := 0
	for _, c := range components {
		if c.Name == "payment" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("want exactly 1 'payment' component, got %d (total: %v)", count, componentNames(components))
	}
}

// --- Test 7: IncludeHidden flag ---

func TestDiscoverComponents_IncludeHidden_False(t *testing.T) {
	root := t.TempDir()
	// .hidden directory should be excluded by default.
	mustMkdir(t, root, ".hidden")
	mustWriteFile(t, root, ".hidden/package.json", `{"name":"hidden"}`)
	mustMkdir(t, root, "visible")
	mustWriteFile(t, root, "visible/package.json", `{"name":"visible"}`)

	components, err := DiscoverComponents("", root, false)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}
	for _, c := range components {
		if c.Name == ".hidden" || c.Name == "hidden" {
			t.Errorf("hidden component should be excluded, got %q", c.Name)
		}
	}
}

func TestDiscoverComponents_IncludeHidden_True(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, root, ".hidden")
	mustWriteFile(t, root, ".hidden/package.json", `{"name":"hidden"}`)

	components, err := DiscoverComponents("", root, true)
	if err != nil {
		t.Fatalf("DiscoverComponents: %v", err)
	}
	found := false
	for _, c := range components {
		if c.Name == ".hidden" {
			found = true
		}
	}
	if !found {
		t.Errorf("hidden component should be included when includeHidden=true, got %v", componentNames(components))
	}
}

// --- Unit tests for individual helpers ---

func TestLoadComponentsYaml_NotExist(t *testing.T) {
	cd := NewComponentDiscovery("", nil, nil, false)
	components, err := cd.loadComponentsYaml("/nonexistent/path/components.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(components) != 0 {
		t.Errorf("expected empty result for missing file, got %v", components)
	}
}

func TestLoadComponentsYaml_Valid(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, root, ".bmd")
	mustWriteFile(t, root, ".bmd/components.yaml", `
components:
  - name: api
    path: services/api
  - name: worker
    path: services/worker
`)
	cd := NewComponentDiscovery("", nil, nil, false)
	components, err := cd.loadComponentsYaml(filepath.Join(root, ".bmd", "components.yaml"))
	if err != nil {
		t.Fatalf("loadComponentsYaml: %v", err)
	}
	if len(components) != 2 {
		t.Fatalf("want 2, got %d: %v", len(components), components)
	}
}

func TestDeduplicateComponents(t *testing.T) {
	input := []DiscoveredComponent{
		{Name: "foo", DiscoveryMethod: "yaml"},
		{Name: "bar", DiscoveryMethod: "marker"},
		{Name: "foo", DiscoveryMethod: "depth_fallback"}, // duplicate
	}
	result := deduplicateComponents(input)
	if len(result) != 2 {
		t.Fatalf("want 2, got %d: %v", len(result), result)
	}
	if result[0].Name != "foo" || result[0].DiscoveryMethod != "yaml" {
		t.Errorf("first 'foo' should be yaml method, got %+v", result[0])
	}
}

func TestDefaultHeuristics(t *testing.T) {
	// Verify that the constants are non-empty.
	if len(DefaultPackageMarkers) == 0 {
		t.Error("DefaultPackageMarkers must not be empty")
	}
	if len(DefaultConventionalDirs) == 0 {
		t.Error("DefaultConventionalDirs must not be empty")
	}
	if DefaultMaxDepth <= 0 {
		t.Error("DefaultMaxDepth must be positive")
	}

	// Verify specific expected entries.
	markers := make(map[string]bool, len(DefaultPackageMarkers))
	for _, m := range DefaultPackageMarkers {
		markers[m] = true
	}
	for _, want := range []string{"go.mod", "package.json", "Cargo.toml", "pom.xml"} {
		if !markers[want] {
			t.Errorf("DefaultPackageMarkers missing %q", want)
		}
	}
}
