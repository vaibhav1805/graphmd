package knowledge

import (
	"sort"
	"testing"
)

// --- helpers ----------------------------------------------------------------

// makeTestGraph builds a graph with given nodes and edges for testing.
func makeTestGraph(nodes []*Node, edges []*Edge) *Graph {
	g := NewGraph()
	for _, n := range nodes {
		_ = g.AddNode(n)
	}
	for _, e := range edges {
		_ = g.AddEdge(e)
	}
	return g
}

// mustEdge creates an edge or panics. For test helpers only.
func mustEdge(source, target string, edgeType EdgeType, confidence float64, evidence string) *Edge {
	e, err := NewEdge(source, target, edgeType, confidence, evidence)
	if err != nil {
		panic(err)
	}
	return e
}

// --- Test: Empty graph ------------------------------------------------------

func TestCrawlStats_EmptyGraph(t *testing.T) {
	g := NewGraph()
	stats := ComputeCrawlStats(g)

	if stats.ComponentCount != 0 {
		t.Errorf("ComponentCount: got %d, want 0", stats.ComponentCount)
	}
	if stats.RelationshipCount != 0 {
		t.Errorf("RelationshipCount: got %d, want 0", stats.RelationshipCount)
	}
	if stats.QualityScore != 0 {
		t.Errorf("QualityScore: got %f, want 0", stats.QualityScore)
	}
	if len(stats.ComponentsByType) != 0 {
		t.Errorf("ComponentsByType: got %d entries, want 0", len(stats.ComponentsByType))
	}
	if stats.ConfidenceDistribution != nil {
		t.Errorf("ConfidenceDistribution: got %v, want nil", stats.ConfidenceDistribution)
	}
	if stats.QualityWarnings != nil {
		t.Errorf("QualityWarnings: got %v, want nil", stats.QualityWarnings)
	}
}

// --- Test: Basic counts and quality score -----------------------------------

func TestCrawlStats_BasicCountsAndQualityScore(t *testing.T) {
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "b", Title: "B", Type: "document", ComponentType: ComponentTypeDatabase},
		{ID: "c", Title: "C", Type: "document", ComponentType: ComponentTypeCache},
	}
	edges := []*Edge{
		mustEdge("a", "b", EdgeDependsOn, 0.8, "depends on"),
		mustEdge("a", "c", EdgeReferences, 0.6, "references"),
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	if stats.ComponentCount != 3 {
		t.Errorf("ComponentCount: got %d, want 3", stats.ComponentCount)
	}
	if stats.RelationshipCount != 2 {
		t.Errorf("RelationshipCount: got %d, want 2", stats.RelationshipCount)
	}

	// QualityScore = (0.8 + 0.6) / 2 * 100 = 70.0
	wantScore := 70.0
	if stats.QualityScore != wantScore {
		t.Errorf("QualityScore: got %f, want %f", stats.QualityScore, wantScore)
	}
}

// --- Test: Confidence distribution ------------------------------------------

