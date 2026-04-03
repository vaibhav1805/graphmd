---
phase: 07-silent-loss-reporting
plan: 02
subsystem: database
tags: [sqlite, stderr, warnings, data-quality]

# Dependency graph
requires:
  - phase: 07-silent-loss-reporting
    provides: "SaveGraph edge-skipping logic to instrument"
provides:
  - "Stderr warnings when SaveGraph drops edges with missing endpoint nodes"
  - "stderrWriter var for test-injectable stderr output"
affects: [08-provenance-access]

# Tech tracking
tech-stack:
  added: []
  patterns: ["injectable io.Writer for stderr capture in tests"]

key-files:
  created:
    - internal/knowledge/db_test.go
  modified:
    - internal/knowledge/db.go

key-decisions:
  - "Used package-level stderrWriter var instead of os.Pipe for stderr capture — simpler, thread-safer for tests"
  - "Warnings print after successful transaction only — no noise on rollback"
  - "SaveGraph signature unchanged — zero caller impact"

patterns-established:
  - "stderrWriter pattern: package-level io.Writer defaulting to os.Stderr, overridable in tests"

requirements-completed: [DEBT-03]

# Metrics
duration: 4min
completed: 2026-03-29
---

# Phase 7 Plan 2: Dropped Edge Warnings Summary

**SaveGraph now prints stderr warnings with count and pair details when edges are dropped due to missing endpoint nodes**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T07:39:21Z
- **Completed:** 2026-03-29T07:43:12Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- SaveGraph tracks dropped edges (missing source or target) during transaction and prints stderr warnings after successful commit
- 4 test cases covering: dropped edge warning with count/pairs, no warning on clean graph, missing source, missing target
- Zero changes to SaveGraph return signature or any caller code

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for dropped edge warnings** - `d7de591` (test)
2. **Task 1 GREEN: Implement dropped edge tracking and stderr warnings** - `b3b69bd` (feat)

## Files Created/Modified
- `internal/knowledge/db.go` - Added stderrWriter var, dropped edge tracking in SaveGraph transaction, stderr warning output after commit
- `internal/knowledge/db_test.go` - 4 test functions covering warning behavior

## Decisions Made
- Used package-level `var stderrWriter io.Writer = os.Stderr` for test capture instead of `os.Pipe()` -- simpler and avoids file descriptor management
- Warning prints after transaction returns nil (not inside closure) so dropped edges are only reported on successful saves
- Test cleanup restores `stderrWriter = os.Stderr` (not nil) to avoid nil pointer dereference in concurrent tests

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Pre-existing test failures in `TestQueryImpact_AcyclicGraph_NoCycles` and `TestQueryImpact_CyclicGraph_CyclesDetectedAbsentInJSON` -- unrelated to this plan's changes, confirmed by running tests on baseline code

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 7 (Silent Loss Reporting) complete with both plans done
- Ready for Phase 8 (Provenance Access)

---
*Phase: 07-silent-loss-reporting*
*Completed: 2026-03-29*
