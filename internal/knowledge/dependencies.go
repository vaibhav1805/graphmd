package knowledge

import (
	"sort"
	"strings"
)

// ComponentRef describes a dependency that one component has on another.
// It captures not only the target component but also how the dependency was
// discovered and how confident the system is in it.
type ComponentRef struct {
	// ComponentID is the ID of the depended-upon component.
	ComponentID string

	// Type describes the nature of the dependency.  Common values:
	//   "direct-call" — synchronous RPC / HTTP call
	//   "queue"       — asynchronous message through a queue
	//   "database"    — shared database dependency
	//   "cache"       — shared cache layer
	//   "reference"   — document link / generic reference
	Type string

	// Evidence is the human-readable source of this dependency relationship
	// (e.g. the edge Evidence string or a markdown link).
	Evidence string

	// Confidence is a [0.0, 1.0] score reflecting extraction certainty.
	Confidence float64
}

// dependencyGraph is the internal directed dependency graph used by
// DependencyAnalyzer. It maps component IDs to their outgoing ComponentRefs.
// This is distinct from ComponentGraph (component_graph.go) which is the
// richer component-level graph used for BFS traversal and debug context.
type dependencyGraph struct {
	// Components maps component ID → *Component.
	Components map[string]*Component

	// Dependencies maps component ID → list of its outgoing ComponentRefs.
	Dependencies map[string][]ComponentRef
}

// newDependencyGraph returns an empty, ready-to-use dependencyGraph.
func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{
		Components:   make(map[string]*Component),
		Dependencies: make(map[string][]ComponentRef),
	}
}

// DependencyChain describes the shortest dependency path between two services.
type DependencyChain struct {
	// Path is the ordered list of service IDs from the source to the
	// destination (inclusive of both endpoints).
	Path []string

	// Distance is the number of hops (len(Path) - 1).  Zero when From == To
	// or no path exists.
	Distance int

	// HasCycle indicates that a cycle was detected along the path search.
	// This flag is informational; the path itself is still the shortest one
	// found before the cycle was encountered.
	HasCycle bool

	// Evidence is a short description of how the path was found (e.g. edge
	// evidence strings joined by " → ").
	Evidence string
}

// DependencyAnalyzer extracts and queries component-to-component dependencies
// from a knowledge Graph and a set of detected Components.
//
// It operates on the component-level view of the graph: only nodes that
// correspond to known components are included in the analysis.
type DependencyAnalyzer struct {
	// depGraph is the computed dependency graph (internal, simple format).
	depGraph *dependencyGraph
}

// NewDependencyAnalyzer creates a DependencyAnalyzer and builds the component
// dependency graph from graph and components.
//
// This is the primary entry point.  All subsequent query methods operate on
// the pre-built dependencyGraph so they run in O(degree) time.
func NewDependencyAnalyzer(graph *Graph, components []Component) *DependencyAnalyzer {
	da := &DependencyAnalyzer{}
	da.depGraph = da.buildDepGraph(graph, components)
	return da
}

// buildDepGraph extracts a component-only subgraph from graph using components
// as the set of known component nodes.
//
// Algorithm:
//  1. Index components by file path (Node ID).
//  2. For each edge in the full graph, check whether both Source and Target
//     map to known components.
//  3. Add qualifying edges to the dependencyGraph with appropriate type/confidence.
func (da *DependencyAnalyzer) buildDepGraph(graph *Graph, components []Component) *dependencyGraph {
	sg := newDependencyGraph()

	// Index components by their document file path.
	byFile := make(map[string]*Component, len(components))
	for i := range components {
		c := &components[i]
		sg.Components[c.ID] = c
		byFile[c.File] = c
	}

	// Iterate every edge in the full knowledge graph and check whether both
	// endpoints correspond to known components.
	for _, edge := range graph.Edges {
		srcComp, srcOK := byFile[edge.Source]
		tgtComp, tgtOK := byFile[edge.Target]
		if !srcOK || !tgtOK {
			continue
		}

		ref := ComponentRef{
			ComponentID: tgtComp.ID,
			Type:        edgeTypeToDepType(edge.Type),
			Evidence:    edge.Evidence,
			Confidence:  edge.Confidence,
		}

		// Avoid duplicating refs for the same (src, tgt, type) triple.
		if !hasRef(sg.Dependencies[srcComp.ID], ref) {
			sg.Dependencies[srcComp.ID] = append(sg.Dependencies[srcComp.ID], ref)
		}
	}

	return sg
}

