---
phase: 04-import-query-pipeline
plan: 01
subsystem: cli
tags: [zip, xdg, import, storage, validation]

# Dependency graph
requires:
  - phase: 03-extract-export
    provides: ZIP export format (graph.db + metadata.json)
provides:
  - ZIP import pipeline with validation and schema version checks
  - XDG-based persistent graph storage with named graphs
  - Current graph tracking for query command defaults
  - LoadStoredGraph API for query commands
affects: [04-import-query-pipeline]

# Tech tracking
tech-stack:
  added: [archive/zip]
  patterns: [XDG data directory, named graph storage, current marker file]

key-files:
  created:
    - internal/knowledge/xdg.go
    - internal/knowledge/import.go
    - internal/knowledge/import_test.go
  modified:
    - cmd/graphmd/main.go

key-decisions:
  - "XDG_DATA_HOME with ~/.local/share fallback for graph storage"
  - "Current graph tracked via plain text marker file"
  - "Full DB validation on import (OpenDB + LoadGraph) before committing to storage"

patterns-established:
  - "Named graph storage: $XDG_DATA_HOME/graphmd/graphs/<name>/{graph.db,metadata.json}"
  - "CmdImport pattern: flag parsing -> ImportZIP -> success message to stderr"

requirements-completed: [IMPORT-01]

# Metrics
duration: 3min
completed: 2026-03-23
---

# Phase 4 Plan 1: Import Pipeline Summary

**ZIP import pipeline with XDG storage, schema validation, named graphs, and current-graph tracking**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-23T17:09:34Z
- **Completed:** 2026-03-23T17:13:02Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- ImportZIP extracts ZIP, validates structure/schema/DB integrity, stores under XDG
- Named graphs via --name flag with filename-derived default
- LoadStoredGraph resolves named or current graph for downstream query commands
- 8 comprehensive tests covering valid import, naming, schema rejection, overwrite, and LoadStoredGraph

## Task Commits

Each task was committed atomically:

1. **Task 1: XDG storage and ZIP import pipeline** - `090ec06` (feat)
2. **Task 2: Import tests and CLI wiring** - `c2283aa` (test)

## Files Created/Modified
- `internal/knowledge/xdg.go` - XDG data directory resolution with fallback
- `internal/knowledge/import.go` - CmdImport, ImportZIP, LoadStoredGraph, copyFile
- `internal/knowledge/import_test.go` - 8 tests for import and stored graph loading
- `cmd/graphmd/main.go` - case "import" wiring, cmdImport function, updated usage

## Decisions Made
- XDG_DATA_HOME with ~/.local/share fallback for cross-platform graph storage
- Current graph tracked via plain text "current" marker file in storage dir
- Full DB validation on import (OpenDB + LoadGraph on temp copy) before committing files
- io.Copy used for streaming file extraction (not ReadAll) for large databases

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Edge ID required in test helper**
- **Found during:** Task 2 (import tests)
- **Issue:** Test helper created edge without ID; AddEdge requires non-empty ID
- **Fix:** Added explicit edge ID "api-gateway->auth-service" in createTestZIP helper
- **Files modified:** internal/knowledge/import_test.go
- **Verification:** TestLoadStoredGraph_NamedGraph passes with correct edge count
- **Committed in:** c2283aa (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor test data fix. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Import pipeline complete; LoadStoredGraph ready for query commands (Plan 2)
- Current graph tracking enables `graphmd query` without explicit --graph flag

## Self-Check: PASSED

All files found. All commits verified.

---
*Phase: 04-import-query-pipeline*
*Completed: 2026-03-23*
