# Phase 7: Silent Loss Reporting - Research

**Researched:** 2026-03-29
**Domain:** Go CLI — BFS cycle detection and export edge-drop warnings
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

User delegated all implementation decisions to Claude. The following guidelines from research should be followed:

**Cycle reporting (DEBT-02):**
- Detect cycle back-edges inline during existing BFS traversal (no separate algorithm pass)
- Add `cycles_detected` field to JSON envelope metadata with `omitempty`
- Each cycle entry should include the cycle path (component names)
- Keep BFS (correct distances) — do NOT switch to DFS
- Cycle info is informational, not an error — queries still succeed with results
- Both impact and dependencies queries should report cycles

**Edge drop warnings (DEBT-03):**
- Use `fmt.Fprintf(os.Stderr)` matching existing operator messaging patterns
- Print count of dropped edges + list the edge source→target pairs
- Do NOT change SaveGraph return signature — stderr warning is sufficient
- Warning should not break JSON output on stdout
- Consider `--verbose` for detailed edge listing, summary count by default

**Cross-cutting constraints:**
- JSON envelope must remain backward-compatible (all new fields `omitempty`)
- No existing field names, types, or positions may change
- Warnings go to stderr only, never stdout (agents parse stdout)

### Claude's Discretion

All implementation decisions delegated to Claude.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DEBT-02 | Report cycles detected during impact/dependencies BFS traversal in JSON envelope (additive CyclesDetected field with omitempty) | BFS functions `executeImpactReverse` and `executeForwardTraversal` in `query_cli.go` already have a `visited` map; back-edge detection is a single conditional check when encountering an already-visited neighbor |
| DEBT-03 | Warn operators via stderr when SaveGraph drops edges with missing endpoint nodes (count + list at verbose) | `SaveGraph` in `db.go` lines 926-931 has two `continue` statements that silently skip edges with missing source/target nodes; add counter and collection before each `continue` |
</phase_requirements>

## Summary

Phase 7 addresses two independent items that surface information previously silently dropped: (1) cycle back-edges encountered during BFS traversal in query commands, and (2) edges dropped during `SaveGraph` due to missing endpoint nodes. Both items are surgical modifications to well-understood code paths with no new dependencies required.

For DEBT-02 (cycle reporting), the two BFS functions (`executeImpactReverse` at line 496 and `executeForwardTraversal` at line 567 in `query_cli.go`) already maintain a `visited` map. When iterating neighbors, the `if !visited[neighbor]` check already identifies back-edges — the else branch (currently implicit/absent) is exactly where cycle edges should be recorded. A new `CycleEntry` struct and a `CyclesDetected []CycleEntry` field on `QueryEnvelopeMetadata` (with `json:",omitempty"`) provide the output channel. The BFS return signature changes from `([]ImpactNode, []EnrichedRelationship)` to `([]ImpactNode, []EnrichedRelationship, []CycleEntry)`.

For DEBT-03 (edge drop warnings), `SaveGraph` in `db.go` has two `continue` statements at lines 926-931 that skip edges when `graph.Nodes[e.Source]` or `graph.Nodes[e.Target]` is missing. The fix is to add a counter and optional slice collector before each `continue`, then print a summary warning to stderr after the transaction completes. The CONTEXT.md specifies NOT changing the `SaveGraph` return signature — stderr output is sufficient.

**Primary recommendation:** Implement DEBT-02 and DEBT-03 as independent tasks (they touch different files with no overlap), using inline back-edge tracking in BFS for cycles and stderr warnings in SaveGraph/export for dropped edges.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `fmt` | 1.24 | stderr warnings via `fmt.Fprintf(os.Stderr, ...)` | Already used throughout codebase for operator messaging |
| Go stdlib `encoding/json` | 1.24 | JSON serialization with `omitempty` tags | Already used for all query output |

### Supporting

No additional libraries needed. All techniques use patterns already present in the codebase.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Inline BFS back-edge tracking | `graph.go` `DetectCycles()` full-graph DFS | Full-graph pass is O(V+E) regardless of query scope; inline is O(1) per edge, only during traversal |
| stderr warnings | Changing `SaveGraph` return to `([]DroppedEdge, error)` | CONTEXT.md explicitly says NOT to change return signature; stderr is sufficient and matches existing patterns |
| `omitempty` JSON field | Separate `/cycles` endpoint | Breaks the single-envelope contract; cycles are metadata about the traversal, not a separate query |