// GetDirectDeps returns the IDs of components that componentID directly depends on.
// Returns nil when componentID is unknown or has no dependencies.
func (da *DependencyAnalyzer) GetDirectDeps(componentID string) []string {
	refs, ok := da.depGraph.Dependencies[componentID]
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if !seen[ref.ComponentID] {
			seen[ref.ComponentID] = true
			ids = append(ids, ref.ComponentID)
		}
	}
	sort.Strings(ids)
	return ids
}

// GetTransitiveDeps returns the IDs of all services reachable from componentID
// by following dependency edges (BFS, no depth limit).
//
// The starting service itself is NOT included.  Returns nil when componentID is
// unknown or has no outgoing dependencies.
func (da *DependencyAnalyzer) GetTransitiveDeps(componentID string) []string {
	if _, ok := da.depGraph.Components[componentID]; !ok {
		return nil
	}

	visited := make(map[string]bool)
	visited[componentID] = true
	queue := []string{componentID}
	var result []string

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, ref := range da.depGraph.Dependencies[cur] {
			if !visited[ref.ComponentID] {
				visited[ref.ComponentID] = true
				result = append(result, ref.ComponentID)
				queue = append(queue, ref.ComponentID)
			}
		}
	}

	sort.Strings(result)
	return result
}

// FindPath returns all simple paths from service from to service to, limited
// to a maximum of maxDepth hops.
//
// Each path is a slice of service IDs starting with from and ending with to.
// Returns nil when no path exists or either service is unknown.
func (da *DependencyAnalyzer) FindPath(from, to string) [][]string {
	const maxDepth = 10 // guard against explosion in dense graphs
	if from == to {
		return nil
	}
	if _, ok := da.depGraph.Components[from]; !ok {
		return nil
	}
	if _, ok := da.depGraph.Components[to]; !ok {
		return nil
	}

	var results [][]string
	visited := make(map[string]bool)

	var dfs func(cur string, path []string)
	dfs = func(cur string, path []string) {
		if len(path)-1 >= maxDepth {
			return
		}
		for _, ref := range da.depGraph.Dependencies[cur] {
			next := ref.ComponentID
			if visited[next] {
				continue
			}
			newPath := append(append([]string{}, path...), next)
			if next == to {
				results = append(results, newPath)
				continue
			}
			visited[next] = true
			dfs(next, newPath)
			visited[next] = false
		}
	}

	visited[from] = true
	dfs(from, []string{from})
	return results
}

// DetectCycles finds all circular dependencies in the service graph using
// iterative DFS with three-colour marking (white/gray/black).
//
// Returns a slice of cycles; each cycle is a slice of service IDs where the
// first and last element are the same service.  Returns nil when the graph
// has no cycles.
func (da *DependencyAnalyzer) DetectCycles() [][]string {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	colour := make(map[string]int, len(da.depGraph.Components))
	parent := make(map[string]string, len(da.depGraph.Components))

	var cycles [][]string
	seen := make(map[string]bool) // dedup identical cycles

	var dfs func(u string)
	dfs = func(u string) {
		colour[u] = gray

		for _, ref := range da.depGraph.Dependencies[u] {
			v := ref.ComponentID
			switch colour[v] {
			case white:
				parent[v] = u
				dfs(v)
			case gray:
				// Back edge — reconstruct cycle path.
				cycle := reconstructServiceCycle(parent, v, u)
				key := cycleKey(cycle)
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
				}
			}
		}

		colour[u] = black
	}

	for id := range da.depGraph.Components {
		if colour[id] == white {
			dfs(id)
		}
	}

	return cycles
}

