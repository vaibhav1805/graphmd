package knowledge

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DiscoveredComponent represents a component found via directory-level
// discovery (as opposed to the existing file-level Component detection).
// It maps a logical component name to a filesystem subtree.
type DiscoveredComponent struct {
	// Name is the human-readable component identifier, e.g. "payment", "auth".
	Name string

	// Path is the component root directory, relative to the scan root,
	// using forward slashes (e.g. "services/payment").
	Path string

	// PackageMarker is the file that triggered marker-based detection, e.g.
	// "package.json" or "go.mod". Empty for yaml/conventional/depth methods.
	PackageMarker string

	// DiscoveryMethod describes how this component was found:
	//   "yaml"           — explicitly declared in components.yaml
	//   "marker"         — package marker file found (go.mod, package.json, …)
	//   "conventional"   — path matched a conventional pattern (services/*, …)
	//   "depth_fallback" — top-level directory, depth-based fallback
	DiscoveryMethod string

	// Files holds relative paths (forward slashes from scan root) of all
	// markdown files discovered within this component's directory tree.
	// Populated externally after discovery.
	Files []string
}

// ComponentDiscovery orchestrates the four-level component discovery cascade:
// YAML config → package markers → conventional patterns → depth-based fallback.
type ComponentDiscovery struct {
	// configPath is the explicit path to a components.yaml file, or empty to
	// use the default location (.bmd/components.yaml in the scan root).
	configPath string

	// packageMarkers lists file names that indicate a component boundary.
	packageMarkers []string

	// conventionalDirs lists glob-style patterns for conventional monorepo
	// directory layouts, e.g. ["services/*", "packages/*"].
	conventionalDirs []string

	// includeHidden controls whether directories whose names begin with "."
	// are traversed during marker and depth-based discovery.
	includeHidden bool
}

// NewComponentDiscovery returns a ComponentDiscovery with the supplied options.
// Pass empty strings / nil slices to use the package-level defaults.
func NewComponentDiscovery(configPath string, packageMarkers, conventionalDirs []string, includeHidden bool) *ComponentDiscovery {
	if len(packageMarkers) == 0 {
		packageMarkers = DefaultPackageMarkers
	}
	if len(conventionalDirs) == 0 {
		conventionalDirs = DefaultConventionalDirs
	}
	return &ComponentDiscovery{
		configPath:       configPath,
		packageMarkers:   packageMarkers,
		conventionalDirs: conventionalDirs,
		includeHidden:    includeHidden,
	}
}

// DiscoverComponents runs the full discovery cascade and returns a deduplicated,
// stable list of DiscoveredComponent values.
//
// Cascade order (first successful source wins):
//  1. components.yaml (explicit config)
//  2. Package marker detection (go.mod, package.json, …)
//  3. Conventional pattern matching (services/*, packages/*, …)
//  4. Depth-based fallback (every top-level subdirectory)
//
// DiscoverComponents does NOT populate DiscoveredComponent.Files; callers are
// responsible for walking component directories and attaching markdown paths.
func DiscoverComponents(configPath, rootDir string, includeHidden bool) ([]DiscoveredComponent, error) {
	cd := NewComponentDiscovery(configPath, nil, nil, includeHidden)
	return cd.discover(rootDir)
}

// discover executes the cascade for the given rootDir.
func (cd *ComponentDiscovery) discover(rootDir string) ([]DiscoveredComponent, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("components_discovery: abs(%q): %w", rootDir, err)
	}

	// --- Level 1: explicit components.yaml ---
	yamlPath := cd.configPath
	if yamlPath == "" {
		// Default location inside the scan root.
		yamlPath = filepath.Join(absRoot, ".bmd", "components.yaml")
	}
	components, err := cd.loadComponentsYaml(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("components_discovery: load yaml: %w", err)
	}
	if len(components) > 0 {
		return deduplicateComponents(components), nil
	}

	// --- Level 2: package marker detection ---
	components, err = cd.detectPackageMarkers(absRoot, cd.packageMarkers)
	if err != nil {
		return nil, fmt.Errorf("components_discovery: marker detection: %w", err)
	}
	if len(components) > 0 {
		return deduplicateComponents(components), nil
	}

	// --- Level 3: conventional pattern matching ---
	components, err = cd.detectConventionalPatterns(absRoot, cd.conventionalDirs)
	if err != nil {
		return nil, fmt.Errorf("components_discovery: conventional patterns: %w", err)
	}
	if len(components) > 0 {
		return deduplicateComponents(components), nil
	}

	// --- Level 4: depth-based fallback ---
	components, err = cd.depthBasedFallback(absRoot, DefaultMaxDepth)
	if err != nil {
		return nil, fmt.Errorf("components_discovery: depth fallback: %w", err)
	}
	return deduplicateComponents(components), nil
}

