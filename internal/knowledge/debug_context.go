package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── DebugContext types ────────────────────────────────────────────────────────

// ComponentInfo describes a single component found during BFS traversal.
type ComponentInfo struct {
	// Name is the human-readable component name.
	Name string `json:"name"`

	// Path is the relative file path of the primary documentation file for
	// this component.
	Path string `json:"path"`

	// Discovery is the method by which this component was identified.
	// Values: "yaml" | "marker" | "conventional" | "depth_fallback" | "heuristic"
	Discovery string `json:"discovery"`

	// Role is the relationship of this component to the target.
	// Values: "target" | "dependency" | "dependent" | "related"
	Role string `json:"role"`

	// DiscoveryDistance is the number of BFS hops from the target component.
	// Zero for the target itself.
	DiscoveryDistance int `json:"discovery_distance"`
}

// Relationship describes a directed dependency edge between two components.
type Relationship struct {
	// To is the target component ID.
	To string `json:"to"`

	// Confidence is a [0.0, 1.0] score for this relationship.
	Confidence float64 `json:"confidence"`

	// Type describes the nature of the relationship.
	// Values: "depends_on" | "used_by" | "related"
	Type string `json:"type"`

	// Evidence holds specific findings that support this relationship.
	Evidence []string `json:"evidence,omitempty"`
}

// AggregationStats holds metrics from the BFS traversal and documentation
// aggregation process.
type AggregationStats struct {
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time"`
	ComponentsVisited int       `json:"components_visited"`
	EdgesTraversed    int       `json:"edges_traversed"`
	DocumentationSize int       `json:"documentation_size_bytes"`
	TraversalDepth    int       `json:"traversal_depth"`
}

// DebugContext is the STATUS-01 compliant payload produced by BFS traversal.
// It aggregates all documentation and relationships needed for debugging a
// component in a monorepo.
type DebugContext struct {
	// TargetComponent is the entry-point component name.
	TargetComponent string `json:"target_component"`

	// QueryDescription is the user's original question / intent.
	QueryDescription string `json:"query_description"`

	// Components is the list of all components found during traversal.
	Components []ComponentInfo `json:"components"`

	// Relationships maps component ID → list of outgoing relationships.
	Relationships map[string][]Relationship `json:"relationships"`

	// Documentation maps component ID → aggregated markdown content.
	Documentation map[string]string `json:"documentation"`

	// Stats holds timing and traversal metrics.
	Stats AggregationStats `json:"stats"`
}

// ─── BFS node ─────────────────────────────────────────────────────────────────

// bfsNode is an internal queue entry for the ComponentBFS traversal.
type bfsNode struct {
	componentID string
	depth       int
	role        string // "target" | "dependency" | "dependent"
}

// ─── ComponentBFS ─────────────────────────────────────────────────────────────

// ComponentBFS implements breadth-first traversal over a ComponentGraph,
// aggregating documentation and relationships for each visited component.
type ComponentBFS struct {
	graph           *ComponentGraph
	rootDir         string // absolute path to the monorepo root for file loading
	visited         map[string]bool
	queue           []*bfsNode
	components      map[string]*ComponentInfo
	relationships   map[string][]Relationship
	documentation   map[string]string
	startTime       time.Time
	maxDepthReached int
}

// NewBFS creates a ComponentBFS rooted at targetComponent in graph.
//
// rootDir is the absolute directory path used to resolve component file paths
// when loading markdown documentation.
//
// Returns an error if targetComponent is not found in graph.
func NewBFS(graph *ComponentGraph, targetComponent string, rootDir string) (*ComponentBFS, error) {
	if graph == nil {
		return nil, fmt.Errorf("debug_context.NewBFS: graph must not be nil")
	}
	if _, ok := graph.Components[targetComponent]; !ok {
		return nil, fmt.Errorf("debug_context.NewBFS: component %q not found in graph", targetComponent)
	}

	bfs := &ComponentBFS{
		graph:         graph,
		rootDir:       rootDir,
		visited:       make(map[string]bool),
		queue:         make([]*bfsNode, 0),
		components:    make(map[string]*ComponentInfo),
		relationships: make(map[string][]Relationship),
		documentation: make(map[string]string),
		startTime:     time.Now(),
	}

	// Seed the queue with the target component at depth 0.
	bfs.queue = append(bfs.queue, &bfsNode{
		componentID: targetComponent,
		depth:       0,
		role:        "target",
	})

	return bfs, nil
}

