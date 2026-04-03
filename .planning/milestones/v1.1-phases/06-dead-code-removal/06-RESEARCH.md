# Phase 6: Dead Code Removal - Research

**Researched:** 2026-03-29
**Domain:** Go dead code removal — orphaned query execution layer
**Confidence:** HIGH

## Summary

Phase 6 is a mechanical deletion of orphaned query execution code from an earlier development cycle. The codebase contains two parallel query implementations: the live CLI path in `query_cli.go` (using `ImpactResult`, `ImpactNode`, `EnrichedRelationship`) and the dead path in `query.go` (using `QueryResult`, `AffectedNode`, `QueryEdge`, `ExecuteImpact`, `ExecuteCrawl`). The dead path has zero callers from any live code — all references are self-contained within `query.go`, `query_test.go`, and the type definitions in `types.go`/`types_test.go`.

Direct codebase inspection confirms that every function and type marked for deletion is exclusively referenced by other dead code. The helper functions `computeDistances`, `passesConfidenceFilter`, and `buildQueryResult` are only called within `query.go` itself. The CLI query layer has its own independent BFS traversal in `query_cli.go` that does not share any functions with the dead path.

**Primary recommendation:** Delete `query.go` and `query_test.go` entirely. Surgically remove `AffectedNode`, `QueryEdge`, `QueryResult` (plus `String()` and `Validate()` methods) from `types.go`, and their tests from `types_test.go`. Verify with `go build`, `go test -cover`, and `go vet`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Delete only truly orphaned code:** Functions and types with zero live callers get removed
- **Keep shared helpers:** If a function (e.g., computeDistances, passesConfidenceFilter) is used by any live code path, it stays in place
- **Full file deletion when possible:** If query.go and query_test.go contain only orphaned code, delete the entire files rather than surgically removing functions
- **Coverage baseline:** Run `go test -cover` before deletion to capture coverage percentages on key files (graph.go, types.go, db.go). Run again after deletion to verify no regression
- **Build verification:** `go build ./...` must succeed after all deletions
- **Test verification:** `go test ./...` must pass with no failures
- **Vet verification:** `go vet ./...` must pass (catches unused variables, unreachable code)
- **Caller verification:** Grep/search for all deleted function and type names to confirm zero references remain
- **One atomic commit** for the entire deletion. Clean git history, easy to revert if needed
- Commit message should list what was removed

### Claude's Discretion
- Exact determination of which functions/types are shared vs orphaned (requires codebase analysis)
- Whether to keep query.go as a file with shared helpers or delete it entirely
- Any cleanup of orphaned imports after deletion

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DEBT-01 | Remove orphaned ExecuteImpact, ExecuteCrawl, QueryResult, AffectedNode, QueryEdge functions and types, plus their tests | Full codebase analysis confirms all targets are orphaned with zero live callers. Deletion scope and safety verification fully mapped below. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24 | Language runtime | Already in use; no new dependencies needed |

### Supporting
No new libraries required. This phase uses only Go toolchain commands (`go build`, `go test`, `go vet`) and text editor operations.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual deletion + grep verification | `go install golang.org/x/tools/cmd/deadcode@latest` | Automated tool finds dead code, but manual approach is simpler for a known, small scope. Manual is preferred per CONTEXT.md. |

## Architecture Patterns

### Deletion Map

The orphaned code spans four files. Two files (`query.go`, `query_test.go`) are entirely dead and can be deleted wholesale. Two files (`types.go`, `types_test.go`) contain a mix of live and dead code requiring surgical removal.

```
internal/knowledge/
├── query.go          # DELETE ENTIRE FILE (224 lines)
│   ├── ImpactQuery struct (lines 11-17)
│   ├── CrawlQuery struct (lines 21-24)
│   ├── ExecuteImpact func (lines 30-86)
│   ├── ExecuteCrawl func (lines 92-113)
│   ├── buildQueryResult func (lines 116-179)
│   ├── computeDistances func (lines 183-208)
│   └── passesConfidenceFilter func (lines 212-224)
│
├── query_test.go     # DELETE ENTIRE FILE (308 lines)
│   ├── buildTestGraph helper (lines 9-31)
│   ├── TestImpactQuery_DirectOnly (lines 33-60)
│   ├── TestImpactQuery_DepthTwo (lines 62-81)
│   ├── TestImpactQuery_ConfidenceFilter (lines 83-106)
│   ├── TestImpactQuery_TierFilter (lines 108-128)
│   ├── TestImpactQuery_TraverseModeDirect (lines 130-149)
│   ├── TestImpactQuery_RootNotFound (lines 151-158)
│   ├── TestCrawlQuery_NoFilter (lines 160-177)
│   ├── TestCrawlQuery_MaxDepth (lines 179-191)
│   ├── TestCrawlQuery_RootNotFound (lines 193-200)
│   ├── TestQueryResult_JSONSchema (lines 202-251)
│   ├── TestQueryResult_ConfidenceFilterComparison (lines 253-273)
│   └── TestQueryResult_CycleHandling (lines 275-308)
│
├── types.go          # SURGICAL: remove lines 371-450 (~80 lines)
│   ├── AffectedNode struct (lines 374-380)
│   ├── QueryEdge struct (lines 384-395)
│   ├── QueryResult struct (lines 400-410)
│   ├── QueryResult.String() method (lines 414-420)
│   └── QueryResult.Validate() method (lines 426-450)
│
└── types_test.go     # SURGICAL: remove lines 171-254 (~84 lines)
    ├── TestQueryResult_JSONMarshal (lines 171-211)
    └── TestQueryResult_Validation (lines 213-254)
```