// FindDependencyChain finds the shortest dependency path from service from to
// service to using BFS.  The search is limited to maxChainDepth hops to
// prevent combinatorial explosion.
//
// Returns a DependencyChain with an empty Path when no route exists.
func (da *DependencyAnalyzer) FindDependencyChain(from, to string) DependencyChain {
	const maxChainDepth = 5

	if from == to {
		return DependencyChain{}
	}
	if _, ok := da.depGraph.Components[from]; !ok {
		return DependencyChain{}
	}
	if _, ok := da.depGraph.Components[to]; !ok {
		return DependencyChain{}
	}

	type bfsEntry struct {
		id       string
		path     []string
		evidence []string
	}

	visited := make(map[string]bool)
	visited[from] = true
	queue := []bfsEntry{{id: from, path: []string{from}}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if len(cur.path)-1 >= maxChainDepth {
			continue
		}

		for _, ref := range da.depGraph.Dependencies[cur.id] {
			if visited[ref.ComponentID] {
				continue
			}
			newPath := append(append([]string{}, cur.path...), ref.ComponentID)
			newEvidence := append(append([]string{}, cur.evidence...), ref.Evidence)

			if ref.ComponentID == to {
				return DependencyChain{
					Path:     newPath,
					Distance: len(newPath) - 1,
					Evidence: strings.Join(newEvidence, " -> "),
				}
			}
			visited[ref.ComponentID] = true
			queue = append(queue, bfsEntry{
				id:       ref.ComponentID,
				path:     newPath,
				evidence: newEvidence,
			})
		}
	}

	// No path found.
	return DependencyChain{}
}

// GetDepGraph returns a read-only view of the computed service dependency
// graph.  Callers should not mutate the returned value.
func (da *DependencyAnalyzer) GetDepGraph() *dependencyGraph {
	return da.depGraph
}

// GetComponentGraph is an alias for GetDepGraph retained for backward
// compatibility with existing call sites.
func (da *DependencyAnalyzer) GetComponentGraph() *dependencyGraph {
	return da.depGraph
}

// --- helpers ----------------------------------------------------------------

// edgeTypeToDepType maps knowledge graph EdgeType values to dependency type
// strings used in ComponentRef.Type.
func edgeTypeToDepType(et EdgeType) string {
	switch et {
	case EdgeCalls:
		return "direct-call"
	case EdgeDependsOn:
		return "direct-call"
	case EdgeMentions:
		return "reference"
	case EdgeReferences:
		return "reference"
	case EdgeImplements:
		return "reference"
	default:
		return "reference"
	}
}

// hasRef returns true when refs already contains a ComponentRef with the same
// ComponentID and Type as ref.
func hasRef(refs []ComponentRef, ref ComponentRef) bool {
	for _, r := range refs {
		if r.ComponentID == ref.ComponentID && r.Type == ref.Type {
			return true
		}
	}
	return false
}

// reconstructServiceCycle builds a cycle path starting and ending at
// cycleRoot by following the parent map backwards from tail.
//
// When the DFS detects a back edge tail→cycleRoot, the cycle is:
//
//	cycleRoot → … → tail → cycleRoot
//
// We reconstruct by walking parent[] from tail back to cycleRoot, building
// the path in reverse, then closing it.
func reconstructServiceCycle(parent map[string]string, cycleRoot, tail string) []string {
	// Collect intermediate nodes from tail back to (but not including) cycleRoot.
	var middle []string
	cur := tail
	for cur != cycleRoot {
		middle = append([]string{cur}, middle...)
		p, ok := parent[cur]
		if !ok {
			break
		}
		cur = p
	}
	// Build: cycleRoot + middle + cycleRoot (closed cycle).
	path := make([]string, 0, len(middle)+2)
	path = append(path, cycleRoot)
	path = append(path, middle...)
	path = append(path, cycleRoot)
	return path
}

// cycleKey returns a canonical string key for a cycle so duplicates can be
// detected regardless of rotation.
func cycleKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	// Use the lexicographically smallest rotation as canonical form.
	n := len(cycle) - 1 // last element == first element; work with n distinct nodes
	if n <= 0 {
		return strings.Join(cycle, "|")
	}
	min := 0
	for i := 1; i < n; i++ {
		if cycle[i] < cycle[min] {
			min = i
		}
	}
	rotated := make([]string, n)
	for i := 0; i < n; i++ {
		rotated[i] = cycle[(min+i)%n]
	}
	return strings.Join(rotated, "|")
}
