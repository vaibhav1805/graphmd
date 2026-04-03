package knowledge

import (
	"fmt"
	"strings"
)

// ComponentNode is a vertex in the component-level graph.
// Each node represents a logical component (service, package, module) that
// may contain multiple markdown files.
type ComponentNode struct {
	// Name is the component identifier (matches Component.ID).
	Name string

	// Path is the relative path of the component's root directory.
	Path string

	// Files holds all markdown file paths belonging to this component.
	Files []string

	// InDegree is the count of edges pointing to this component.
	InDegree int

	// OutDegree is the count of edges from this component.
	OutDegree int
}

// ComponentEdge is a directed relationship between two components.
type ComponentEdge struct {
	// From is the name of the source component.
	From string

	// To is the name of the target component.
	To string

	// Confidence is a score in [0.0, 1.0] indicating how certain the system
	// is that a real dependency or relationship exists.
	Confidence float64

	// Type describes the semantic relationship category.
	// Common values: "depends_on", "used_by", "related".
	Type string

	// Evidence holds the references or mentions found that support this edge.
	Evidence []string
}

// ComponentGraph is a directed component-level graph where nodes represent
// logical components and edges represent typed, confidence-scored relationships.
//
// Zero value is NOT valid; always create via NewComponentGraph or newComponentGraph.
type ComponentGraph struct {
	// Components maps component name → *Component (the original Component data).
	Components map[string]*Component

	// Nodes maps component name → *ComponentNode (graph metadata).
	Nodes map[string]*ComponentNode

	// Edges is an adjacency list: [from][to]*ComponentEdge.
	// Used by Wave 2+ graph construction pipeline.
	Edges map[string]map[string]*ComponentEdge

	// Dependencies maps component ID → outgoing ComponentRefs.
	// Used by DependencyAnalyzer, commands.go, and debug_context.go for BFS
	// traversal.  Kept in sync with Edges when edges are added via AddEdge.
	Dependencies map[string][]ComponentRef

	// FileToComponent maps file path → component name.
	FileToComponent map[string]string

	// ScanConfig is the scan configuration inherited from the knowledge system.
	ScanConfig *ScanConfig
}

// newComponentGraph returns an empty, ready-to-use ComponentGraph with all
// internal maps initialised.  This is the unexported constructor used by
// DependencyAnalyzer.BuildComponentGraph.
func newComponentGraph() *ComponentGraph {
	return &ComponentGraph{
		Components:      make(map[string]*Component),
		Nodes:           make(map[string]*ComponentNode),
		Edges:           make(map[string]map[string]*ComponentEdge),
		Dependencies:    make(map[string][]ComponentRef),
		FileToComponent: make(map[string]string),
	}
}

// NewComponentGraph initialises a ComponentGraph from a slice of Component values.
// Each component becomes a node in the graph with an empty edge list.
func NewComponentGraph(components []Component) *ComponentGraph {
	cg := &ComponentGraph{
		Components:      make(map[string]*Component, len(components)),
		Nodes:           make(map[string]*ComponentNode, len(components)),
		Edges:           make(map[string]map[string]*ComponentEdge, len(components)),
		Dependencies:    make(map[string][]ComponentRef, len(components)),
		FileToComponent: make(map[string]string),
	}

	for i := range components {
		comp := &components[i]
		cg.Components[comp.ID] = comp
		cg.Nodes[comp.ID] = &ComponentNode{
			Name:  comp.ID,
			Path:  directoryOf(comp.File),
			Files: []string{comp.File},
		}
		cg.Edges[comp.ID] = make(map[string]*ComponentEdge)
	}

	return cg
}

// AddEdge adds or updates a directed edge from → to in the graph.
//
// Deduplication rule: if an edge between (from, to) already exists with a
// higher confidence, the existing edge is kept unchanged.  Otherwise the new
// edge replaces it.
//
// Returns an error when from == to (self-loops not allowed) or when either
// component name is empty.
func (cg *ComponentGraph) AddEdge(from, to string, confidence float64, edgeType string, evidence []string) error {
	if from == "" {
		return fmt.Errorf("ComponentGraph.AddEdge: from must not be empty")
	}
	if to == "" {
		return fmt.Errorf("ComponentGraph.AddEdge: to must not be empty")
	}
	if from == to {
		return fmt.Errorf("ComponentGraph.AddEdge: self-loop not allowed (%q)", from)
	}

	// Ensure the target adjacency map exists (for components added outside
	// NewComponentGraph, e.g., during dynamic expansion).
	if cg.Edges[from] == nil {
		cg.Edges[from] = make(map[string]*ComponentEdge)
	}

	if existing, ok := cg.Edges[from][to]; ok {
		if existing.Confidence >= confidence {
			// Keep the higher-confidence edge.
			return nil
		}
		// Update in-place with the higher-confidence edge data.
		existing.Confidence = confidence
		existing.Type = edgeType
		existing.Evidence = evidence
		return nil
	}

	// New edge — add and update degree counts.
	edge := &ComponentEdge{
		From:       from,
		To:         to,
		Confidence: confidence,
		Type:       edgeType,
		Evidence:   evidence,
	}
	cg.Edges[from][to] = edge

	// Sync to Dependencies map for backward compatibility with DependencyAnalyzer.
	evidenceStr := ""
	if len(evidence) > 0 {
		evidenceStr = evidence[0]
	}
	ref := ComponentRef{
		ComponentID: to,
		Type:        edgeType,
		Evidence:    evidenceStr,
		Confidence:  confidence,
	}
	if !hasRef(cg.Dependencies[from], ref) {
		cg.Dependencies[from] = append(cg.Dependencies[from], ref)
	}

	// Update degree counts on affected nodes.
	if fromNode, ok := cg.Nodes[from]; ok {
		fromNode.OutDegree++
	}
	if toNode, ok := cg.Nodes[to]; ok {
		toNode.InDegree++
	}

	return nil
}