func TestCrawlStats_ConfidenceDistribution(t *testing.T) {
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "b", Title: "B", Type: "document", ComponentType: ComponentTypeDatabase},
		{ID: "c", Title: "C", Type: "document", ComponentType: ComponentTypeCache},
	}
	edges := []*Edge{
		mustEdge("a", "b", EdgeDependsOn, 0.8, "strong-inference tier"),
		mustEdge("a", "c", EdgeReferences, 0.6, "moderate tier"),
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	if len(stats.ConfidenceDistribution) != 2 {
		t.Fatalf("ConfidenceDistribution: got %d entries, want 2", len(stats.ConfidenceDistribution))
	}

	// Should be ordered: strong-inference first (higher), moderate second
	if stats.ConfidenceDistribution[0].Tier != TierStrongInference {
		t.Errorf("First tier: got %q, want %q", stats.ConfidenceDistribution[0].Tier, TierStrongInference)
	}
	if stats.ConfidenceDistribution[0].Count != 1 {
		t.Errorf("First tier count: got %d, want 1", stats.ConfidenceDistribution[0].Count)
	}
	if stats.ConfidenceDistribution[0].Percentage != 50.0 {
		t.Errorf("First tier percentage: got %f, want 50.0", stats.ConfidenceDistribution[0].Percentage)
	}
	if stats.ConfidenceDistribution[0].RangeLow != 0.75 {
		t.Errorf("First tier RangeLow: got %f, want 0.75", stats.ConfidenceDistribution[0].RangeLow)
	}
	if stats.ConfidenceDistribution[0].RangeHigh != 0.95 {
		t.Errorf("First tier RangeHigh: got %f, want 0.95", stats.ConfidenceDistribution[0].RangeHigh)
	}

	if stats.ConfidenceDistribution[1].Tier != TierModerate {
		t.Errorf("Second tier: got %q, want %q", stats.ConfidenceDistribution[1].Tier, TierModerate)
	}
	if stats.ConfidenceDistribution[1].Count != 1 {
		t.Errorf("Second tier count: got %d, want 1", stats.ConfidenceDistribution[1].Count)
	}
	if stats.ConfidenceDistribution[1].Percentage != 50.0 {
		t.Errorf("Second tier percentage: got %f, want 50.0", stats.ConfidenceDistribution[1].Percentage)
	}
}

// --- Test: All 6 tiers populated -------------------------------------------

func TestCrawlStats_AllTiersPopulated(t *testing.T) {
	// Build a graph with one edge in each tier
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "b", Title: "B", Type: "document", ComponentType: ComponentTypeService},
		{ID: "c", Title: "C", Type: "document", ComponentType: ComponentTypeService},
		{ID: "d", Title: "D", Type: "document", ComponentType: ComponentTypeService},
		{ID: "e", Title: "E", Type: "document", ComponentType: ComponentTypeService},
		{ID: "f", Title: "F", Type: "document", ComponentType: ComponentTypeService},
		{ID: "g", Title: "G", Type: "document", ComponentType: ComponentTypeService},
	}
	edges := []*Edge{
		mustEdge("a", "b", EdgeReferences, 0.98, "explicit"),           // explicit
		mustEdge("a", "c", EdgeDependsOn, 0.80, "strong-inference"),    // strong-inference
		mustEdge("a", "d", EdgeMentions, 0.60, "moderate"),             // moderate
		mustEdge("a", "e", EdgeRelated, 0.50, "weak"),                  // weak
		mustEdge("a", "f", EdgeMentions, 0.43, "semantic"),             // semantic
		mustEdge("a", "g", EdgeRelated, 0.40, "threshold"),             // threshold
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	if len(stats.ConfidenceDistribution) != 6 {
		t.Fatalf("ConfidenceDistribution: got %d entries, want 6", len(stats.ConfidenceDistribution))
	}

	// Verify descending tier order
	expectedTiers := []ConfidenceTier{
		TierExplicit, TierStrongInference, TierModerate, TierWeak, TierSemantic, TierThreshold,
	}
	for i, ts := range stats.ConfidenceDistribution {
		if ts.Tier != expectedTiers[i] {
			t.Errorf("Tier[%d]: got %q, want %q", i, ts.Tier, expectedTiers[i])
		}
		if ts.Count != 1 {
			t.Errorf("Tier[%d] count: got %d, want 1", i, ts.Count)
		}
		// Each is 1/6 * 100
		wantPct := 100.0 / 6.0
		diff := ts.Percentage - wantPct
		if diff < -0.01 || diff > 0.01 {
			t.Errorf("Tier[%d] percentage: got %f, want ~%f", i, ts.Percentage, wantPct)
		}
	}
}

// --- Test: Single tier (others skipped) -------------------------------------

