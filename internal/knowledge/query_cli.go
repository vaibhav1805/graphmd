package knowledge

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// --- Response envelope types -------------------------------------------------

// QueryEnvelope is the top-level JSON structure returned by all query commands.
// It wraps query parameters, results, and graph metadata into a consistent
// contract that AI agents can parse reliably.
type QueryEnvelope struct {
	Query    QueryEnvelopeParams   `json:"query"`
	Results  interface{}           `json:"results"`
	Metadata QueryEnvelopeMetadata `json:"metadata"`
}

// QueryEnvelopeParams describes the query that was executed.
type QueryEnvelopeParams struct {
	Type          string  `json:"type"`
	Component     string  `json:"component,omitempty"`
	From          string  `json:"from,omitempty"`
	To            string  `json:"to,omitempty"`
	Depth         int     `json:"depth,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
}

// QueryEnvelopeMetadata contains graph-level metadata for the response.
type QueryEnvelopeMetadata struct {
	ExecutionTimeMs int64  `json:"execution_time_ms"`
	NodeCount       int    `json:"node_count"`
	EdgeCount       int    `json:"edge_count"`
	GraphName       string `json:"graph_name"`
	GraphVersion    string `json:"graph_version"`
	CreatedAt       string `json:"created_at"`
	ComponentCount  int    `json:"component_count"`
}

// EnrichedRelationship extends edge data with a human-readable confidence tier.
type EnrichedRelationship struct {
	From             string  `json:"from"`
	To               string  `json:"to"`
	Confidence       float64 `json:"confidence"`
	ConfidenceTier   string  `json:"confidence_tier"`
	Type             string  `json:"type"`
	SourceFile       string  `json:"source_file"`
	ExtractionMethod string  `json:"extraction_method"`
}

// --- Path result types -------------------------------------------------------

// PathResult is the results payload for path queries.
type PathResult struct {
	Paths  []PathInfo `json:"paths"`
	Count  int        `json:"count"`
	Reason string     `json:"reason,omitempty"`
}

// PathInfo describes a single path between two components.
type PathInfo struct {
	Nodes           []string  `json:"nodes"`
	Hops            []HopInfo `json:"hops"`
	TotalConfidence float64   `json:"total_confidence"`
}

// HopInfo describes a single hop in a path.
type HopInfo struct {
	From             string  `json:"from"`
	To               string  `json:"to"`
	Confidence       float64 `json:"confidence"`
	ConfidenceTier   string  `json:"confidence_tier"`
	SourceFile       string  `json:"source_file"`
	ExtractionMethod string  `json:"extraction_method"`
}

// --- List result types -------------------------------------------------------

// ListResult is the results payload for list queries.
type ListResult struct {
	Components []ListComponent `json:"components"`
	Count      int             `json:"count"`
}

// ListComponent describes a component in a list query result.
type ListComponent struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	IncomingEdges int    `json:"incoming_edges"`
	OutgoingEdges int    `json:"outgoing_edges"`
}

// --- Impact/dependencies result types ----------------------------------------

// ImpactResult is the results payload for impact and dependencies queries.
type ImpactResult struct {
	AffectedNodes []ImpactNode           `json:"affected_nodes"`
	Relationships []EnrichedRelationship `json:"relationships"`
}

// ImpactNode describes a node reached during impact or dependencies traversal.
type ImpactNode struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Distance       int    `json:"distance"`
	ConfidenceTier string `json:"confidence_tier"`
}

// --- Error JSON type ---------------------------------------------------------

type queryErrorJSON struct {
	Error       string   `json:"error"`
	Code        string   `json:"code"`
	Suggestions []string `json:"suggestions,omitempty"`
	Action      string   `json:"action,omitempty"`
}

// --- CmdQuery: top-level router ----------------------------------------------

// CmdQuery is the entry point for the `graphmd query` CLI command.
// It routes to subcommands: impact, dependencies, path, list.
func CmdQuery(args []string) error {
	if len(args) == 0 {
		printQueryUsage(os.Stderr)
		return fmt.Errorf("query: subcommand required")
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "impact":
		return cmdQueryImpact(subArgs)
	case "dependencies", "deps":
		return cmdQueryDependencies(subArgs)
	case "path":
		return cmdQueryPath(subArgs)
	case "list":
		return cmdQueryList(subArgs)
	default:
		printQueryUsage(os.Stderr)
		return fmt.Errorf("query: unknown subcommand %q", subcommand)
	}
}

// --- Impact subcommand -------------------------------------------------------

func cmdQueryImpact(args []string) error {
	fs := flag.NewFlagSet("query impact", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	component := fs.String("component", "", "Component to query impact for (required)")
	depthStr := fs.String("depth", "1", "Traversal depth (integer or \"all\")")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence threshold")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query impact: %w", err)
	}

	if *component == "" {
		return writeErrorJSONStdout("--component is required", "MISSING_ARG", nil)
	}

	depth, err := parseDepth(*depthStr)
	if err != nil {
		return writeErrorJSONStdout(err.Error(), "INVALID_ARG", nil)
	}

	g, meta, err := LoadStoredGraph(*graphName)
	if err != nil {
		return handleLoadError(err)
	}

	start := time.Now()

	// Check component exists.
	if _, ok := g.Nodes[*component]; !ok {
		suggestions := suggestComponents(g, *component)
		return writeErrorJSONStdout(
			fmt.Sprintf("component %q not found", *component),
			"NOT_FOUND", suggestions)
	}

	// Impact = reverse traversal: follow ByTarget to find things that depend on this component.
	affectedNodes, relationships := executeImpactReverse(g, *component, depth, minConf)

	elapsed := time.Since(start).Milliseconds()

	result := ImpactResult{
		AffectedNodes: affectedNodes,
		Relationships: relationships,
	}

	var confParam float64
	if *minConf > 0 {
		confParam = *minConf
	}

	envelope := QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "impact",
			Component:     *component,
			Depth:         depth,
			MinConfidence: confParam,
		},
		Results:  result,
		Metadata: buildMetadata(g, meta, *graphName, elapsed),
	}

	return outputEnvelope(envelope, *format, "impact")
}

// --- Dependencies subcommand -------------------------------------------------

func cmdQueryDependencies(args []string) error {
	fs := flag.NewFlagSet("query dependencies", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	component := fs.String("component", "", "Component to query dependencies for (required)")
	depthStr := fs.String("depth", "1", "Traversal depth (integer or \"all\")")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence threshold")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query dependencies: %w", err)
	}

	if *component == "" {
		return writeErrorJSONStdout("--component is required", "MISSING_ARG", nil)
	}

	depth, err := parseDepth(*depthStr)
	if err != nil {
		return writeErrorJSONStdout(err.Error(), "INVALID_ARG", nil)
	}

	g, meta, err := LoadStoredGraph(*graphName)
	if err != nil {
		return handleLoadError(err)
	}

	start := time.Now()

	// Check component exists.
	if _, ok := g.Nodes[*component]; !ok {
		suggestions := suggestComponents(g, *component)
		return writeErrorJSONStdout(
			fmt.Sprintf("component %q not found", *component),
			"NOT_FOUND", suggestions)
	}

	// Dependencies = forward traversal: follow BySource to find what this component depends on.
	affectedNodes, relationships := executeForwardTraversal(g, *component, depth, minConf)

	elapsed := time.Since(start).Milliseconds()

	result := ImpactResult{
		AffectedNodes: affectedNodes,
		Relationships: relationships,
	}

	var confParam float64
	if *minConf > 0 {
		confParam = *minConf
	}

	envelope := QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "dependencies",
			Component:     *component,
			Depth:         depth,
			MinConfidence: confParam,
		},
		Results:  result,
		Metadata: buildMetadata(g, meta, *graphName, elapsed),
	}

	return outputEnvelope(envelope, *format, "dependencies")
}

// --- Path subcommand ---------------------------------------------------------

func cmdQueryPath(args []string) error {
	fs := flag.NewFlagSet("query path", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	from := fs.String("from", "", "Source component (required)")
	to := fs.String("to", "", "Target component (required)")
	limit := fs.Int("limit", 10, "Maximum number of paths to return")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence per hop")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query path: %w", err)
	}

	if *from == "" || *to == "" {
		return writeErrorJSONStdout("--from and --to are required", "MISSING_ARG", nil)
	}

	g, meta, err := LoadStoredGraph(*graphName)
	if err != nil {
		return handleLoadError(err)
	}

	start := time.Now()

	// Validate both components exist.
	if _, ok := g.Nodes[*from]; !ok {
		suggestions := suggestComponents(g, *from)
		return writeErrorJSONStdout(
			fmt.Sprintf("component %q not found", *from),
			"NOT_FOUND", suggestions)
	}
	if _, ok := g.Nodes[*to]; !ok {
		suggestions := suggestComponents(g, *to)
		return writeErrorJSONStdout(
			fmt.Sprintf("component %q not found", *to),
			"NOT_FOUND", suggestions)
	}

	// Find paths.
	rawPaths := g.FindPaths(*from, *to, 20)

	var pathInfos []PathInfo
	for _, nodePath := range rawPaths {
		hops, totalConf, valid := buildHops(g, nodePath, *minConf)
		if !valid {
			continue // some hop below min-confidence
		}
		pathInfos = append(pathInfos, PathInfo{
			Nodes:           nodePath,
			Hops:            hops,
			TotalConfidence: totalConf,
		})
	}

	// Sort by total confidence descending.
	sort.Slice(pathInfos, func(i, j int) bool {
		return pathInfos[i].TotalConfidence > pathInfos[j].TotalConfidence
	})

	// Apply limit.
	if len(pathInfos) > *limit {
		pathInfos = pathInfos[:*limit]
	}

	result := PathResult{
		Paths: pathInfos,
		Count: len(pathInfos),
	}
	if result.Paths == nil {
		result.Paths = []PathInfo{}
	}
	if len(pathInfos) == 0 {
		result.Reason = fmt.Sprintf("no path found between %s and %s", *from, *to)
	}

	elapsed := time.Since(start).Milliseconds()

	var confParam float64
	if *minConf > 0 {
		confParam = *minConf
	}

	envelope := QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "path",
			From:          *from,
			To:            *to,
			MinConfidence: confParam,
		},
		Results:  result,
		Metadata: buildMetadata(g, meta, *graphName, elapsed),
	}

	// No path found is success (exit 0).
	return outputEnvelopeSuccess(envelope, *format, "path")
}

// --- List subcommand ---------------------------------------------------------

func cmdQueryList(args []string) error {
	fs := flag.NewFlagSet("query list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	typeName := fs.String("type", "", "Filter by component type")
	minConf := fs.Float64("min-confidence", 0, "Minimum confidence for connected edges")
	graphName := fs.String("graph", "", "Named graph to query")
	format := fs.String("format", "json", "Output format: json or table")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("query list: %w", err)
	}

	g, meta, err := LoadStoredGraph(*graphName)
	if err != nil {
		return handleLoadError(err)
	}

	start := time.Now()

	var components []ListComponent
	for id, node := range g.Nodes {
		// Type filter.
		if *typeName != "" && string(node.ComponentType) != *typeName {
			continue
		}

		incoming := len(g.ByTarget[id])
		outgoing := len(g.BySource[id])

		// Min-confidence filter: only include nodes where at least one connected edge meets threshold.
		if *minConf > 0 {
			hasQualifyingEdge := false
			for _, e := range g.BySource[id] {
				if e.Confidence >= *minConf {
					hasQualifyingEdge = true
					break
				}
			}
			if !hasQualifyingEdge {
				for _, e := range g.ByTarget[id] {
					if e.Confidence >= *minConf {
						hasQualifyingEdge = true
						break
					}
				}
			}
			if !hasQualifyingEdge {
				continue
			}
		}

		nodeType := string(node.ComponentType)
		if nodeType == "" {
			nodeType = "unknown"
		}

		components = append(components, ListComponent{
			Name:          id,
			Type:          nodeType,
			IncomingEdges: incoming,
			OutgoingEdges: outgoing,
		})
	}

	// Sort by name for deterministic output.
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	result := ListResult{
		Components: components,
		Count:      len(components),
	}
	if result.Components == nil {
		result.Components = []ListComponent{}
	}

	elapsed := time.Since(start).Milliseconds()

	var confParam float64
	if *minConf > 0 {
		confParam = *minConf
	}

	envelope := QueryEnvelope{
		Query: QueryEnvelopeParams{
			Type:          "list",
			MinConfidence: confParam,
		},
		Results:  result,
		Metadata: buildMetadata(g, meta, *graphName, elapsed),
	}

	return outputEnvelopeSuccess(envelope, *format, "list")
}

// --- Reverse traversal for impact queries ------------------------------------

// executeImpactReverse performs BFS following ByTarget (incoming edges) to find
// components that depend on root. This answers "if root fails, what breaks?"
func executeImpactReverse(g *Graph, root string, maxDepth int, minConf *float64) ([]ImpactNode, []EnrichedRelationship) {
	type entry struct {
		id    string
		depth int
	}

	visited := map[string]bool{root: true}
	queue := []entry{{id: root, depth: 0}}

	var nodes []ImpactNode
	var rels []EnrichedRelationship

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		// Follow incoming edges: things that depend on cur.id.
		for _, edge := range g.ByTarget[cur.id] {
			if minConf != nil && *minConf > 0 && edge.Confidence < *minConf {
				continue
			}
			if !visited[edge.Source] {
				visited[edge.Source] = true
				dist := cur.depth + 1

				nodeType := "unknown"
				if n, ok := g.Nodes[edge.Source]; ok && string(n.ComponentType) != "" {
					nodeType = string(n.ComponentType)
				}

				tier := safeScoreToTier(edge.Confidence)

				nodes = append(nodes, ImpactNode{
					Name:           edge.Source,
					Type:           nodeType,
					Distance:       dist,
					ConfidenceTier: string(tier),
				})

				rels = append(rels, EnrichedRelationship{
					From:             edge.Source,
					To:               edge.Target,
					Confidence:       edge.Confidence,
					ConfidenceTier:   string(tier),
					Type:             string(edge.Type),
					SourceFile:       edge.SourceFile,
					ExtractionMethod: edge.ExtractionMethod,
				})

				queue = append(queue, entry{id: edge.Source, depth: dist})
			}
		}
	}

	if nodes == nil {
		nodes = []ImpactNode{}
	}
	if rels == nil {
		rels = []EnrichedRelationship{}
	}
	return nodes, rels
}

// --- Forward traversal for dependencies queries ------------------------------

// executeForwardTraversal performs BFS following BySource (outgoing edges) to find
// what root depends on. This answers "what does root need to work?"
func executeForwardTraversal(g *Graph, root string, maxDepth int, minConf *float64) ([]ImpactNode, []EnrichedRelationship) {
	type entry struct {
		id    string
		depth int
	}

	visited := map[string]bool{root: true}
	queue := []entry{{id: root, depth: 0}}

	var nodes []ImpactNode
	var rels []EnrichedRelationship

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		// Follow outgoing edges: things that cur.id depends on.
		for _, edge := range g.BySource[cur.id] {
			if minConf != nil && *minConf > 0 && edge.Confidence < *minConf {
				continue
			}
			if !visited[edge.Target] {
				visited[edge.Target] = true
				dist := cur.depth + 1

				nodeType := "unknown"
				if n, ok := g.Nodes[edge.Target]; ok && string(n.ComponentType) != "" {
					nodeType = string(n.ComponentType)
				}

				tier := safeScoreToTier(edge.Confidence)

				nodes = append(nodes, ImpactNode{
					Name:           edge.Target,
					Type:           nodeType,
					Distance:       dist,
					ConfidenceTier: string(tier),
				})

				rels = append(rels, EnrichedRelationship{
					From:             edge.Source,
					To:               edge.Target,
					Confidence:       edge.Confidence,
					ConfidenceTier:   string(tier),
					Type:             string(edge.Type),
					SourceFile:       edge.SourceFile,
					ExtractionMethod: edge.ExtractionMethod,
				})

				queue = append(queue, entry{id: edge.Target, depth: dist})
			}
		}
	}

	if nodes == nil {
		nodes = []ImpactNode{}
	}
	if rels == nil {
		rels = []EnrichedRelationship{}
	}
	return nodes, rels
}

// --- Fuzzy component matching ------------------------------------------------

// suggestComponents returns up to 5 component name suggestions similar to the
// input query. Scoring: substring match = 2, prefix match on first 3 chars = 1.
func suggestComponents(g *Graph, query string) []string {
	type suggestion struct {
		name  string
		score int
	}

	lowerQuery := strings.ToLower(query)
	queryParts := strings.FieldsFunc(lowerQuery, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	var candidates []suggestion

	for id := range g.Nodes {
		lowerID := strings.ToLower(id)
		score := 0
		// Substring match in either direction.
		if strings.Contains(lowerID, lowerQuery) || strings.Contains(lowerQuery, lowerID) {
			score += 2
		}
		// Prefix match on first 3 chars.
		if len(lowerQuery) >= 3 && len(lowerID) >= 3 && lowerID[:3] == lowerQuery[:3] {
			score++
		}
		// Word-level overlap: check if any word from the query appears in the ID.
		idParts := strings.FieldsFunc(lowerID, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
		for _, qp := range queryParts {
			if len(qp) < 3 {
				continue
			}
			for _, ip := range idParts {
				if strings.Contains(ip, qp) || strings.Contains(qp, ip) {
					score++
				}
			}
		}
		if score > 0 {
			candidates = append(candidates, suggestion{name: id, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].name < candidates[j].name
	})

	var result []string
	for i, c := range candidates {
		if i >= 5 {
			break
		}
		result = append(result, c.name)
	}
	return result
}

// --- Helpers -----------------------------------------------------------------

func parseDepth(s string) (int, error) {
	if s == "all" {
		return 100, nil
	}
	d, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid depth %q: must be integer or \"all\"", s)
	}
	if d < 1 {
		return 1, nil
	}
	return d, nil
}

func safeScoreToTier(score float64) ConfidenceTier {
	if score < 0.4 {
		return TierThreshold
	}
	return ScoreToTier(score)
}

func buildMetadata(g *Graph, meta *ExportMetadata, graphName string, elapsedMs int64) QueryEnvelopeMetadata {
	m := QueryEnvelopeMetadata{
		ExecutionTimeMs: elapsedMs,
		NodeCount:       len(g.Nodes),
		EdgeCount:       len(g.Edges),
		ComponentCount:  len(g.Nodes),
		GraphName:       graphName,
	}
	if meta != nil {
		m.GraphVersion = meta.Version
		m.CreatedAt = meta.CreatedAt
	}
	return m
}

func buildHops(g *Graph, nodePath []string, minConf float64) ([]HopInfo, float64, bool) {
	var hops []HopInfo
	totalConf := 1.0

	for i := 0; i < len(nodePath)-1; i++ {
		from := nodePath[i]
		to := nodePath[i+1]

		// Find edge between consecutive nodes.
		var foundEdge *Edge
		for _, e := range g.BySource[from] {
			if e.Target == to {
				foundEdge = e
				break
			}
		}

		conf := 0.0
		var sf, em string
		if foundEdge != nil {
			conf = foundEdge.Confidence
			sf = foundEdge.SourceFile
			em = foundEdge.ExtractionMethod
		}

		if minConf > 0 && conf < minConf {
			return nil, 0, false
		}

		tier := safeScoreToTier(conf)

		hops = append(hops, HopInfo{
			From:             from,
			To:               to,
			Confidence:       conf,
			ConfidenceTier:   string(tier),
			SourceFile:       sf,
			ExtractionMethod: em,
		})
		totalConf *= conf
	}

	return hops, totalConf, true
}

// --- Error output ------------------------------------------------------------

func writeErrorJSON(w io.Writer, message, code string, suggestions []string) {
	errObj := queryErrorJSON{
		Error:       message,
		Code:        code,
		Suggestions: suggestions,
	}
	if code == "NO_GRAPH" {
		errObj.Action = "run 'graphmd import <file.zip>' to import a graph first"
	}
	data, _ := json.MarshalIndent(errObj, "", "  ")
	fmt.Fprintln(w, string(data))
}

func writeErrorJSONStdout(message, code string, suggestions []string) error {
	writeErrorJSON(os.Stdout, message, code, suggestions)
	return fmt.Errorf("%s", message)
}

func handleLoadError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "no graph imported") {
		writeErrorJSON(os.Stdout, msg, "NO_GRAPH", nil)
		return err
	}
	if strings.Contains(msg, "not found") {
		writeErrorJSON(os.Stdout, msg, "NOT_FOUND", nil)
		return err
	}
	return err
}

// --- Output formatting -------------------------------------------------------

// outputEnvelope writes the envelope as JSON or table and returns an error
// (non-nil triggers exit 1 for error cases, nil for success).
func outputEnvelope(envelope QueryEnvelope, format, queryType string) error {
	if format == "table" {
		writeTable(os.Stdout, envelope, queryType)
		return nil
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputEnvelopeSuccess always returns nil (for commands where empty results are not errors).
func outputEnvelopeSuccess(envelope QueryEnvelope, format, queryType string) error {
	if format == "table" {
		writeTable(os.Stdout, envelope, queryType)
		return nil
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// writeTable writes tabular output for different query types.
func writeTable(w io.Writer, env QueryEnvelope, queryType string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	switch queryType {
	case "impact", "dependencies":
		result, ok := env.Results.(ImpactResult)
		if !ok {
			fmt.Fprintln(w, "(no results)")
			return
		}
		fmt.Fprintln(tw, "NAME\tTYPE\tDISTANCE\tCONFIDENCE\tTIER")
		for _, n := range result.AffectedNodes {
			conf := ""
			for _, r := range result.Relationships {
				if (queryType == "impact" && r.From == n.Name) || (queryType == "dependencies" && r.To == n.Name) {
					conf = fmt.Sprintf("%.2f", r.Confidence)
					break
				}
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", n.Name, n.Type, n.Distance, conf, n.ConfidenceTier)
		}
		tw.Flush()

	case "path":
		result, ok := env.Results.(PathResult)
		if !ok {
			fmt.Fprintln(w, "(no results)")
			return
		}
		if result.Count == 0 {
			fmt.Fprintf(w, "No paths found. %s\n", result.Reason)
			return
		}
		for i, p := range result.Paths {
			fmt.Fprintf(w, "Path %d (confidence: %.4f):\n", i+1, p.TotalConfidence)
			fmt.Fprintln(tw, "  FROM\tTO\tCONFIDENCE\tTIER")
			for _, h := range p.Hops {
				fmt.Fprintf(tw, "  %s\t%s\t%.2f\t%s\n", h.From, h.To, h.Confidence, h.ConfidenceTier)
			}
			tw.Flush()
			fmt.Fprintln(w)
		}

	case "list":
		result, ok := env.Results.(ListResult)
		if !ok {
			fmt.Fprintln(w, "(no results)")
			return
		}
		fmt.Fprintln(tw, "NAME\tTYPE\tINCOMING\tOUTGOING")
		for _, c := range result.Components {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", c.Name, c.Type, c.IncomingEdges, c.OutgoingEdges)
		}
		tw.Flush()
	}
}

// --- Usage -------------------------------------------------------------------

func printQueryUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: graphmd query <subcommand> [options]

Subcommands:
  impact          Query downstream impact of a component failure
  dependencies    Query what a component depends on (alias: deps)
  path            Find paths between two components
  list            List components with optional filters

Global flags:
  --graph <name>         Select a named graph (default: most recent import)
  --min-confidence <f>   Filter relationships below threshold
  --format json|table    Output format (default: json)

Examples:
  graphmd query impact --component payment-api
  graphmd query impact --component primary-db --depth all
  graphmd query dependencies --component web-frontend --depth all
  graphmd query path --from web-frontend --to primary-db
  graphmd query list --type service --min-confidence 0.7
`)
}