**Total deletion: ~696 lines** (224 + 308 + 80 + 84)

### Caller Verification Results

Every function and type marked for deletion was grep-verified against the entire codebase:

| Symbol | Files Referencing | All Dead? |
|--------|-------------------|-----------|
| `ExecuteImpact` | `query.go`, `query_test.go` | YES — only self-references and dead tests |
| `ExecuteCrawl` | `query.go`, `query_test.go` | YES — only self-references and dead tests |
| `ImpactQuery` | `query.go`, `query_test.go` | YES — struct only used by ExecuteImpact |
| `CrawlQuery` | `query.go`, `query_test.go` | YES — struct only used by ExecuteCrawl |
| `QueryResult` | `query.go`, `query_test.go`, `types.go`, `types_test.go` | YES — no CLI or import/export callers |
| `AffectedNode` | `query.go`, `types.go`, `types_test.go` | YES — CLI uses `ImpactNode` instead |
| `QueryEdge` | `query.go`, `types.go`, `types_test.go` | YES — CLI uses `EnrichedRelationship` instead |
| `buildQueryResult` | `query.go` | YES — only called within query.go |
| `computeDistances` | `query.go` | YES — only called within query.go |
| `passesConfidenceFilter` | `query.go` | YES — only called within query.go |
| `buildTestGraph` | `query_test.go` | YES — only used by dead tests |

**Note:** `context.go` contains `pageIndexQueryResult` — this is a completely different type (unexported, local to context indexing) and is NOT related to the orphaned `QueryResult`.

**Note:** `query_cli_test.go` references `result.AffectedNodes` but this is the `ImpactResult.AffectedNodes` field (type `[]ImpactNode`), not the orphaned `AffectedNode` type.

### Anti-Patterns to Avoid
- **Partial file deletion leaving dead imports:** After removing types from `types.go`, check if the `"encoding/json"` and `"fmt"` imports are still needed by remaining code. `go vet` will catch this, but be aware.
- **Deleting shared graph infrastructure:** `NewGraph`, `AddNode`, `AddEdge`, `NewEdge`, `TraverseDFS`, `ScoreToTier`, `TierAtLeast` are all used by `query_test.go` BUT are also used extensively by live code. These stay. The test file is the only dead caller — the functions themselves are live.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Dead code detection | Custom analysis script | `go build ./...` + `go vet ./...` | Go compiler is authoritative for internal packages; if it compiles and vets clean, no orphaned references remain |
| Import cleanup | Manual import editing | `goimports` or Go toolchain auto-fix | Automatically removes unused imports after type deletion |

**Key insight:** For an internal Go package with no external consumers, the compiler is the definitive dead code verifier. If `go build ./...` succeeds after deletion, no live code referenced the deleted symbols.

## Common Pitfalls

### Pitfall 1: Accidentally Removing Shared Test Helpers
**What goes wrong:** `query_test.go` contains `buildTestGraph()` which creates a graph using `NewGraph`, `AddNode`, `AddEdge` — functions that are live infrastructure. Deleting the test file is safe because it only removes a *caller* of those functions, not the functions themselves.
**Why it happens:** Confusing "this test exercises live code" with "this test IS live code."
**How to avoid:** Verify that the functions being tested (`ExecuteImpact`, `ExecuteCrawl`) are dead. The test helper that constructs test data is an internal detail of the dead tests.
**Warning signs:** Coverage regression on `graph.go` after deletion — run `go test -cover` before and after.