// FindComponent returns the ComponentNode for name, or nil when not found.
func (cg *ComponentGraph) FindComponent(name string) *ComponentNode {
	return cg.Nodes[name]
}

// GetOutgoing returns all edges whose From field is componentName.
// Returns nil (not an error) when the component has no outgoing edges.
func (cg *ComponentGraph) GetOutgoing(componentName string) []*ComponentEdge {
	adj, ok := cg.Edges[componentName]
	if !ok {
		return nil
	}
	edges := make([]*ComponentEdge, 0, len(adj))
	for _, e := range adj {
		edges = append(edges, e)
	}
	return edges
}

// GetIncoming returns all edges whose To field is componentName.
// Returns nil when the component has no incoming edges.
func (cg *ComponentGraph) GetIncoming(componentName string) []*ComponentEdge {
	var edges []*ComponentEdge
	for _, adj := range cg.Edges {
		if e, ok := adj[componentName]; ok {
			edges = append(edges, e)
		}
	}
	return edges
}

// MapFilesToComponents populates the FileToComponent map by matching each file
// path against known component file lists.
//
// The mapping is derived from the Node.Files slices set during construction
// (and augmented by callers).  allFiles is typically the full set of markdown
// paths discovered during a directory scan; any path that already matches a
// node's file list is indexed here for quick reverse lookup.
func (cg *ComponentGraph) MapFilesToComponents(allFiles []string) {
	// Build reverse index from node-owned files first.
	for name, node := range cg.Nodes {
		for _, f := range node.Files {
			cg.FileToComponent[f] = name
		}
	}

	// For any remaining file, try to assign it to a component based on
	// directory prefix matching (longest prefix wins).
	for _, f := range allFiles {
		if _, alreadyMapped := cg.FileToComponent[f]; alreadyMapped {
			continue
		}

		bestMatch := ""
		bestLen := 0
		fileDir := directoryOf(f)

		for name, node := range cg.Nodes {
			if node.Path == "" {
				continue
			}
			if strings.HasPrefix(fileDir, node.Path) && len(node.Path) > bestLen {
				bestLen = len(node.Path)
				bestMatch = name
			}
		}

		if bestMatch != "" {
			cg.FileToComponent[f] = bestMatch
			// Add file to node's file list so it is tracked.
			node := cg.Nodes[bestMatch]
			node.Files = appendIfMissing(node.Files, f)
		}
	}
}

// extractComponentEdges converts file-level edges from a Graph into
// component-level ComponentEdge values.
//
// Rules:
//   - Only edges where both source and target map to known components are included.
//   - Self-component edges (source component == target component) are skipped.
//   - When multiple file-level edges map to the same component pair, the one
//     with the highest Confidence is kept (deduplication by max confidence).
//
// The resulting slice contains one entry per unique (from, to) component pair.
func extractComponentEdges(fileGraph *Graph, fileToComp map[string]string) []*ComponentEdge {
	// best maps "from\x00to" → *ComponentEdge (max-confidence winner).
	best := make(map[string]*ComponentEdge)

	for _, edge := range fileGraph.Edges {
		fromComp, ok1 := fileToComp[edge.Source]
		toComp, ok2 := fileToComp[edge.Target]

		if !ok1 || !ok2 {
			continue // one or both files not mapped to a component
		}
		if fromComp == toComp {
			continue // skip intra-component edges
		}

		key := fromComp + "\x00" + toComp
		evidence := edge.Evidence
		if evidence == "" {
			evidence = fmt.Sprintf("file-level: %s → %s", edge.Source, edge.Target)
		}

		if existing, found := best[key]; found {
			if edge.Confidence > existing.Confidence {
				existing.Confidence = edge.Confidence
				existing.Evidence = append(existing.Evidence, evidence)
			}
		} else {
			best[key] = &ComponentEdge{
				From:       fromComp,
				To:         toComp,
				Confidence: edge.Confidence,
				Type:       mapEdgeType(edge.Type),
				Evidence:   []string{evidence},
			}
		}
	}

	result := make([]*ComponentEdge, 0, len(best))
	for _, e := range best {
		result = append(result, e)
	}
	return result
}

