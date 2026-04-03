---
phase: 08-provenance-access
plan: 01
subsystem: api
tags: [sqlite, provenance, cli, json, mentions]

# Dependency graph
requires:
  - phase: 02-graph-export
    provides: component_mentions table schema and SaveComponentMentions method
provides:
  - LoadComponentMentions DB method returning map[string][]ComponentMention
  - MentionDetail struct for JSON output
  - --include-provenance and --max-mentions flags on impact/dependencies queries
  - decorateWithMentions post-BFS mention decoration
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [post-BFS decoration pattern for optional data enrichment, non-fatal DB access for optional features]

key-files:
  created: []
  modified:
    - internal/knowledge/db.go
    - internal/knowledge/db_test.go
    - internal/knowledge/query_cli.go
    - internal/knowledge/query_cli_test.go

key-decisions:
  - "MentionDetail maps detected_by column to detection_method JSON field for clarity"
  - "loadMentionsForGraph is non-fatal: warns to stderr on error and continues without mentions"
  - "Default mention limit is 5 per component; --max-mentions 0 means unlimited"

patterns-established:
  - "Post-BFS decoration: optional data enrichment applied after traversal, not during"
  - "Non-fatal DB access: secondary data loading warns to stderr on failure rather than aborting"

requirements-completed: [DEBT-04]

# Metrics
duration: 4min
completed: 2026-03-29
---

# Phase 8 Plan 1: Provenance Access Summary

**LoadComponentMentions DB method with --include-provenance and --max-mentions flags surfacing detection provenance in impact/dependencies query JSON output**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T16:13:45Z
- **Completed:** 2026-03-29T16:17:49Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- LoadComponentMentions reads all mentions from DB grouped by component_id, sorted by confidence DESC
- --include-provenance flag adds mentions array with file_path, detection_method, confidence, context per affected node
- --max-mentions flag controls volume (default 5, 0 = unlimited) with mention_count showing total before truncation
- Without --include-provenance, output is byte-identical to previous behavior (omitempty suppression)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add LoadComponentMentions database method with tests** - `deeb50b` (feat)
2. **Task 2: Add --include-provenance and --max-mentions flags with mention decoration** - `c0ca31b` (feat)

_Note: TDD tasks have RED+GREEN combined in single commits for efficiency_

## Files Created/Modified
- `internal/knowledge/db.go` - Added LoadComponentMentions method
- `internal/knowledge/db_test.go` - Tests for LoadComponentMentions (populated and empty table)
- `internal/knowledge/query_cli.go` - MentionDetail struct, decorateWithMentions, loadMentionsForGraph, flags on impact/dependencies
- `internal/knowledge/query_cli_test.go` - Provenance tests: with/without flag, impact/dependencies, max-mentions truncation/unlimited

## Decisions Made
- MentionDetail maps DB column `detected_by` to JSON field `detection_method` for user clarity
- loadMentionsForGraph is non-fatal: warns to stderr on error, continues without mentions
- Default mention limit is 5 per component; --max-mentions 0 returns all mentions
- MentionCount uses int with omitempty so zero value suppresses field when provenance not requested

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- DEBT-04 fully satisfied: component_mentions data accessible in query results via --include-provenance
- Phase 8 complete (single plan phase)
- All v1.1 tech debt cleanup phases (6, 7, 8) are now complete

---
*Phase: 08-provenance-access*
*Completed: 2026-03-29*