// DiscoveryComponentsYaml represents the structure of a components.yaml file
// for directory-level component discovery (different from ComponentConfig which
// is used for file-level graph-node matching).
type DiscoveryComponentsYaml struct {
	Components []DiscoveryComponentEntry
}

// DiscoveryComponentEntry is a single entry in the discovery components.yaml.
type DiscoveryComponentEntry struct {
	Name string
	Path string
}

// loadComponentsYaml parses a discovery-oriented components.yaml file.
// Returns (nil, nil) when the file does not exist (graceful fallback).
// The expected YAML format is:
//
//	components:
//	  - name: payment
//	    path: services/payment
func (cd *ComponentDiscovery) loadComponentsYaml(path string) ([]DiscoveredComponent, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // not an error — file is optional
		}
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	entries, err := parseDiscoveryYaml(f)
	if err != nil {
		return nil, err
	}

	var components []DiscoveredComponent
	for _, e := range entries {
		if e.Name == "" || e.Path == "" {
			continue
		}
		components = append(components, DiscoveredComponent{
			Name:            e.Name,
			Path:            filepath.ToSlash(e.Path),
			DiscoveryMethod: "yaml",
		})
	}
	return components, nil
}

// detectPackageMarkers walks the directory tree one level deep looking for
// any of the given marker files. Each directory that contains a marker file
// is treated as one component.
//
// Only immediate subdirectories of root are examined (depth=1). The function
// does not recurse into nested subdirectories to avoid creating one component
// per leaf package in deep hierarchies.
func (cd *ComponentDiscovery) detectPackageMarkers(rootDir string, markers []string) ([]DiscoveredComponent, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("readdir %q: %w", rootDir, err)
	}

	var components []DiscoveredComponent
	markerSet := make(map[string]struct{}, len(markers))
	for _, m := range markers {
		markerSet[m] = struct{}{}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !cd.includeHidden {
			continue
		}

		dirPath := filepath.Join(rootDir, name)
		marker, found := findMarkerInDir(dirPath, markerSet)
		if found {
			components = append(components, DiscoveredComponent{
				Name:            name,
				Path:            name,
				PackageMarker:   marker,
				DiscoveryMethod: "marker",
			})
			continue
		}

		// Also look one level deeper (e.g. services/payment/).
		subEntries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			subName := sub.Name()
			if strings.HasPrefix(subName, ".") && !cd.includeHidden {
				continue
			}
			subPath := filepath.Join(dirPath, subName)
			subMarker, subFound := findMarkerInDir(subPath, markerSet)
			if subFound {
				relPath := filepath.ToSlash(filepath.Join(name, subName))
				components = append(components, DiscoveredComponent{
					Name:            subName,
					Path:            relPath,
					PackageMarker:   subMarker,
					DiscoveryMethod: "marker",
				})
			}
		}
	}
	return components, nil
}

// findMarkerInDir returns the first marker file found in dirPath, or ("", false).
func findMarkerInDir(dirPath string, markerSet map[string]struct{}) (string, bool) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := markerSet[e.Name()]; ok {
			return e.Name(), true
		}
	}
	return "", false
}

