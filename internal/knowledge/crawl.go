package knowledge

import "sort"

// CrawlOptions configures a multi-start graph crawl operation.
//
// FromFiles specifies one or more starting nodes (document IDs).  Direction
// controls edge traversal: "backward" follows incoming edges (who depends on
// me), "forward" follows outgoing edges (what do I depend on), and "both"
// follows edges in either direction.  MaxDepth limits BFS depth (-1 for
// unlimited).  IncludeCycles enables post-traversal cycle detection.
type CrawlOptions struct {
	// FromFiles is the list of starting node IDs (document relative paths).
	FromFiles []string

	// Direction controls which edges are followed during traversal.
	// Valid values: "backward", "forward", "both".
	Direction string

	// MaxDepth limits the BFS traversal depth.  -1 means unlimited.
	// 0 means only start nodes (no traversal).
	MaxDepth int

	// IncludeCycles enables post-traversal cycle detection via DFS.
	IncludeCycles bool
}

// NodeInfo describes a single node discovered during a graph crawl.
//
// Depth is the shortest BFS distance from any start node.  EdgesOut lists the
// IDs of nodes reachable via outgoing edges.  Parents tracks all nodes that
// led to this node during traversal, supporting multi-path analysis.
type NodeInfo struct {
	// File is the document ID (relative path) of this node.
	File string

	// Depth is the shortest BFS distance from any start node.
	Depth int

	// EdgesOut lists the IDs of nodes connected via outgoing edges from this node.
	EdgesOut []string

	// Parents lists node IDs that discovered this node during BFS traversal.
	// A node may have multiple parents when reachable via different branches.
	Parents []string
}

// CrawlResult contains the complete output of a multi-start graph crawl.
type CrawlResult struct {
	// StartNodes lists the IDs of the starting nodes that were found in the graph.
	StartNodes []string

	// Strategy describes the crawl direction used ("backward", "forward", or "both").
	Strategy string

	// Nodes maps document ID to its NodeInfo.
	Nodes map[string]*NodeInfo

	// TotalNodes is the number of unique nodes discovered.
	TotalNodes int

	// TotalEdges is the count of edges between discovered nodes.
	TotalEdges int

	// Cycles contains detected cycles (only populated when IncludeCycles is true).
	Cycles []*Cycle
}

// Cycle describes a circular dependency path discovered during crawl.
type Cycle struct {
	// Path is the ordered list of node IDs forming the cycle.
	// The first and last elements are the same node (closed loop).
	Path []string

	// Type classifies the cycle: "direct" for simple back-edges,
	// "cross_branch" for cycles involving nodes from different start branches.
	Type string

	// Description is a human-readable summary of the cycle.
	Description string
}

// CrawlMulti performs a multi-start BFS traversal of the graph.
//
// All start nodes are enqueued simultaneously.  The BFS respects the
// Direction field in opts: "forward" follows outgoing edges (BySource),
// "backward" follows incoming edges (ByTarget), and "both" follows edges
// in either direction.  When a previously-visited node is re-encountered
// from a different parent, the parent is appended to NodeInfo.Parents
// without re-enqueuing the node (shortest-depth is preserved).
//
// If opts.IncludeCycles is true, a post-traversal DFS detects cycles among
// the discovered subgraph.
//
// Returns a CrawlResult with all discovered nodes, edge counts, and
// optional cycle information.
func (g *Graph) CrawlMulti(opts CrawlOptions) *CrawlResult {
	result := &CrawlResult{
		Strategy: opts.Direction,
		Nodes:    make(map[string]*NodeInfo),
	}

	if opts.Direction == "" {
		opts.Direction = "forward"
		result.Strategy = "forward"
	}

	type bfsEntry struct {
		id     string
		depth  int
		parent string
	}

	var queue []bfsEntry

	// Enqueue all valid start nodes.
	for _, startFile := range opts.FromFiles {
		if _, ok := g.Nodes[startFile]; !ok {
			continue
		}
		result.StartNodes = append(result.StartNodes, startFile)
		result.Nodes[startFile] = &NodeInfo{
			File:  startFile,
			Depth: 0,
		}
		queue = append(queue, bfsEntry{id: startFile, depth: 0})
	}

	// BFS traversal.
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		// Respect depth limit.
		if opts.MaxDepth >= 0 && cur.depth >= opts.MaxDepth {
			continue
		}

		// Collect neighbors based on direction.
		neighbors := g.crawlNeighbors(cur.id, opts.Direction)

		for _, neighborID := range neighbors {
			// Skip nodes not in the graph's node set.
			if _, ok := g.Nodes[neighborID]; !ok {
				continue
			}

			nextDepth := cur.depth + 1

			if existing, visited := result.Nodes[neighborID]; visited {
				// Node already discovered: track additional parent if not a start node revisit.
				if cur.id != "" && !containsStr(existing.Parents, cur.id) {
					existing.Parents = append(existing.Parents, cur.id)
				}
				continue
			}

			// First visit to this node.
			info := &NodeInfo{
				File:  neighborID,
				Depth: nextDepth,
			}
			if cur.id != "" {
				info.Parents = []string{cur.id}
			}
			result.Nodes[neighborID] = info
			queue = append(queue, bfsEntry{id: neighborID, depth: nextDepth, parent: cur.id})
		}
	}

	// Populate EdgesOut for all discovered nodes.
	edgeCount := 0
	for nodeID, info := range result.Nodes {
		for _, edge := range g.BySource[nodeID] {
			if _, inResult := result.Nodes[edge.Target]; inResult {
				info.EdgesOut = append(info.EdgesOut, edge.Target)
				edgeCount++
			}
		}
		sort.Strings(info.EdgesOut)
	}

	result.TotalNodes = len(result.Nodes)
	result.TotalEdges = edgeCount

	// Post-traversal cycle detection.
	if opts.IncludeCycles {
		result.Cycles = g.detectCrawlCycles(result.Nodes)
	}

	return result
}

