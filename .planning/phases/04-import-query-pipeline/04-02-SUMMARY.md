---
phase: 04-import-query-pipeline
plan: 02
subsystem: cli
tags: [query, impact, dependencies, path, json, confidence-tiers, fuzzy-match]

# Dependency graph
requires:
  - phase: 04-import-query-pipeline
    provides: ZIP import pipeline, LoadStoredGraph API, XDG storage
  - phase: 02-accuracy-foundation
    provides: Confidence tiers, graph traversal, query result types
provides:
  - Four query subcommands: impact, dependencies, path, list
  - Consistent JSON envelope with confidence tier enrichment
  - Reverse BFS traversal for impact analysis
  - Fuzzy component matching with suggestions
  - Table output format for human readability
affects: [05-crawl-exploration]

# Tech tracking
tech-stack:
  added: [text/tabwriter]
  patterns: [reverse BFS for impact, forward BFS for dependencies, JSON error envelope, fuzzy matching]

key-files:
  created:
    - internal/knowledge/query_cli.go
    - internal/knowledge/query_cli_test.go
  modified:
    - cmd/graphmd/main.go

key-decisions:
  - "Impact uses reverse BFS (ByTarget) not forward traversal, correctly answering 'what breaks if X fails?'"
  - "Dependencies uses forward BFS (BySource) for 'what does X need to work?'"
  - "No-path-found returns exit 0 with reason field (not an error condition)"
  - "Word-level overlap scoring in fuzzy matching for better suggestions"

patterns-established:
  - "CmdQuery router pattern: subcommand dispatch with per-subcommand flag.NewFlagSet"
  - "QueryEnvelope: {query, results, metadata} consistent JSON contract for all query types"
  - "EnrichedRelationship: every relationship includes both confidence (float) and confidence_tier (string)"

requirements-completed: [IMPORT-02, IMPORT-03]

# Metrics
duration: 5min
completed: 2026-03-23
---

# Phase 4 Plan 2: Query CLI Summary

**Four query subcommands (impact/dependencies/path/list) with reverse BFS for impact analysis, JSON envelope with confidence tiers, and fuzzy component matching**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-23T17:15:58Z
- **Completed:** 2026-03-23T17:20:52Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Impact subcommand uses reverse BFS (ByTarget) to correctly answer "if X fails, what breaks?"
- Dependencies subcommand uses forward BFS (BySource) for "what does X need?"
- Path subcommand finds routes with per-hop confidence scoring and product-of-confidences ranking
- List subcommand supports composable --type and --min-confidence AND filters
- All output uses consistent JSON envelope: {query, results, metadata}
- Every relationship enriched with confidence_tier alongside numeric score
- 13 query-specific tests including critical impact-vs-dependencies direction proof

## Task Commits

Each task was committed atomically:

1. **Task 1: Query router, response envelope, and all four subcommands** - `4fed02d` (feat)
2. **Task 2: Query tests and CLI wiring** - `4fbc44e` (test)

## Files Created/Modified
- `internal/knowledge/query_cli.go` - CmdQuery router, 4 subcommands, envelope types, reverse BFS, fuzzy matching, table output
- `internal/knowledge/query_cli_test.go` - 13 tests with 5-component test graph covering all query patterns
- `cmd/graphmd/main.go` - case "query" wiring, cmdQueryMain function, updated usage with examples

## Decisions Made
- Impact uses reverse BFS following ByTarget (incoming edges) to find what depends on a component, not forward traversal
- Dependencies uses forward BFS following BySource (outgoing edges) to find what a component needs
- No-path-found returns success (exit 0) with empty paths array and reason field, not an error
- Word-level overlap scoring added to fuzzy matching for better suggestions (e.g., "nonexistent-service" suggests "auth-service")

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Enhanced fuzzy matching with word-level overlap**
- **Found during:** Task 2 (query tests)
- **Issue:** suggestComponents returned empty suggestions for "nonexistent-service" because simple substring matching failed
- **Fix:** Added word-level overlap scoring (split on hyphens/underscores, match parts) alongside substring and prefix matching
- **Files modified:** internal/knowledge/query_cli.go
- **Verification:** TestQueryError_UnknownComponent and TestSuggestComponents both pass
- **Committed in:** 4fbc44e (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor fuzzy matching improvement. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All Phase 4 plans complete; import and query pipeline fully operational
- Query interface ready for AI agents to use via `graphmd query impact/dependencies/path/list`
- Phase 5 crawl exploration can build on these query patterns

## Self-Check: PASSED

All files found. All commits verified.

---
*Phase: 04-import-query-pipeline*
*Completed: 2026-03-23*