func TestCrawlStats_SingleTierPopulated(t *testing.T) {
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "b", Title: "B", Type: "document", ComponentType: ComponentTypeService},
	}
	edges := []*Edge{
		mustEdge("a", "b", EdgeReferences, 0.99, "explicit only"),
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	if len(stats.ConfidenceDistribution) != 1 {
		t.Fatalf("ConfidenceDistribution: got %d entries, want 1", len(stats.ConfidenceDistribution))
	}
	if stats.ConfidenceDistribution[0].Tier != TierExplicit {
		t.Errorf("Tier: got %q, want %q", stats.ConfidenceDistribution[0].Tier, TierExplicit)
	}
	if stats.ConfidenceDistribution[0].Percentage != 100.0 {
		t.Errorf("Percentage: got %f, want 100.0", stats.ConfidenceDistribution[0].Percentage)
	}
}

// --- Test: Components grouped by type with alphabetical sorting -------------

func TestCrawlStats_ComponentsByType(t *testing.T) {
	nodes := []*Node{
		{ID: "z-service", Title: "Z", Type: "document", ComponentType: ComponentTypeService},
		{ID: "a-service", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "m-db", Title: "M", Type: "document", ComponentType: ComponentTypeDatabase},
		{ID: "b-cache", Title: "B", Type: "document", ComponentType: ComponentTypeCache},
	}
	g := makeTestGraph(nodes, nil)
	stats := ComputeCrawlStats(g)

	// Service group should have 2 IDs sorted alphabetically
	services, ok := stats.ComponentsByType[ComponentTypeService]
	if !ok {
		t.Fatal("missing service group")
	}
	if len(services) != 2 {
		t.Fatalf("service count: got %d, want 2", len(services))
	}
	if services[0] != "a-service" || services[1] != "z-service" {
		t.Errorf("services: got %v, want [a-service z-service]", services)
	}

	// Database group
	dbs, ok := stats.ComponentsByType[ComponentTypeDatabase]
	if !ok {
		t.Fatal("missing database group")
	}
	if len(dbs) != 1 || dbs[0] != "m-db" {
		t.Errorf("databases: got %v, want [m-db]", dbs)
	}

	// Cache group
	caches, ok := stats.ComponentsByType[ComponentTypeCache]
	if !ok {
		t.Fatal("missing cache group")
	}
	if len(caches) != 1 || caches[0] != "b-cache" {
		t.Errorf("caches: got %v, want [b-cache]", caches)
	}
}

// --- Test: Quality warning - orphan node ------------------------------------

func TestCrawlStats_OrphanNodeWarning(t *testing.T) {
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "b", Title: "B", Type: "document", ComponentType: ComponentTypeService},
		{ID: "orphan", Title: "Orphan", Type: "document", ComponentType: ComponentTypeUnknown},
	}
	edges := []*Edge{
		mustEdge("a", "b", EdgeReferences, 0.9, "link"),
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	var orphanWarning *QualityWarning
	for i := range stats.QualityWarnings {
		if stats.QualityWarnings[i].Type == "orphan_node" {
			orphanWarning = &stats.QualityWarnings[i]
			break
		}
	}

	if orphanWarning == nil {
		t.Fatal("expected orphan_node warning, got none")
	}
	if len(orphanWarning.Items) != 1 || orphanWarning.Items[0] != "orphan" {
		t.Errorf("orphan items: got %v, want [orphan]", orphanWarning.Items)
	}
}

// --- Test: Quality warning - dangling edge ----------------------------------

func TestCrawlStats_DanglingEdgeWarning(t *testing.T) {
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
	}
	// Create an edge manually that references a non-existent node
	dangling := &Edge{
		ID:         "dangling-edge",
		Source:     "a",
		Target:     "missing-node",
		Type:       EdgeReferences,
		Confidence: 0.9,
	}
	g := NewGraph()
	_ = g.AddNode(nodes[0])
	// Add edge directly to bypass AddEdge validation if it checks node existence
	g.Edges[dangling.ID] = dangling
	g.BySource[dangling.Source] = append(g.BySource[dangling.Source], dangling)
	g.ByTarget[dangling.Target] = append(g.ByTarget[dangling.Target], dangling)

	stats := ComputeCrawlStats(g)

	var danglingWarning *QualityWarning
	for i := range stats.QualityWarnings {
		if stats.QualityWarnings[i].Type == "dangling_edge" {
			danglingWarning = &stats.QualityWarnings[i]
			break
		}
	}

	if danglingWarning == nil {
		t.Fatal("expected dangling_edge warning, got none")
	}
	if len(danglingWarning.Items) != 1 || danglingWarning.Items[0] != "dangling-edge" {
		t.Errorf("dangling items: got %v, want [dangling-edge]", danglingWarning.Items)
	}
}