// crawlNeighbors returns the node IDs reachable from nodeID in the given
// direction.  "forward" returns outgoing edge targets, "backward" returns
// incoming edge sources, "both" returns the union.
func (g *Graph) crawlNeighbors(nodeID, direction string) []string {
	var neighbors []string
	seen := make(map[string]bool)

	if direction == "forward" || direction == "both" {
		for _, edge := range g.BySource[nodeID] {
			if !seen[edge.Target] {
				seen[edge.Target] = true
				neighbors = append(neighbors, edge.Target)
			}
		}
	}

	if direction == "backward" || direction == "both" {
		for _, edge := range g.ByTarget[nodeID] {
			if !seen[edge.Source] {
				seen[edge.Source] = true
				neighbors = append(neighbors, edge.Source)
			}
		}
	}

	return neighbors
}

// detectCrawlCycles runs DFS cycle detection over the subgraph defined by
// discoveredNodes.  Only edges between discovered nodes are considered.
//
// Each detected cycle is classified as "direct" (simple back-edge within a
// single DFS path) or "cross_branch" (involves nodes first reached from
// different start nodes).
func (g *Graph) detectCrawlCycles(discoveredNodes map[string]*NodeInfo) []*Cycle {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	colour := make(map[string]int, len(discoveredNodes))
	parent := make(map[string]string, len(discoveredNodes))

	var cycles []*Cycle
	seen := make(map[string]bool)

	var dfs func(u string)
	dfs = func(u string) {
		colour[u] = gray

		for _, edge := range g.BySource[u] {
			v := edge.Target
			// Only consider edges within the discovered subgraph.
			if _, inScope := discoveredNodes[v]; !inScope {
				continue
			}

			switch colour[v] {
			case white:
				parent[v] = u
				dfs(v)
			case gray:
				// Back edge: reconstruct cycle.
				path := reconstructCrawlCycle(parent, v, u)
				key := cycleSigKey(path)
				if !seen[key] {
					seen[key] = true
					cycleType := classifyCycle(discoveredNodes, path)
					desc := formatCyclePath(path)
					cycles = append(cycles, &Cycle{
						Path:        path,
						Type:        cycleType,
						Description: desc,
					})
				}
			}
		}

		colour[u] = black
	}

	// Deterministic iteration order for stable test output.
	nodeIDs := make([]string, 0, len(discoveredNodes))
	for id := range discoveredNodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, id := range nodeIDs {
		if colour[id] == white {
			dfs(id)
		}
	}

	return cycles
}

// reconstructCrawlCycle builds a cycle path starting and ending at cycleRoot
// by following the parent map backwards from tail.
func reconstructCrawlCycle(parent map[string]string, cycleRoot, tail string) []string {
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
	// Build closed cycle: cycleRoot + middle + cycleRoot.
	path := make([]string, 0, len(middle)+2)
	path = append(path, cycleRoot)
	path = append(path, middle...)
	path = append(path, cycleRoot)
	return path
}

// classifyCycle determines whether a cycle is "direct" (all nodes share
// the same depth-0 ancestry) or "cross_branch" (nodes were reached from
// different start branches).
func classifyCycle(nodes map[string]*NodeInfo, path []string) string {
	// A cycle involving a depth-0 node (start node) is always direct.
	for _, id := range path[:len(path)-1] {
		if info, ok := nodes[id]; ok && info.Depth == 0 {
			return "direct"
		}
	}
	// Check if multiple parents from different branches are involved.
	parentSet := make(map[string]bool)
	for _, id := range path[:len(path)-1] {
		if info, ok := nodes[id]; ok {
			for _, p := range info.Parents {
				parentSet[p] = true
			}
		}
	}
	if len(parentSet) > len(path)-1 {
		return "cross_branch"
	}
	return "direct"
}

// formatCyclePath creates a human-readable description of a cycle.
func formatCyclePath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	desc := path[0]
	for i := 1; i < len(path); i++ {
		desc += " -> " + path[i]
	}
	return desc
}

// cycleSigKey returns a canonical string key for deduplicating cycles,
// using the lexicographically smallest rotation as canonical form.
func cycleSigKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	n := len(cycle) - 1
	if n <= 0 {
		return cycle[0]
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
	key := rotated[0]
	for i := 1; i < len(rotated); i++ {
		key += "|" + rotated[i]
	}
	return key
}

// containsStr returns true if slice contains s.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
