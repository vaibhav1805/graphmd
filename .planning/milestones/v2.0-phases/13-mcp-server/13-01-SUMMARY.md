---
phase: 13-mcp-server
plan: 01
subsystem: api
tags: [query, refactor, mcp, go, exported-functions]

# Dependency graph
requires:
  - phase: 02-query-json
    provides: "Query CLI handlers and envelope types"
  - phase: 12-signal-integration
    provides: "Source-type filter and hybrid graph enrichment"
provides:
  - "5 exported query execution functions (ExecuteImpactQuery, ExecuteDependenciesQuery, ExecutePathQuery, ExecuteListQuery, GetGraphInfo)"
  - "QueryError type for structured error classification"
  - "Typed param structs for all query operations"
affects: [13-mcp-server]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Execute* function pattern: typed params in, envelope out, no stdout"]

key-files:
  created:
    - internal/knowledge/query_exec.go
    - internal/knowledge/query_exec_test.go
  modified:
    - internal/knowledge/query_cli.go

key-decisions:
  - "QueryError type with Code field for MCP layer to distinguish user errors from infrastructure errors"
  - "MaxMentions=0 means unlimited (passed through directly, not defaulted)"
  - "CLI parseDepth stays in CLI layer since it handles string-to-int conversion for 'all' keyword"

patterns-established:
  - "Execute* pattern: typed param struct in, (*QueryEnvelope, error) out, no side effects"
  - "handleQueryError bridges QueryError to CLI JSON output"

requirements-completed: [MCP-01]

# Metrics
duration: 5min
completed: 2026-04-03
---

# Phase 13 Plan 01: Query Execution API Summary

**Exported query execution functions with typed params and QueryError type for MCP server integration**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-03T00:55:53Z
- **Completed:** 2026-04-03T01:00:25Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created 5 exported query functions (impact, dependencies, path, list, graph_info) that accept typed param structs and return structured results without stdout side effects
- Introduced QueryError type enabling MCP layer to programmatically classify user errors (MISSING_ARG, NOT_FOUND, INVALID_ARG) vs infrastructure errors
- Refactored all 4 CLI handlers to delegate to exported functions while preserving identical output behavior

## Task Commits

Each task was committed atomically:

1. **Task 1: Create exported query execution functions** - `55f275e` (feat)
2. **Task 2: Refactor CLI handlers to delegate to exported functions** - `cc28422` (refactor)

## Files Created/Modified
- `internal/knowledge/query_exec.go` - 5 exported Execute* functions, QueryError type, typed param structs
- `internal/knowledge/query_exec_test.go` - Tests for validation, error handling, and QueryError interface
- `internal/knowledge/query_cli.go` - Refactored CLI handlers to delegate; added handleQueryError bridge

## Decisions Made
- QueryError type uses Code string field (not enum) for extensibility -- MCP can type-assert to *QueryError and switch on Code
- MaxMentions value of 0 passed through as-is (means unlimited) rather than defaulting to 5 in the Execute layer -- the CLI flag default of 5 handles the common case
- parseDepth remains in CLI layer since it converts the string flag value "all" to int 100 -- Execute* functions accept the already-parsed int

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed MaxMentions default overriding unlimited semantics**
- **Found during:** Task 2 (CLI refactor verification)
- **Issue:** ExecuteImpactQuery/ExecuteDependenciesQuery defaulted MaxMentions=0 to 5, breaking the CLI contract where --max-mentions=0 means unlimited
- **Fix:** Removed the default; pass MaxMentions through directly from params
- **Files modified:** internal/knowledge/query_exec.go
- **Verification:** TestQueryImpact_MaxMentionsZero passes
- **Committed in:** cc28422 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for backward compatibility. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 5 exported query functions are callable from external packages (e.g., internal/mcp/)
- QueryError type enables the MCP server to return structured error responses
- Ready for Plan 02 (MCP server implementation) to import and use these functions

---
*Phase: 13-mcp-server*
*Completed: 2026-04-03*
