---
phase: 07-silent-loss-reporting
plan: 01
subsystem: query
tags: [cycle-detection, bfs, json-envelope, graph-traversal]

# Dependency graph
requires:
  - phase: 02-query-engine
    provides: "BFS traversal functions and QueryEnvelopeMetadata struct"
provides:
  - "CycleEntry struct for reporting graph cycles in JSON"
  - "CyclesDetected field on QueryEnvelopeMetadata with omitempty"
  - "detectRelevantCycles helper using DFS-based graph cycle detection"
affects: [07-02-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns: ["DFS cycle detection filtered by BFS traversal scope"]

key-files:
  created: []
  modified:
    - "internal/knowledge/query_cli.go"
    - "internal/knowledge/query_cli_test.go"

key-decisions:
  - "Used graph's existing DFS DetectCycles() instead of BFS back-edge detection to avoid false positives from DAG diamond patterns"
  - "Root node included in cycle entries when genuinely part of a cycle; only excluded when falsely detected"

patterns-established:
  - "detectRelevantCycles: shared helper filtering DFS cycles to BFS-traversed nodes"

requirements-completed: [DEBT-02]

# Metrics
duration: 8min
completed: 2026-03-29
---

# Phase 7 Plan 01: Cycle Detection Summary

**BFS traversal cycle detection using DFS back-edge analysis, surfaced via cycles_detected field in JSON query envelope metadata**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-29T07:39:06Z
- **Completed:** 2026-03-29T07:47:01Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 2

## Accomplishments
- CycleEntry struct with From/To JSON fields for reporting graph cycles
- CyclesDetected field on QueryEnvelopeMetadata with omitempty (absent from JSON on acyclic graphs)
- Both executeImpactReverse and executeForwardTraversal return cycle information
- detectRelevantCycles helper filters DFS-detected cycles to only those involving BFS-traversed nodes
- Comprehensive test suite with cyclic graph fixture, acyclic validation, BFS distance correctness checks

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add failing cycle detection tests** - `f4a4a57` (test)
2. **Task 1 (GREEN): Implement cycle detection** - `4f51903` (feat)

## Files Created/Modified
- `internal/knowledge/query_cli.go` - Added CycleEntry struct, CyclesDetected metadata field, detectRelevantCycles helper, updated BFS functions
- `internal/knowledge/query_cli_test.go` - Added setupCyclicQueryTestGraph, 6 new cycle detection tests

## Decisions Made
- Used the graph's existing DFS-based DetectCycles() method rather than inline BFS back-edge detection, because BFS back-edge detection produces false positives on diamond-shaped DAGs (multiple paths to same node misidentified as cycles)
- Root node is included in cycle entries when genuinely part of a cycle; the "root not falsely reported" requirement is satisfied by using DFS which has no false positives

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Changed from BFS back-edge detection to DFS-based cycle detection**
- **Found during:** Task 1 (GREEN phase)
- **Issue:** BFS back-edge detection (checking if neighbor is already visited) produces false positives on DAGs with diamond patterns (e.g., two paths converging on the same node)
- **Fix:** Used graph's existing DetectCycles() DFS method and filter results to nodes reached by BFS traversal
- **Files modified:** internal/knowledge/query_cli.go
- **Verification:** Acyclic graph tests pass with zero false cycle reports
- **Committed in:** 4f51903

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for correctness -- BFS-only approach would report false cycles on any DAG with converging paths.

## Issues Encountered
None beyond the deviation documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Cycle detection is live for impact and dependencies queries
- Ready for plan 07-02 (remaining silent loss reporting work)

---
*Phase: 07-silent-loss-reporting*
*Completed: 2026-03-29*