## Architecture Patterns

### Recommended Project Structure

No new files needed. Changes are to existing files:

```
internal/knowledge/
├── query_cli.go       # DEBT-02: CycleEntry struct, BFS cycle tracking, metadata field
├── query_cli_test.go  # DEBT-02: cycle detection tests with cyclic graph fixtures
├── db.go              # DEBT-03: dropped edge counting in SaveGraph
├── export.go          # DEBT-03: stderr warning after SaveGraph call
└── export_test.go     # DEBT-03: tests for dropped edge warning behavior
```

Additionally, callers of `SaveGraph` outside `export.go`:
- `cmd/graphmd/main.go:163` — `cmdIndex` function
- `internal/knowledge/command_helpers.go:144` — `indexDirectory` helper

### Pattern 1: Inline BFS Back-Edge Detection
**What:** Record cycle back-edges when BFS encounters an already-visited neighbor
**When to use:** When you need cycle information as a byproduct of an existing traversal
**Example:**
```go
// In executeImpactReverse / executeForwardTraversal:
for _, edge := range g.ByTarget[cur.id] {
    // ... confidence filter ...
    if !visited[edge.Source] {
        // existing BFS logic: enqueue, record node/rel
        visited[edge.Source] = true
        // ...
    } else if edge.Source != root {
        // Back-edge detected: edge.Source was already visited
        // Record cycle: cur.id -> edge.Source (already visited)
        cycles = append(cycles, CycleEntry{
            Edge:    fmt.Sprintf("%s -> %s", cur.id, edge.Source),
            Message: fmt.Sprintf("cycle: %s already visited at depth %d", edge.Source, ...),
        })
    }
}
```

### Pattern 2: Stderr Warning Collection
**What:** Collect warnings during a function, print summary after completion
**When to use:** When a function should succeed despite data quality issues but operators need visibility
**Example:**
```go
// In SaveGraph, before each continue:
if _, ok := graph.Nodes[e.Source]; !ok {
    droppedCount++
    if verbose {
        droppedEdges = append(droppedEdges, fmt.Sprintf("%s -> %s (missing source)", e.Source, e.Target))
    }
    continue
}

// After transaction completes successfully in export.go:
if droppedCount > 0 {
    fmt.Fprintf(os.Stderr, "  Warning: %d edges dropped (missing endpoint nodes)\n", droppedCount)
}
```

**Note on CONTEXT.md constraint:** The CONTEXT.md says "Do NOT change SaveGraph return signature" but also says "Print count of dropped edges + list the edge source→target pairs." Since SaveGraph is a `Database` method that returns only `error`, the warning must be printed from within SaveGraph itself (to stderr) or via a separate mechanism. The cleanest approach that respects the constraint: add a `DroppedEdges` field to `Database` struct or use a callback, but the simplest is to print directly from within `SaveGraph` to stderr. The caller (`export.go` at line 234, `main.go` at line 163) already prints to stderr extensively.

### Anti-Patterns to Avoid
- **Replacing BFS with DFS for cycle detection:** BFS produces correct shortest-distance values; DFS does not. The `Distance` field in `ImpactNode` depends on BFS ordering.
- **Making cycles an error condition:** Cycles are real in infrastructure (A depends on B, B depends on A). The query must still return all reachable nodes; cycles are informational metadata.
- **Printing warnings to stdout:** AI agents parse stdout as JSON. All operator messages go to stderr.
- **Failing SaveGraph on dropped edges:** Dropping edges with missing endpoints is expected behavior for imperfect real-world graph extraction. Changing `continue` to `return error` would break every export on real-world data.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Full-graph cycle detection | Separate `DetectCycles()` call before/after BFS | Inline back-edge check in existing BFS loop | `DetectCycles()` exists in `graph.go` but is O(V+E) over the full graph; inline detection is O(1) per edge and scoped to the actual traversal |
| JSON backward compatibility | Manual field-by-field serialization | `json:",omitempty"` struct tags | Go's `encoding/json` handles absent fields natively; `omitempty` means old consumers see no change |

**Key insight:** Both items are tiny modifications to existing code paths. The risk is over-engineering, not under-engineering. No new algorithms, no new data structures beyond a simple `CycleEntry` struct.

## Common Pitfalls