// Traverse performs BFS up to maxDepth hops from the target component.
//
// For each visited component it:
//  1. Marks the component as visited.
//  2. Records ComponentInfo (role, discovery distance).
//  3. Adds outgoing edges as "depends_on" relationships.
//  4. Adds incoming edges as "used_by" relationships.
//  5. Aggregates markdown documentation (up to maxDocBytes total).
//  6. Enqueues unvisited neighbours within depth limit.
//
// Returns when the queue is empty or depth/doc limits are reached.
func (b *ComponentBFS) Traverse(maxDepth int, maxDocBytes int) error {
	totalDocSize := 0

	for len(b.queue) > 0 {
		node := b.queue[0]
		b.queue = b.queue[1:]

		if b.visited[node.componentID] {
			continue
		}
		b.visited[node.componentID] = true

		comp, ok := b.graph.Components[node.componentID]
		if !ok {
			continue
		}

		// Track the deepest level reached.
		if node.depth > b.maxDepthReached {
			b.maxDepthReached = node.depth
		}

		// Determine discovery method from component confidence.
		discovery := discoveryMethodFromComponent(comp)

		// Record ComponentInfo.
		b.components[node.componentID] = &ComponentInfo{
			Name:              comp.Name,
			Path:              comp.File,
			Discovery:         discovery,
			Role:              node.role,
			DiscoveryDistance: node.depth,
		}

		// Aggregate documentation for this component.
		if totalDocSize < maxDocBytes {
			remaining := maxDocBytes - totalDocSize
			doc, docErr := b.AggregateDocumentation(node.componentID, remaining)
			if docErr == nil && doc != "" {
				b.documentation[node.componentID] = doc
				totalDocSize += len(doc)
			}
		}

		if node.depth >= maxDepth {
			continue
		}

		// Enqueue outgoing dependencies (depends_on).
		outEdges := b.graph.GetOutgoing(node.componentID)
		for _, edge := range outEdges {
			b.addRelationship(node.componentID, Relationship{
				To:         edge.To,
				Confidence: edge.Confidence,
				Type:       "depends_on",
				Evidence:   edge.Evidence,
			})

			if !b.visited[edge.To] {
				role := depRole(node.role)
				b.queue = append(b.queue, &bfsNode{
					componentID: edge.To,
					depth:       node.depth + 1,
					role:        role,
				})
			}
		}

		// Enqueue incoming dependents (used_by) — components that reference this one.
		inEdges := b.graph.GetIncoming(node.componentID)
		for _, edge := range inEdges {
			b.addRelationship(node.componentID, Relationship{
				To:         edge.From,
				Confidence: edge.Confidence,
				Type:       "used_by",
				Evidence:   edge.Evidence,
			})

			if !b.visited[edge.From] {
				b.queue = append(b.queue, &bfsNode{
					componentID: edge.From,
					depth:       node.depth + 1,
					role:        "dependent",
				})
			}
		}
	}

	return nil
}