// detectConventionalPatterns matches directory entries against conventional
// monorepo patterns like "services/*" or "packages/*".
//
// Each pattern element before "/*" defines a parent directory to look in.
// All immediate subdirectories of that parent are collected as components.
// Patterns without "/*" are treated as exact component directories.
func (cd *ComponentDiscovery) detectConventionalPatterns(rootDir string, patterns []string) ([]DiscoveredComponent, error) {
	var components []DiscoveredComponent

	for _, pattern := range patterns {
		// We only support patterns of the form "dir/*" or exact "dir".
		if strings.HasSuffix(pattern, "/*") {
			// Parent directory glob: treat all subdirs as components.
			parent := strings.TrimSuffix(pattern, "/*")
			parentPath := filepath.Join(rootDir, parent)
			info, err := os.Stat(parentPath)
			if err != nil || !info.IsDir() {
				continue // parent doesn't exist — skip silently
			}
			entries, err := os.ReadDir(parentPath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasPrefix(name, ".") && !cd.includeHidden {
					continue
				}
				relPath := filepath.ToSlash(filepath.Join(parent, name))
				components = append(components, DiscoveredComponent{
					Name:            name,
					Path:            relPath,
					DiscoveryMethod: "conventional",
				})
			}
		} else {
			// Exact directory — treat it as a single component.
			dirPath := filepath.Join(rootDir, pattern)
			info, err := os.Stat(dirPath)
			if err != nil || !info.IsDir() {
				continue
			}
			name := filepath.Base(pattern)
			components = append(components, DiscoveredComponent{
				Name:            name,
				Path:            filepath.ToSlash(pattern),
				DiscoveryMethod: "conventional",
			})
		}
	}
	return components, nil
}

// depthBasedFallback walks rootDir up to maxDepth levels deep and treats every
// directory it encounters as a component. maxDepth=1 gives only top-level
// subdirectories; maxDepth=2 also includes their immediate children, etc.
//
// This is the last-resort fallback when no other discovery method yields results.
func (cd *ComponentDiscovery) depthBasedFallback(rootDir string, maxDepth int) ([]DiscoveredComponent, error) {
	var components []DiscoveredComponent

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if path == rootDir {
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") && !cd.includeHidden {
			return filepath.SkipDir
		}

		// Compute depth relative to root (root = depth 0).
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		depth := len(strings.Split(filepath.ToSlash(rel), "/"))

		if depth > maxDepth {
			return filepath.SkipDir
		}

		relPath := filepath.ToSlash(rel)
		components = append(components, DiscoveredComponent{
			Name:            name,
			Path:            relPath,
			DiscoveryMethod: "depth_fallback",
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %q: %w", rootDir, err)
	}
	return components, nil
}

// deduplicateComponents removes components with duplicate names, keeping the
// first occurrence (which reflects cascade priority: yaml > marker > conventional > depth).
func deduplicateComponents(components []DiscoveredComponent) []DiscoveredComponent {
	seen := make(map[string]struct{}, len(components))
	result := make([]DiscoveredComponent, 0, len(components))
	for _, c := range components {
		if _, ok := seen[c.Name]; ok {
			continue
		}
		seen[c.Name] = struct{}{}
		result = append(result, c)
	}
	return result
}

// --- YAML parser for discovery components.yaml ------------------------------

// parseDiscoveryYaml parses the minimal discovery-oriented YAML format.
// It understands:
//
//	components:
//	  - name: payment
//	    path: services/payment
func parseDiscoveryYaml(f *os.File) ([]DiscoveryComponentEntry, error) {
	var entries []DiscoveryComponentEntry
	scanner := bufio.NewScanner(f)

	type state int
	const (
		stateRoot state = iota
		stateComponents
		stateEntry
	)

	current := stateRoot
	var currentEntry *DiscoveryComponentEntry

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		switch current {
		case stateRoot:
			if strings.TrimRight(trimmed, ":") == "components" {
				current = stateComponents
			}

		case stateComponents, stateEntry:
			if strings.HasPrefix(trimmed, "- ") {
				if currentEntry != nil {
					entries = append(entries, *currentEntry)
				}
				currentEntry = &DiscoveryComponentEntry{}
				current = stateEntry
				rest := strings.TrimPrefix(trimmed, "- ")
				parseDiscoveryYamlKV(currentEntry, rest)
			} else if current == stateEntry && currentEntry != nil {
				parseDiscoveryYamlKV(currentEntry, trimmed)
			}
		}
	}

	if currentEntry != nil {
		entries = append(entries, *currentEntry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("yaml scanner: %w", err)
	}
	return entries, nil
}

// parseDiscoveryYamlKV parses a single "key: value" line and sets the field.
func parseDiscoveryYamlKV(entry *DiscoveryComponentEntry, line string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, `"'`)

	switch key {
	case "name":
		entry.Name = value
	case "path":
		entry.Path = value
	}
}
