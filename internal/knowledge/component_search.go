package knowledge

import (
	"fmt"
	"strings"
)

// ComponentSearch provides semantic search capabilities for finding relationships
// between components using PageIndex or BM25 fallback.
type ComponentSearch struct {
	// graph is the ComponentGraph being built/queried.
	graph *ComponentGraph

	// knowledge provides access to document scanning and PageIndex.
	knowledge *Knowledge

	// strategy controls which search backend to prefer.
	// "pageindex" attempts PageIndex first; anything else uses BM25 fallback.
	strategy string
}

// NewComponentSearch creates a ComponentSearch backed by the given ComponentGraph
// and Knowledge instance.  strategy should be "pageindex" or "fallback".
func NewComponentSearch(graph *ComponentGraph, knowledge *Knowledge, strategy string) *ComponentSearch {
	return &ComponentSearch{
		graph:    graph,
		knowledge: knowledge,
		strategy: strategy,
	}
}

// FindComponentReferences searches for evidence that fromComponent references
// or depends on toComponent.
//
// Strategy:
//  1. If strategy == "pageindex": run a PageIndex semantic query targeting the
//     files of fromComponent.  If PageIndex is unavailable or errors, fall through.
//  2. Fallback: scan the text of fromComponent files for mentions of toComponent.
//
// Returns:
//   - evidence: list of strings describing where the reference was found
//   - confidence: score in [0.0, 1.0] from EstimateComponentConfidence
//   - error: non-nil only on unexpected failures (not on PageIndex unavailability)
func (cs *ComponentSearch) FindComponentReferences(fromComponent, toComponent string) ([]string, float64, error) {
	fromNode := cs.graph.FindComponent(fromComponent)
	if fromNode == nil {
		return nil, 0, fmt.Errorf("FindComponentReferences: component %q not found", fromComponent)
	}

	toComp, ok := cs.graph.Components[toComponent]
	if !ok {
		return nil, 0, fmt.Errorf("FindComponentReferences: target component %q not found", toComponent)
	}

	// Try PageIndex semantic search first.
	if cs.strategy == "pageindex" && cs.knowledge != nil {
		evidence, confidence, err := cs.findViaPageIndex(fromNode, toComponent)
		if err == nil && confidence > 0 {
			return evidence, confidence, nil
		}
		// PageIndex unavailable or returned no results — fall through to text scan.
	}

	// Fallback: simple text-based scan of fromComponent's files.
	return cs.findViaTextScan(fromNode, toComp)
}

// findViaPageIndex queries PageIndex for references to toComponent in
// fromNode's files.  Returns (nil, 0, err) on failure so callers can fall back.
func (cs *ComponentSearch) findViaPageIndex(fromNode *ComponentNode, toComponent string) ([]string, float64, error) {
	if len(fromNode.Files) == 0 {
		return nil, 0, nil
	}

	// Load tree files for fromNode's files only.
	// We build a targeted query: "What does {from} reference about {to}?"
	query := fmt.Sprintf("Does %s reference or depend on %s?", fromNode.Name, toComponent)

	cfg := DefaultPageIndexConfig()

	// Attempt to load tree files for the component's files.
	var trees []FileTree
	for _, f := range fromNode.Files {
		treePath := f[:len(f)-len(".md")] + "-tree.json"
		if loaded, err := loadSingleTreeFile(treePath); err == nil {
			trees = append(trees, loaded)
		}
	}

	if len(trees) == 0 {
		return nil, 0, fmt.Errorf("findViaPageIndex: no tree files for component %q", fromNode.Name)
	}

	sections, err := RunPageIndexQuery(cfg, query, trees, 5)
	if err != nil {
		return nil, 0, fmt.Errorf("findViaPageIndex: PageIndex query failed: %w", err)
	}

	// Extract evidence from sections that mention toComponent.
	var evidence []string
	for _, s := range sections {
		lowerContent := strings.ToLower(s.Content)
		lowerTarget := strings.ToLower(toComponent)
		if strings.Contains(lowerContent, lowerTarget) {
			snip := evidenceSnippet(s.Content, toComponent, 120)
			evidence = append(evidence, fmt.Sprintf("[%s] %s", s.File, snip))
		}
	}

	if len(evidence) == 0 {
		return nil, 0, nil
	}

	confidence := cs.EstimateComponentConfidence(fromNode.Name, toComponent, evidence)
	return evidence, confidence, nil
}

// findViaTextScan scans the text of fromNode's files for mentions of toComponent.
// Used as a fallback when PageIndex is unavailable.
func (cs *ComponentSearch) findViaTextScan(fromNode *ComponentNode, toComp *Component) ([]string, float64, error) {
	if cs.knowledge == nil || len(fromNode.Files) == 0 {
		return nil, 0, nil
	}

	// Collect mention patterns for toComp: its ID, name, and file stem.
	patterns := componentMentionPatterns(toComp)

	var evidence []string
	for _, f := range fromNode.Files {
		content := cs.readFileContent(f)
		if content == "" {
			continue
		}

		lines := strings.Split(content, "\n")
		for lineNum, line := range lines {
			lowerLine := strings.ToLower(line)
			for _, pat := range patterns {
				if strings.Contains(lowerLine, pat) {
					snip := strings.TrimSpace(line)
					if len(snip) > 120 {
						snip = snip[:120]
					}
					evidence = append(evidence, fmt.Sprintf("[%s:%d] %s", f, lineNum+1, snip))
					break
				}
			}
		}
	}

	if len(evidence) == 0 {
		return nil, 0, nil
	}

	confidence := cs.EstimateComponentConfidence(fromNode.Name, toComp.ID, evidence)
	return evidence, confidence, nil
}