// AggregateDocumentation loads and concatenates all markdown content for
// componentID.
//
// The component's primary file is loaded from rootDir.  The content is
// prefixed with a heading identifying the component.
//
// If the total content exceeds maxSize bytes, it is truncated with an
// ellipsis marker.
//
// Returns ("", nil) when the component has no file or the file cannot be read.
func (b *ComponentBFS) AggregateDocumentation(componentID string, maxSize int) (string, error) {
	comp, ok := b.graph.Components[componentID]
	if !ok {
		return "", nil
	}
	if comp.File == "" {
		return "", nil
	}

	// Resolve the absolute path.
	absPath := filepath.Join(b.rootDir, filepath.FromSlash(comp.File))
	raw, err := os.ReadFile(absPath) //nolint:gosec
	if err != nil {
		// File unreadable — return empty, not an error (graceful degradation).
		return "", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n\n", comp.Name)
	sb.Write(raw)

	content := sb.String()

	if maxSize > 0 && len(content) > maxSize {
		// Truncate to maxSize, keeping unicode boundaries by working on runes.
		runes := []rune(content)
		truncLimit := maxSize - len("\n...[truncated]")
		if truncLimit < 0 {
			truncLimit = 0
		}
		if truncLimit > len(runes) {
			truncLimit = len(runes)
		}
		content = string(runes[:truncLimit]) + "\n...[truncated]"
	}

	return content, nil
}

// BuildDebugContext assembles a DebugContext from the current traversal state.
//
// This method should be called after Traverse has completed.
// It populates all fields including aggregation stats.
func (b *ComponentBFS) BuildDebugContext(targetComponent, query string) *DebugContext {
	endTime := time.Now()

	// Collect ComponentInfo slice in deterministic order (target first, then
	// sorted by discovery distance then name).
	compInfos := make([]ComponentInfo, 0, len(b.components))
	for _, ci := range b.components {
		compInfos = append(compInfos, *ci)
	}
	sortComponentInfos(compInfos)

	// Count total documentation size.
	totalDocSize := 0
	for _, doc := range b.documentation {
		totalDocSize += len(doc)
	}

	// Count total edges traversed.
	edgesTotal := 0
	for _, rels := range b.relationships {
		edgesTotal += len(rels)
	}

	return &DebugContext{
		TargetComponent:  targetComponent,
		QueryDescription: query,
		Components:       compInfos,
		Relationships:    b.relationships,
		Documentation:    b.documentation,
		Stats: AggregationStats{
			StartTime:         b.startTime,
			EndTime:           endTime,
			ComponentsVisited: len(b.visited),
			EdgesTraversed:    edgesTotal,
			DocumentationSize: totalDocSize,
			TraversalDepth:    b.maxDepthReached,
		},
	}
}

// ToJSON marshals a DebugContext into a STATUS-01 compliant ContractResponse
// JSON envelope.
//
// Returns the JSON bytes and any marshaling error.
func (dc *DebugContext) ToJSON() ([]byte, error) {
	msg := fmt.Sprintf(
		"Debug context assembled for %q: %d components, %d bytes documentation",
		dc.TargetComponent,
		len(dc.Components),
		dc.Stats.DocumentationSize,
	)

	var status string
	if len(dc.Components) == 0 {
		status = "empty"
	} else {
		status = "ok"
	}

	var resp ContractResponse
	if status == "empty" {
		resp = NewEmptyResponse(msg, dc)
	} else {
		resp = NewOKResponse(msg, dc)
	}

	return json.MarshalIndent(resp, "", "  ")
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// addRelationship appends rel to the relationships map for fromID,
// deduplicating by (To, Type) pair.
func (b *ComponentBFS) addRelationship(fromID string, rel Relationship) {
	existing := b.relationships[fromID]
	for _, r := range existing {
		if r.To == rel.To && r.Type == rel.Type {
			return // already recorded
		}
	}
	b.relationships[fromID] = append(b.relationships[fromID], rel)
}

// discoveryMethodFromComponent returns a discovery method string based on
// the component confidence score. Confidence tiers match constants in
// components.go.
func discoveryMethodFromComponent(comp *Component) string {
	if comp == nil {
		return "depth_fallback"
	}
	switch {
	case comp.Confidence >= ConfidenceConfigured:
		return "yaml"
	case comp.Confidence >= ConfidenceComponentFilename:
		return "marker"
	case comp.Confidence >= ConfidenceComponentHeading:
		return "conventional"
	default:
		return "depth_fallback"
	}
}

// depRole returns the appropriate role for an outgoing dependency's target.
// If the current node is the "target", its dependencies are "dependency".
// Otherwise, they are "related".
func depRole(currentRole string) string {
	if currentRole == "target" {
		return "dependency"
	}
	return "related"
}

// evidenceSlice wraps a single evidence string into a string slice, returning
// nil when evidence is empty.
func evidenceSlice(evidence string) []string {
	if evidence == "" {
		return nil
	}
	return []string{evidence}
}

// sortComponentInfos sorts a slice of ComponentInfo: target first, then by
// discovery distance ascending, then by name alphabetically.
func sortComponentInfos(infos []ComponentInfo) {
	// Insertion sort is fine for small slices; use a stable in-place sort.
	for i := 1; i < len(infos); i++ {
		for j := i; j > 0; j-- {
			a, bb := infos[j-1], infos[j]
			// target always first
			if bb.Role == "target" && a.Role != "target" {
				infos[j-1], infos[j] = infos[j], infos[j-1]
				continue
			}
			if a.Role == "target" {
				break
			}
			// then by distance
			if bb.DiscoveryDistance < a.DiscoveryDistance {
				infos[j-1], infos[j] = infos[j], infos[j-1]
				continue
			}
			if bb.DiscoveryDistance > a.DiscoveryDistance {
				break
			}
			// then by name
			if bb.Name < a.Name {
				infos[j-1], infos[j] = infos[j], infos[j-1]
			} else {
				break
			}
		}
	}
}
