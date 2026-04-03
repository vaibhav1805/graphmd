---
phase: 02-accuracy-foundation
plan: 01
subsystem: relationships
tags: [pageindex, deduplication, aggregation, weighted-average, location-tracking]

requires:
  - phase: 01-component-model
    provides: "Component types, DiscoverySignal struct, Edge struct"
provides:
  - "RelationshipLocation struct for file:line tracking"
  - "AggregateSignalsByLocation() with weighted averaging"
  - "AlgorithmWeight map for discovery algorithm prioritization"
  - "Location field on DiscoverySignal and AggregatedEdge"
affects: [02-confidence-tier-system, 02-edge-provenance-schema, 03-extract-export-pipeline]

tech-stack:
  added: []
  patterns: ["location-based deduplication via file:line keys", "weighted averaging for multi-algorithm confidence aggregation"]

key-files:
  created: []
  modified:
    - internal/knowledge/types.go
    - internal/knowledge/algo_aggregator.go
    - internal/knowledge/algo_aggregator_test.go
    - internal/knowledge/types_test.go

key-decisions:
  - "Weighted average formula: sum(confidence_i * weight_i) / sum(weight_i) per algorithm"
  - "Algorithm weights: cooccurrence=0.3, ner=0.5, structural=0.6, semantic=0.7, llm=1.0"
  - "Unknown algorithms default to weight 0.5"
  - "Existing AggregateSignals() preserved for backward compatibility"

patterns-established:
  - "Location dedup key format: file:line (e.g. docs/service.yaml:42)"
  - "AggregateSignalsByLocation groups by (source, target, location) triple"
  - "RelationshipLocation.IsValid() enforces relative paths and non-negative lines"

requirements-completed: [REL-05]

duration: 15min
completed: 2026-03-19
---

# Phase 2 Plan 1: Pageindex Integration & Deduplication Summary

**RelationshipLocation struct with file:line dedup keys and weighted-average signal aggregation across discovery algorithms**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-19T14:11:59Z
- **Completed:** 2026-03-19T14:27:35Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- RelationshipLocation struct with File, Line, ByteOffset, Evidence fields and deterministic dedup keys
- AggregateSignalsByLocation() groups by (source, target, location) triple with weighted averaging
- Same relationship from 3 algorithms at same file:line produces 1 edge with aggregated confidence
- Different file:line locations for same source/target produce separate edges (no false merging)
- Benchmark: 600 signals aggregated in ~170us on Apple M2 Pro (well under 500ms target)

## Task Commits

Each task was committed atomically:

1. **Task 1: Define RelationshipLocation struct and helpers** - `9d8abed` (feat)
2. **Task 2: Location-aware signal aggregation with weighted averaging** - `b776c2f` (feat)
3. **Task 3: E2E integration test and benchmark** - `c31e99a` (test)

## Files Created/Modified
- `internal/knowledge/types.go` - Added RelationshipLocation struct, LocationKey(), String(), IsValid()
- `internal/knowledge/algo_aggregator.go` - Added Location to DiscoverySignal/AggregatedEdge, AlgorithmWeight map, AggregateSignalsByLocation()
- `internal/knowledge/types_test.go` - Tests for location key determinism, validation, string output
- `internal/knowledge/algo_aggregator_test.go` - Dedup tests, weighted avg verification, E2E pipeline test, benchmark

## Decisions Made
- Used weighted average (not max) for confidence aggregation: better reflects combined evidence strength
- Algorithm weights from CONTEXT.md research: LLM highest (1.0), cooccurrence lowest (0.3)
- Unknown algorithms default to 0.5 weight: safe middle ground
- Preserved existing AggregateSignals() for backward compatibility: callers without location data still work

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Location infrastructure ready for confidence tier system (Plan 2)
- AggregateSignalsByLocation ready to wire into edge provenance (Plan 3)
- All existing tests continue to pass (backward compatible)

---
*Phase: 02-accuracy-foundation*
*Completed: 2026-03-19*