### Pitfall 1: Cycle Detection Reports Root as a Cycle
**What goes wrong:** The BFS starts with `visited[root] = true`. When processing the root's neighbors, any edge pointing back to root would trigger a "cycle detected" even though it's just the starting node.
**Why it happens:** The `else` branch of `if !visited[neighbor]` fires for root because root is pre-marked as visited.
**How to avoid:** Add `edge.Source != root` (for reverse traversal) or `edge.Target != root` (for forward traversal) guard to the cycle detection branch. Only report cycles between non-root visited nodes, or explicitly report root-involved cycles with appropriate labeling.
**Warning signs:** Every query on a densely-connected graph reports the queried component as a cycle participant.

### Pitfall 2: Cycle Path Reconstruction in BFS
**What goes wrong:** BFS doesn't naturally track the full path to a visited node. Recording "A -> B (cycle)" without the full cycle path (e.g., A -> C -> D -> B -> A) gives incomplete information.
**Why it happens:** BFS uses a `visited` map (boolean), not a parent-tracking map. Reconstructing the full cycle path requires additional bookkeeping.
**How to avoid:** For Phase 7, report the back-edge itself (source -> already-visited target) rather than the full cycle path. The CONTEXT.md says "Each cycle entry should include the cycle path (component names)" — this can be satisfied by recording the back-edge endpoints, since the full traversal path is available in the `AffectedNodes` results. A lightweight approach: track `parent` map during BFS to reconstruct the path from root to the cycle point.
**Warning signs:** Cycle entries contain only edge pairs with no path context.

### Pitfall 3: SaveGraph Warning Printed Inside Transaction
**What goes wrong:** If warnings are printed to stderr inside the `transaction()` callback, they appear even if the transaction later rolls back.
**Why it happens:** `SaveGraph` wraps all operations in `transaction(db.conn, func(tx *sql.Tx) error { ... })`.
**How to avoid:** Collect dropped edge info during the transaction, print warnings after the transaction succeeds. Since CONTEXT.md says don't change the return signature, the simplest approach is to collect into a local slice within the closure, then print after `transaction()` returns nil.
**Warning signs:** Warnings appear for dropped edges even when the export fails.

### Pitfall 4: Test Graph Fixtures Without Cycles
**What goes wrong:** The existing test graph in `setupQueryTestGraph` is a DAG (no cycles). Tests for cycle detection need a separate fixture with cycles.
**Why it happens:** The acyclic test graph was designed for correctness testing of impact/dependencies traversal.
**How to avoid:** Create a `setupCyclicQueryTestGraph` helper that adds a cycle (e.g., `primary-db -> payment-api` creating the cycle `payment-api -> primary-db -> payment-api`). Verify both that cycles are reported AND that query results are still correct (all reachable nodes found, distances correct).
**Warning signs:** Cycle tests pass trivially because the fixture has no cycles.

### Pitfall 5: Multiple SaveGraph Callers Not Updated
**What goes wrong:** SaveGraph is called from 4 locations. If the warning mechanism requires caller-side changes, missing a caller creates inconsistent behavior.
**Why it happens:** `SaveGraph` is called in `export.go:234`, `main.go:163`, `command_helpers.go:144`, and multiple test files.
**How to avoid:** Since CONTEXT.md says not to change the return signature, if warnings are printed from within SaveGraph itself, no caller changes are needed. If a callback or field-based approach is used, audit all callers. The test callers (`qa_validation_test.go`, `edge_provenance_test.go`, `query_cli_test.go`, etc.) typically don't need warning output.
**Warning signs:** `go build ./...` fails after changing SaveGraph signature; or warnings appear in test output.

## Code Examples

### CycleEntry Struct
```go
// CycleEntry describes a cycle back-edge detected during BFS traversal.
type CycleEntry struct {
    From string   `json:"from"`
    To   string   `json:"to"`
    Path []string `json:"path,omitempty"`
}
```

### Extended QueryEnvelopeMetadata
```go
type QueryEnvelopeMetadata struct {
    ExecutionTimeMs int64        `json:"execution_time_ms"`
    NodeCount       int          `json:"node_count"`
    EdgeCount       int          `json:"edge_count"`
    GraphName       string       `json:"graph_name"`
    GraphVersion    string       `json:"graph_version"`
    CreatedAt       string       `json:"created_at"`
    ComponentCount  int          `json:"component_count"`
    CyclesDetected  []CycleEntry `json:"cycles_detected,omitempty"`
}
```