// mapEdgeType converts a file-level EdgeType to a component-level edge type string.
func mapEdgeType(et EdgeType) string {
	switch et {
	case EdgeDependsOn:
		return "depends_on"
	case EdgeCalls:
		return "depends_on"
	case EdgeImplements:
		return "depends_on"
	case EdgeMentions:
		return "related"
	case EdgeRelated:
		return "related"
	default:
		return "related"
	}
}

// BuildComponentGraph builds a ComponentGraph from a list of Components and an
// existing file-level Graph.
//
// Steps:
//  1. Initialise ComponentGraph with components.
//  2. MapFilesToComponents using all nodes from the file graph.
//  3. Extract file-level edges → component edges (deduplication by max confidence).
//  4. For each component pair, optionally use PageIndex (via knowledge) to find
//     additional references (confidence > 0.5 threshold).
//  5. Build final degree counts.
//
// knowledge may be nil; when nil, PageIndex enrichment is skipped.
func BuildComponentGraph(components []Component, fileGraph *Graph, knowledge *Knowledge) (*ComponentGraph, error) {
	if len(components) == 0 {
		return nil, fmt.Errorf("BuildComponentGraph: no components provided")
	}

	cg := NewComponentGraph(components)

	// Collect all file paths from the file graph.
	allFiles := make([]string, 0, len(fileGraph.Nodes))
	for id := range fileGraph.Nodes {
		allFiles = append(allFiles, id)
	}

	// Map files to components.
	cg.MapFilesToComponents(allFiles)

	// Extract component-level edges from file-level edges.
	compEdges := extractComponentEdges(fileGraph, cg.FileToComponent)
	for _, ce := range compEdges {
		_ = cg.AddEdge(ce.From, ce.To, ce.Confidence, ce.Type, ce.Evidence)
	}

	// PageIndex enrichment: for each component pair not already connected,
	// query PageIndex to discover additional relationships.
	if knowledge != nil {
		cs := &ComponentSearch{
			graph:    cg,
			knowledge: knowledge,
			strategy: "pageindex",
		}

		compNames := make([]string, 0, len(cg.Components))
		for name := range cg.Components {
			compNames = append(compNames, name)
		}

		for i, fromName := range compNames {
			for j, toName := range compNames {
				if i == j {
					continue
				}

				// Skip if edge already exists with high confidence.
				if existingEdge, ok := cg.Edges[fromName][toName]; ok && existingEdge.Confidence >= 0.7 {
					continue
				}

				evidence, confidence, err := cs.FindComponentReferences(fromName, toName)
				if err != nil || confidence < 0.5 {
					continue
				}

				_ = cg.AddEdge(fromName, toName, confidence, "depends_on", evidence)
			}
		}
	}

	return cg, nil
}

// BuildComponentGraphFromConfig discovers components from rootDir and builds
// the component-level graph.
//
// It uses the ComponentDetector to find components (config-based or heuristic),
// loads the file-level graph, then delegates to BuildComponentGraph.
func BuildComponentGraphFromConfig(rootDir string, knowledge *Knowledge) (*ComponentGraph, error) {
	if knowledge == nil {
		knowledge = DefaultKnowledge()
	}

	docs, err := knowledge.Scan(rootDir)
	if err != nil {
		return nil, fmt.Errorf("BuildComponentGraphFromConfig: scan %q: %w", rootDir, err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("BuildComponentGraphFromConfig: no documents found in %q", rootDir)
	}

	// Build file-level graph.
	gb := NewGraphBuilder(rootDir)
	fileGraph := gb.Build(docs)

	// Detect components.
	detector := NewComponentDetector()
	components := detector.DetectComponents(fileGraph, docs)

	if len(components) == 0 {
		return nil, fmt.Errorf("BuildComponentGraphFromConfig: no components detected in %q", rootDir)
	}

	return BuildComponentGraph(components, fileGraph, knowledge)
}

// NodeCount returns the number of component nodes in the graph.
func (cg *ComponentGraph) NodeCount() int { return len(cg.Nodes) }

// EdgeCount returns the total number of component edges in the graph.
func (cg *ComponentGraph) EdgeCount() int {
	count := 0
	for _, adj := range cg.Edges {
		count += len(adj)
	}
	return count
}

// --- internal helpers --------------------------------------------------------

// directoryOf returns the directory portion of a file path.
// "services/auth/README.md" → "services/auth"
// "README.md" → ""
func directoryOf(path string) string {
	idx := strings.LastIndexByte(path, '/')
	if idx < 0 {
		return ""
	}
	return path[:idx]
}

// appendIfMissing appends s to slice only when it is not already present.
func appendIfMissing(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}