### Pitfall 2: Import Statement Residue
**What goes wrong:** After removing `QueryResult.String()` and `QueryResult.Validate()` from `types.go`, the `"encoding/json"` or `"fmt"` imports may become unused, causing build failure.
**Why it happens:** Go has strict unused import rules.
**How to avoid:** After surgical removal, run `go build ./...` immediately. Fix any unused import errors. Or use `goimports` to auto-clean.
**Warning signs:** Build error mentioning "imported and not used."

### Pitfall 3: Coverage Number Misinterpretation
**What goes wrong:** After deleting dead code and dead tests, the coverage percentage may change in either direction. Deleting dead tests removes coverage of dead functions — this is expected and correct, not a regression.
**Why it happens:** Coverage percentages are relative to total statements. Removing both code and tests changes the denominator.
**How to avoid:** Focus on coverage of *specific live files* (`graph.go`, `db.go`, `query_cli.go`), not the overall percentage. If coverage for those files stays the same or increases, the deletion is safe.
**Warning signs:** Coverage for `graph.go` or `types.go` drops — this would mean a deleted test was the only test covering a live function.

## Code Examples

### File Deletion (query.go, query_test.go)
```bash
# Full file deletion — no surgical editing needed
rm internal/knowledge/query.go
rm internal/knowledge/query_test.go
```

### Surgical Removal from types.go
Remove the `AffectedNode`, `QueryEdge`, and `QueryResult` type definitions plus the `String()` and `Validate()` methods (approximately lines 371-450). The `TraversalState` type and everything above it are live code and must stay.

### Surgical Removal from types_test.go
Remove `TestQueryResult_JSONMarshal` and `TestQueryResult_Validation` (approximately lines 171-254). The `TestRelationshipLocationKey_Deterministic` test and everything below it are live code and must stay.

### Post-Deletion Verification
```bash
# Step 1: Capture baseline coverage before deletion
go test -cover ./internal/knowledge/ 2>&1

# Step 2: Perform deletions (files + surgical edits)

# Step 3: Verify build
go build ./...

# Step 4: Verify tests pass
go test ./internal/knowledge/

# Step 5: Verify no lint issues
go vet ./...

# Step 6: Verify no orphaned references
grep -rn "ExecuteImpact\|ExecuteCrawl\|ImpactQuery\|CrawlQuery\|QueryResult\|AffectedNode\|QueryEdge\|buildQueryResult\|computeDistances\|passesConfidenceFilter" internal/knowledge/*.go

# Step 7: Verify coverage on live files hasn't regressed
go test -cover ./internal/knowledge/ 2>&1
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `query.go` query execution (Phase 2) | `query_cli.go` CLI-integrated queries (Phase 4) | During v1.0 development | Phase 4 superseded Phase 2's in-memory query path with a full CLI implementation. Phase 2 code was never removed. |

**Deprecated/outdated:**
- `QueryResult`, `AffectedNode`, `QueryEdge`: Replaced by `QueryEnvelope`, `ImpactResult`, `ImpactNode`, `EnrichedRelationship` in `query_cli.go`
- `ExecuteImpact`, `ExecuteCrawl`: Replaced by `executeImpactReverse`, `executeForwardTraversal` (unexported functions in `query_cli.go`)
- `ImpactQuery`, `CrawlQuery`: Replaced by CLI flag parsing in `query_cli.go` command handlers

## Open Questions

None. All questions about shared vs. orphaned code were resolved by direct codebase grep analysis. The deletion scope is fully determined.

## Baseline Measurements

Current test coverage: **52.7%** overall for `internal/knowledge/`. This number will change after deletion (dead code and dead tests removed from both numerator and denominator). The meaningful metric is that coverage for live files (`graph.go`, `db.go`, `query_cli.go`, `types.go`) must not decrease.

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection: `internal/knowledge/query.go` — all 7 functions confirmed dead via grep across entire repository
- Direct codebase inspection: `internal/knowledge/query_test.go` — all 13 test functions call only dead functions
- Direct codebase inspection: `internal/knowledge/types.go` lines 371-450 — `AffectedNode`, `QueryEdge`, `QueryResult` referenced only by dead code
- Direct codebase inspection: `internal/knowledge/types_test.go` lines 171-254 — tests reference only orphaned types
- Direct codebase inspection: `internal/knowledge/query_cli.go` — live CLI types confirmed independent of orphaned types
- `go test -cover ./internal/knowledge/` — baseline coverage: 52.7%

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no libraries needed; Go toolchain only
- Architecture: HIGH — deletion map produced from direct grep and file reading of every affected symbol
- Pitfalls: HIGH — pitfalls derived from actual code structure, not hypothetical scenarios

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable — dead code doesn't change)
