# Roadmap: graphmd

## Milestones

- ✅ **v1.0 graphmd MVP** — Phases 1-5 (shipped 2026-03-28)
- 🚧 **v1.1 Tech Debt Cleanup** — Phases 6-8 (in progress)

## Phases

<details>
<summary>✅ v1.0 graphmd MVP (Phases 1-5) — SHIPPED 2026-03-28</summary>

- [x] Phase 1: Component Model (3 plans) — 12-type taxonomy, schema persistence, type queries
- [x] Phase 2: Accuracy Foundation (5 plans) — confidence tiers, provenance, cycle safety, pageindex
- [x] Phase 3: Extract & Export Pipeline (2 plans) — .graphmdignore, aliases, ZIP export
- [x] Phase 4: Import & Query Pipeline (3 plans) — XDG import, 4 query patterns, JSON envelope
- [x] Phase 5: Crawl Exploration (2 plans) — stats engine, text/JSON formatters, quality warnings

**18 requirements delivered. Full details: `.planning/milestones/v1.0-ROADMAP.md`**

</details>

### v1.1 Tech Debt Cleanup

**Milestone Goal:** Resolve all tech debt from v1.0 audit — remove orphaned code, surface hidden information, warn on silent data loss.

- [ ] **Phase 6: Dead Code Removal** - Remove orphaned query execution types and functions from v1.0 Phase 2
- [ ] **Phase 7: Silent Loss Reporting** - Surface cycle detection in queries and warn on dropped edges during export
- [ ] **Phase 8: Provenance Access** - Expose component_mentions data in query results via opt-in flag

## Phase Details

### Phase 6: Dead Code Removal
**Goal**: The query layer has one unambiguous type hierarchy with no orphaned parallel implementations
**Depends on**: Phase 5 (v1.0 complete)
**Requirements**: DEBT-01
**Success Criteria** (what must be TRUE):
  1. `go build ./...` succeeds with no compilation errors after deletion
  2. `go test ./...` passes with no coverage regression on graph.go or types.go
  3. ExecuteImpact, ExecuteCrawl, QueryResult, AffectedNode, and QueryEdge are not present anywhere in the codebase
  4. No orphaned test file (query_test.go) remains in internal/knowledge
**Plans**: 1 plan

Plans:
- [ ] 06-01-PLAN.md — Remove orphaned query execution types and functions

### Phase 7: Silent Loss Reporting
**Goal**: Operators and AI agents are informed when data is silently dropped during export or traversal
**Depends on**: Phase 6
**Requirements**: DEBT-02, DEBT-03
**Success Criteria** (what must be TRUE):
  1. Running `graphmd query impact <component>` on a graph with cycles includes a `cycles_detected` field in the JSON envelope metadata listing the cycle paths encountered
  2. Running `graphmd query dependencies <component>` on a graph with cycles includes cycle information in the same way
  3. Running `graphmd export` on data with edges referencing non-existent components prints a warning to stderr with the count of dropped edges
  4. The JSON envelope schema remains backward-compatible (all new fields use omitempty, no existing fields renamed or removed)
**Plans**: 2 plans

Plans:
- [ ] 07-01-PLAN.md — Cycle back-edge detection in BFS traversal with JSON envelope metadata
- [ ] 07-02-PLAN.md — Dropped edge stderr warnings in SaveGraph

### Phase 8: Provenance Access
**Goal**: AI agents can retrieve component detection provenance (source files, detection methods, confidence) alongside query results
**Depends on**: Phase 7
**Requirements**: DEBT-04
**Success Criteria** (what must be TRUE):
  1. Running `graphmd query impact <component> --include-provenance` returns mention data (file path, detection method, confidence) for each affected component in the JSON output
  2. Running `graphmd query impact <component>` without the flag produces identical output to current behavior (no mention data, no schema change)
  3. `graphmd query dependencies <component> --include-provenance` works the same way
  4. Mention data is limited per component (not unbounded) to prevent output bloat
**Plans**: TBD

Plans:
- [ ] 08-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 6 → 7 → 8

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Component Model | v1.0 | 3 | Complete | 2026-03-16 |
| 2. Accuracy Foundation | v1.0 | 5 | Complete | 2026-03-19 |
| 3. Extract & Export | v1.0 | 2 | Complete | 2026-03-19 |
| 4. Import & Query | v1.0 | 3 | Complete | 2026-03-23 |
| 5. Crawl Exploration | v1.0 | 2 | Complete | 2026-03-24 |
| 6. Dead Code Removal | v1.1 | 0/1 | Not started | - |
| 7. Silent Loss Reporting | v1.1 | 0/2 | Not started | - |
| 8. Provenance Access | v1.1 | 0/1 | Not started | - |

---
*Created: 2026-03-16*
*Last updated: 2026-03-29 (v1.1 roadmap created)*