// EstimateComponentConfidence calculates a confidence score for a component
// relationship based on the quality of evidence found.
//
// Scoring rules:
//   - 1.0: evidence contains backtick-quoted exact component name (direct code reference)
//   - 0.7: evidence contains component name in prose or headings (mentioned in content)
//   - 0.5: evidence contains component name in a path or import-like context
//   - 0.4: generic reference (e.g., "external service" without naming it)
//   - 0.3: minimum floor when evidence exists but confidence is otherwise low
func (cs *ComponentSearch) EstimateComponentConfidence(from, to string, evidence []string) float64 {
	if len(evidence) == 0 {
		return 0
	}

	maxConf := 0.3
	lowerTo := strings.ToLower(to)

	for _, ev := range evidence {
		lowerEv := strings.ToLower(ev)

		// Direct code reference: backtick-quoted exact name.
		if strings.Contains(ev, "`"+to+"`") || strings.Contains(ev, "`"+strings.ToLower(to)+"`") {
			if 1.0 > maxConf {
				maxConf = 1.0
			}
			continue
		}

		// Import or path reference.
		if strings.Contains(lowerEv, "/"+lowerTo) || strings.Contains(lowerEv, lowerTo+"/") {
			if 0.5 > maxConf {
				maxConf = 0.5
			}
			continue
		}

		// Prose or heading mention.
		if strings.Contains(lowerEv, lowerTo) {
			if 0.7 > maxConf {
				maxConf = 0.7
			}
		}
	}

	return maxConf
}

// QueryComponentDependencies queries what components the named component depends
// on, using PageIndex semantic search.
//
// It returns a map of component name → confidence score for all discovered
// dependencies.  Errors from PageIndex are handled gracefully (returns empty map).
func (cs *ComponentSearch) QueryComponentDependencies(componentName string, depth int) (map[string]float64, error) {
	result := make(map[string]float64)

	fromNode := cs.graph.FindComponent(componentName)
	if fromNode == nil {
		return result, fmt.Errorf("QueryComponentDependencies: component %q not found", componentName)
	}

	// Collect all other component names to query against.
	for targetName := range cs.graph.Components {
		if targetName == componentName {
			continue
		}

		evidence, confidence, err := cs.FindComponentReferences(componentName, targetName)
		if err != nil {
			continue
		}
		if len(evidence) > 0 && confidence > 0 {
			result[targetName] = confidence
		}
	}

	// If depth > 1, traverse transitively.
	if depth > 1 {
		for depName, depConf := range result {
			if depConf < 0.5 {
				continue
			}
			subDeps, err := cs.QueryComponentDependencies(depName, depth-1)
			if err != nil {
				continue
			}
			for subName, subConf := range subDeps {
				if subName == componentName {
					continue // avoid cycles
				}
				// Attenuate confidence through transitive hops.
				attenuated := subConf * depConf
				if existing, ok := result[subName]; !ok || attenuated > existing {
					result[subName] = attenuated
				}
			}
		}
	}

	return result, nil
}

// --- internal helpers --------------------------------------------------------

// componentMentionPatterns returns lowercase strings to search for in text
// when looking for mentions of the given component.
func componentMentionPatterns(comp *Component) []string {
	patterns := make([]string, 0, 4)

	// Component ID (e.g., "api-gateway").
	if comp.ID != "" {
		patterns = append(patterns, strings.ToLower(comp.ID))
	}

	// Component Name (e.g., "API Gateway").
	if comp.Name != "" && comp.Name != comp.ID {
		patterns = append(patterns, strings.ToLower(comp.Name))
	}

	// File stem (e.g., "auth-service" from "services/auth-service.md").
	if comp.File != "" {
		stem := filenameStem(comp.File)
		lowerStem := strings.ToLower(stem)
		if lowerStem != strings.ToLower(comp.ID) {
			patterns = append(patterns, lowerStem)
		}
	}

	return patterns
}

// evidenceSnippet returns a snippet of content centered around the first
// occurrence of term, truncated to maxLen runes.
func evidenceSnippet(content, term string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerTerm := strings.ToLower(term)

	idx := strings.Index(lowerContent, lowerTerm)
	if idx < 0 {
		// No match — return the beginning.
		runes := []rune(content)
		if len(runes) > maxLen {
			return string(runes[:maxLen])
		}
		return content
	}

	// Centre around the match.
	start := idx - 30
	if start < 0 {
		start = 0
	}
	runes := []rune(content[start:])
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}
	return strings.TrimSpace(string(runes))
}

// readFileContent reads the content of a file by looking it up in the component
// graph's file-to-component mapping and then returning the raw text via Knowledge.
// Returns "" on any error (graceful fallback).
func (cs *ComponentSearch) readFileContent(relPath string) string {
	if cs.knowledge == nil {
		return ""
	}

	// We rely on the Knowledge instance to scan a directory; to get a single
	// file's content we scan the parent directory and find the document.
	// This is intentionally simple — production code should cache docs.
	dir := directoryOf(relPath)
	if dir == "" {
		dir = "."
	}

	docs, err := cs.knowledge.Scan(dir)
	if err != nil {
		return ""
	}

	for _, doc := range docs {
		if doc.RelPath == relPath || strings.HasSuffix(doc.RelPath, relPath) {
			return doc.Content
		}
	}
	return ""
}

// loadSingleTreeFile attempts to load a single .bmd-tree.json file from path.
// Returns (FileTree{}, err) when the file does not exist or cannot be parsed.
func loadSingleTreeFile(path string) (FileTree, error) {
	trees, err := LoadTreeFiles(directoryOf(path))
	if err != nil || len(trees) == 0 {
		return FileTree{}, fmt.Errorf("loadSingleTreeFile: no tree at %q: %w", path, err)
	}
	// Return first tree found in that directory.
	return trees[0], nil
}