### BFS Back-Edge Detection (Reverse Traversal)
```go
// Inside executeImpactReverse, within the neighbor loop:
if !visited[edge.Source] {
    // existing enqueue logic...
} else {
    // Back-edge: cur.id <- edge.Source, but edge.Source already visited
    cycles = append(cycles, CycleEntry{
        From: edge.Source,
        To:   cur.id,
    })
}
```

### SaveGraph Dropped Edge Warning
```go
// Inside SaveGraph transaction, before each continue:
var droppedCount int
var droppedPairs []string

// At line 926:
if _, ok := graph.Nodes[e.Source]; !ok {
    droppedCount++
    droppedPairs = append(droppedPairs, fmt.Sprintf("  %s -> %s (missing source %q)", e.Source, e.Target, e.Source))
    continue
}
// At line 929:
if _, ok := graph.Nodes[e.Target]; !ok {
    droppedCount++
    droppedPairs = append(droppedPairs, fmt.Sprintf("  %s -> %s (missing target %q)", e.Source, e.Target, e.Target))
    continue
}

// After transaction returns nil:
if droppedCount > 0 {
    fmt.Fprintf(os.Stderr, "  Warning: %d edge(s) dropped (missing endpoint nodes)\n", droppedCount)
    for _, pair := range droppedPairs {
        fmt.Fprintf(os.Stderr, "%s\n", pair)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Silent `continue` on missing nodes | Collect + warn on stderr | Phase 7 | Operators see data quality issues |
| No cycle info in query results | `cycles_detected` in metadata | Phase 7 | AI agents can detect circular dependencies |
| `DetectCycles()` in graph.go (full-graph DFS) | Inline BFS back-edge tracking | Phase 7 | Scoped to query traversal, not full graph |

**Existing infrastructure (retained, not replaced):**
- `graph.go:DetectCycles()` — full-graph DFS cycle detection. Still available for other use cases; not used by query commands.
- `dependencies.go:DetectCycles()` — DependencyAnalyzer cycle detection. Separate subsystem, unaffected by Phase 7.

## Open Questions

1. **Cycle path reconstruction depth**
   - What we know: BFS `visited` map is boolean; reconstructing the full cycle path requires a `parent` map
   - What's unclear: Whether CONTEXT.md's "cycle path (component names)" means the full cycle or just the back-edge endpoints
   - Recommendation: Start with back-edge endpoints only (`from`, `to`). Add `parent` map for path reconstruction if the planner determines full paths are needed. The overhead is one map lookup per BFS step.

2. **SaveGraph warning verbosity**
   - What we know: CONTEXT.md suggests `--verbose` for detailed edge listing, summary count by default
   - What's unclear: How to pass a verbose flag to `SaveGraph` without changing its signature
   - Recommendation: Print summary count always; print individual edge pairs only when total is ≤ 10 (reasonable default). Or add a package-level `var Verbose bool` that export.go sets before calling SaveGraph. The simplest approach is to always print both count and pairs since the list is typically short.

## Sources

### Primary (HIGH confidence)
- `internal/knowledge/query_cli.go` lines 496-632 — BFS traversal implementations with `visited` map pattern
- `internal/knowledge/query_cli.go` lines 21-46 — QueryEnvelope and QueryEnvelopeMetadata struct definitions
- `internal/knowledge/db.go` lines 877-950 — SaveGraph with silent `continue` at lines 926-931
- `internal/knowledge/export.go` lines 100-302 — CmdExport calling SaveGraph at line 234
- `cmd/graphmd/main.go` line 163 — cmdIndex calling SaveGraph
- `internal/knowledge/command_helpers.go` line 144 — indexDirectory calling SaveGraph
- `internal/knowledge/query_cli_test.go` — existing query test patterns and fixtures
- `internal/knowledge/graph.go` lines 296-395 — existing DetectCycles (full-graph DFS, not used by queries)

### Secondary (MEDIUM confidence)
- `.planning/research/SUMMARY.md` — milestone-level research confirming architecture and pitfalls

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all patterns verified in codebase
- Architecture: HIGH — exact file paths, line numbers, and code patterns identified from direct code reading
- Pitfalls: HIGH — pitfalls derived from actual code structure (root-as-cycle, transaction timing, caller audit), not hypothetical

**Research date:** 2026-03-29
**Valid until:** 2026-04-28 (stable codebase, no external dependencies)