// --- Test: Quality warning - weak-only component ----------------------------

func TestCrawlStats_WeakOnlyWarning(t *testing.T) {
	nodes := []*Node{
		{ID: "strong", Title: "Strong", Type: "document", ComponentType: ComponentTypeService},
		{ID: "weak-node", Title: "Weak", Type: "document", ComponentType: ComponentTypeService},
		{ID: "other", Title: "Other", Type: "document", ComponentType: ComponentTypeService},
	}
	edges := []*Edge{
		mustEdge("strong", "other", EdgeReferences, 0.8, "strong edge"),
		mustEdge("weak-node", "other", EdgeMentions, 0.50, "weak edge"),   // weak tier
		mustEdge("strong", "weak-node", EdgeRelated, 0.45, "weak edge 2"), // also weak
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	var weakWarning *QualityWarning
	for i := range stats.QualityWarnings {
		if stats.QualityWarnings[i].Type == "weak_only" {
			weakWarning = &stats.QualityWarnings[i]
			break
		}
	}

	if weakWarning == nil {
		t.Fatal("expected weak_only warning, got none")
	}
	// weak-node has only edges with confidence < 0.55
	if len(weakWarning.Items) != 1 || weakWarning.Items[0] != "weak-node" {
		t.Errorf("weak_only items: got %v, want [weak-node]", weakWarning.Items)
	}
}

// --- Test: No warnings on healthy graph -------------------------------------

func TestCrawlStats_NoWarningsOnHealthyGraph(t *testing.T) {
	nodes := []*Node{
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "b", Title: "B", Type: "document", ComponentType: ComponentTypeService},
	}
	edges := []*Edge{
		mustEdge("a", "b", EdgeReferences, 0.9, "link"),
	}
	g := makeTestGraph(nodes, edges)
	stats := ComputeCrawlStats(g)

	if len(stats.QualityWarnings) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(stats.QualityWarnings), stats.QualityWarnings)
	}
}

// --- Test: Tier ranges are correct ------------------------------------------

func TestCrawlStats_TierRanges(t *testing.T) {
	expected := map[ConfidenceTier][2]float64{
		TierExplicit:        {0.95, 1.0},
		TierStrongInference: {0.75, 0.95},
		TierModerate:        {0.55, 0.75},
		TierWeak:            {0.45, 0.55},
		TierSemantic:        {0.42, 0.45},
		TierThreshold:       {0.40, 0.42},
	}

	bounds := tierBounds()
	if len(bounds) != 6 {
		t.Fatalf("tierBounds: got %d entries, want 6", len(bounds))
	}

	for tier, want := range expected {
		got, ok := bounds[tier]
		if !ok {
			t.Errorf("tierBounds missing tier %q", tier)
			continue
		}
		if got[0] != want[0] || got[1] != want[1] {
			t.Errorf("tierBounds[%q]: got [%f, %f], want [%f, %f]", tier, got[0], got[1], want[0], want[1])
		}
	}
}

// --- Test: Deterministic output (sorted) ------------------------------------

func TestCrawlStats_DeterministicOutput(t *testing.T) {
	nodes := []*Node{
		{ID: "z", Title: "Z", Type: "document", ComponentType: ComponentTypeService},
		{ID: "a", Title: "A", Type: "document", ComponentType: ComponentTypeService},
		{ID: "m", Title: "M", Type: "document", ComponentType: ComponentTypeService},
	}
	g := makeTestGraph(nodes, nil)

	// Run multiple times to verify determinism
	for i := 0; i < 5; i++ {
		stats := ComputeCrawlStats(g)
		services := stats.ComponentsByType[ComponentTypeService]
		if !sort.StringsAreSorted(services) {
			t.Errorf("Run %d: services not sorted: %v", i, services)
		}
	}
}
