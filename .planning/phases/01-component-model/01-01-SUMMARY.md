---
phase: 01-component-model
plan: 01
subsystem: knowledge
tags: [component-types, sqlite, taxonomy, inference, cli]

requires:
  - phase: none
    provides: brownfield codebase with scanner, extractor, discovery algorithms
provides:
  - 12-type component taxonomy with constants and validation
  - Component type persistence in SQLite schema (component_type column, component_mentions table)
  - Type inference engine with pattern matching and confidence scoring
  - CLI list --type command for type-filtered queries
  - SeedConfig for user-extensible type mappings
affects: [02-accuracy-foundation, 03-extract-export, 05-crawl-exploration]

tech-stack:
  added: []
  patterns: [component-type-inference, provenance-tracking, schema-migration]

key-files:
  created:
    - internal/knowledge/types.go
    - internal/knowledge/types_test.go
    - internal/knowledge/component_types_test.go
  modified:
    - internal/knowledge/db.go
    - internal/knowledge/graph.go
    - internal/knowledge/components.go
    - internal/knowledge/registry.go
    - cmd/graphmd/main.go

key-decisions:
  - "12-type taxonomy based on Backstage/Cartography patterns covers common infrastructure components"
  - "InferComponentType uses longest-match pattern strategy with 3-tier confidence (0.95 exact, 0.85 name, 0.65 context)"
  - "SeedConfig uses glob patterns (redis* -> cache) with precedence over auto-detection (confidence 1.0)"
  - "ComponentTypeAPI and ComponentTypeConfig preserved as aliases for backward compatibility"
  - "SaveGraph skips edges referencing non-existent nodes to handle FK constraints from dangling references"

patterns-established:
  - "Component type inference: InferComponentType(name, context...) -> (type, confidence)"
  - "Schema migration: migrateV2ToV3() adds columns/tables idempotently"
  - "Provenance tracking: component_mentions table records detection method and file path"

requirements-completed: [COMP-01, COMP-02, COMP-03]

duration: 11min
completed: 2026-03-16
---

# Phase 1 Plan 1: Component Type System Definition & Persistence Summary

**12-type component taxonomy with pattern-based inference, SQLite persistence, and CLI type-filtered queries**

## Performance

- **Duration:** 11 min
- **Started:** 2026-03-16T05:41:00Z
- **Completed:** 2026-03-16T05:52:57Z
- **Tasks:** 5
- **Files modified:** 7

## Accomplishments
- Defined 12-type component taxonomy (service, database, cache, queue, message-broker, load-balancer, gateway, storage, container-registry, config-server, monitoring, log-aggregator) plus unknown default
- Extended SQLite schema to v3 with component_type column and component_mentions provenance table
- Wired component detection pipeline to classify and persist types during indexing
- Implemented `graphmd list --type` CLI command with strict and inclusive (--include-tags) modes
- Added comprehensive unit tests (type validation, inference, schema migration) and integration tests (detect -> persist -> query)

## Task Commits

Each task was committed atomically:

1. **Task 1.1: Define Component Type Constants & Taxonomy** - `2dd796f` (feat)
2. **Task 1.2: Extend SQLite Schema with Component Classification** - `d800aef` (feat)
3. **Task 1.3: Wire Component Detection to Type Persistence** - `f47380d` (feat)
4. **Task 1.4: Implement CLI list --type T Filter** - `8728091` (feat)
5. **Task 1.5: Add Unit & Integration Tests** - `51477f1` (test)

## Files Created/Modified
- `internal/knowledge/types.go` - Component type constants, validation, inference, seed config
- `internal/knowledge/types_test.go` - Unit tests for type system
- `internal/knowledge/component_types_test.go` - Schema, migration, detection, and integration tests
- `internal/knowledge/db.go` - Schema v3 migration, SaveComponentMentions, ListComponentsByType
- `internal/knowledge/graph.go` - ComponentType field on Node struct
- `internal/knowledge/components.go` - Type/TypeConfidence/DetectionMethods on Component struct
- `internal/knowledge/registry.go` - Migrated ComponentType to types.go, added aliases
- `cmd/graphmd/main.go` - list --type command, node creation during indexing

## Decisions Made
- Used longest-match pattern strategy for type inference to handle ambiguous names correctly
- Preserved ComponentTypeAPI/ComponentTypeConfig as aliases to avoid breaking existing registry data
- Added "store" to database patterns for backward compatibility with existing inferComponentType behavior
- Made SaveGraph skip dangling edge references instead of failing (pre-existing FK constraint issue)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed dangling edge FK constraint failure in SaveGraph**
- **Found during:** Task 1.3 (Wire Detection to Persistence)
- **Issue:** Existing extractor creates edges referencing non-existent nodes, causing FK constraint failures when saving graph
- **Fix:** SaveGraph now skips edges where source or target node is not in the graph
- **Files modified:** internal/knowledge/db.go
- **Verification:** indexing test-data succeeds; 62 docs indexed, 109 relationships saved
- **Committed in:** f47380d (Task 1.3 commit)

**2. [Rule 3 - Blocking] Index command was not adding document nodes to graph**
- **Found during:** Task 1.3 (Wire Detection to Persistence)
- **Issue:** cmdIndex only extracted edges without creating graph nodes, resulting in empty graphs
- **Fix:** Added node creation from scanned documents before edge extraction
- **Files modified:** cmd/graphmd/main.go
- **Verification:** Graph now contains 62 nodes after indexing test-data
- **Committed in:** f47380d (Task 1.3 commit)

**3. [Rule 3 - Blocking] Schema DDL index creation failed on pre-v3 databases**
- **Found during:** Task 1.3 (Wire Detection to Persistence)
- **Issue:** CREATE INDEX on component_type column failed when existing database didn't have the column yet (pre-migration)
- **Fix:** execMulti now tolerates "no such column" errors from index creation statements
- **Files modified:** internal/knowledge/db.go
- **Verification:** Opening existing v2 databases succeeds; migration adds column and index
- **Committed in:** f47380d (Task 1.3 commit)

---

**Total deviations:** 3 auto-fixed (3 blocking)
**Impact on plan:** All fixes were necessary for the pipeline to function. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Component type system is fully operational: types defined, persisted, and queryable
- Ready for Plan 2 (User-Facing Documentation) and Plan 3 (QA & Validation)
- 6 distinct component types detected from test corpus (service, database, gateway, monitoring, log-aggregator, unknown)
- Schema migration is backward compatible with existing databases

---
*Phase: 01-component-model*
*Completed: 2026-03-16*
