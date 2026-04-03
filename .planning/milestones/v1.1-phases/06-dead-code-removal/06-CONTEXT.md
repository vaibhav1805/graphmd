# Phase 6: Dead Code Removal - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Remove orphaned query execution code from Phase 2 that was superseded by Phase 4's CLI implementation. This includes ExecuteImpact, ExecuteCrawl, QueryResult, AffectedNode, QueryEdge, and their tests. The goal is a clean query layer with one unambiguous type hierarchy.

</domain>

<decisions>
## Implementation Decisions

### Deletion Scope

- **Delete only truly orphaned code:** Functions and types with zero live callers get removed
- **Keep shared helpers:** If a function (e.g., computeDistances, passesConfidenceFilter) is used by any live code path, it stays in place
- **Full file deletion when possible:** If query.go and query_test.go contain only orphaned code, delete the entire files rather than surgically removing functions

### Safety Verification

- **Coverage baseline:** Run `go test -cover` before deletion to capture coverage percentages on key files (graph.go, types.go, db.go). Run again after deletion to verify no regression
- **Build verification:** `go build ./...` must succeed after all deletions
- **Test verification:** `go test ./...` must pass with no failures
- **Vet verification:** `go vet ./...` must pass (catches unused variables, unreachable code)
- **Caller verification:** Grep/search for all deleted function and type names to confirm zero references remain

### Commit Strategy

- **One atomic commit** for the entire deletion. Clean git history, easy to revert if needed
- Commit message should list what was removed

### Claude's Discretion

- Exact determination of which functions/types are shared vs orphaned (requires codebase analysis)
- Whether to keep query.go as a file with shared helpers or delete it entirely
- Any cleanup of orphaned imports after deletion

</decisions>

<specifics>
## Specific Ideas

- Research confirmed: ExecuteImpact, ExecuteCrawl, QueryResult, AffectedNode, QueryEdge have zero callers from CLI code
- The CLI uses its own types: ImpactResult, ImpactNode, EnrichedRelationship in query_cli.go
- Pitfalls research warns: query_test.go may implicitly test shared helpers (NewGraph, AddNode, AddEdge) — coverage check catches this

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 06-dead-code-removal*
*Context gathered: 2026-03-29*
