---
phase: 10-python-js-ts-parsers
plan: 01
subsystem: code-analysis
tags: [python, regex, parser, http, database, cache, queue, signals]

# Dependency graph
requires:
  - phase: 09-code-analysis-foundation
    provides: "LanguageParser interface, CodeSignal struct, CodeAnalyzer dispatcher"
provides:
  - "PythonParser implementing LanguageParser for .py files"
  - "Python detection patterns for requests, httpx, psycopg2, asyncpg, pymongo, sqlalchemy, pymysql, redis, pymemcache, kafka, pika, boto3"
  - "Import aliasing resolution (import X as Y, from X import Y as Z)"
affects: [10-python-js-ts-parsers, 12-signal-merging]

# Tech tracking
tech-stack:
  added: []
  patterns: [regex-based-python-parsing, two-phase-import-then-call-scan, host-port-extraction]

key-files:
  created:
    - internal/code/pyparser/parser.go
    - internal/code/pyparser/patterns.go
    - internal/code/pyparser/parser_test.go

key-decisions:
  - "Regex-based parsing for Python (no AST/tree-sitter) following plan guidance"
  - "Two-phase scan: import resolution pass then call detection pass"
  - "host:port extraction added to extractURLHost for bootstrap_servers-style targets"
  - "Bare call matching for from-imports (Redis(...), KafkaProducer(...)) separate from obj.fn matching"

patterns-established:
  - "Python parser subpackage pattern: pyparser/ with parser.go, patterns.go, parser_test.go"
  - "From-import handling: track qualifiedName for pattern lookup on bare calls"

requirements-completed: [CODE-02]

# Metrics
duration: 4min
completed: 2026-03-31
---

# Phase 10 Plan 01: Python Language Parser Summary

**Regex-based Python parser detecting HTTP, DB, cache, and queue dependencies with import alias resolution and host:port target extraction**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-31T04:05:49Z
- **Completed:** 2026-03-31T04:09:22Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 3

## Accomplishments
- PythonParser implements LanguageParser interface with compile-time check
- 24 detection patterns across 6 Python library families (HTTP, DB, cache, queue)
- Import aliasing fully supported: bare import, aliased import, from-import, from-import-aliased
- False positive guards: test files, comment lines, decorator lines, import-only (no call)
- Comment hint detection for # Calls/Depends on/Uses/Connects to patterns at 0.4 confidence
- 24 tests all passing covering every pattern family and edge case

## Task Commits

Each task was committed atomically (TDD flow):

1. **RED: Failing tests for Python parser** - `4f5abc0` (test)
2. **GREEN: Implement Python parser and patterns** - `ef56e5c` (feat)

## Files Created/Modified
- `internal/code/pyparser/parser.go` - PythonParser struct, ParseFile with two-phase scan, import resolution, target extraction
- `internal/code/pyparser/patterns.go` - PyDetectionPattern struct, DefaultPythonPatterns (24 patterns), regex definitions
- `internal/code/pyparser/parser_test.go` - 24 test functions with inline Python source fixtures

## Decisions Made
- Regex-based parsing (no AST/tree-sitter) per plan specification and v2.0 architecture decision
- Two-phase scan: first pass builds importMap from import statements, second pass matches calls against patterns
- Added host:port extraction to extractURLHost for bootstrap_servers-style values (e.g., "kafka-broker:9092" -> "kafka-broker")
- Bare call matching (Redis(...), KafkaProducer(...)) handled separately from obj.fn matching to support from-imports

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] host:port target extraction for bootstrap_servers**
- **Found during:** GREEN phase (test failures)
- **Issue:** bootstrap_servers="kafka-broker:9092" extracted as "kafka-broker:9092" instead of "kafka-broker" because url.Parse doesn't extract hostname from bare host:port
- **Fix:** Added host:port splitting in extractURLHost when url.Parse fails to extract hostname
- **Files modified:** internal/code/pyparser/parser.go
- **Verification:** TestQueueKafkaProducer, TestQueueKafkaConsumer, TestFromImportAliased all pass
- **Committed in:** ef56e5c (GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Auto-fix necessary for correctness of Kafka target extraction. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Python parser ready for registration with CodeAnalyzer in Phase 12 (signal merging)
- Pattern established for JS/TS parser (Plan 02/03) to follow same subpackage structure
- LanguageParser interface proven to generalize beyond Go as intended

## Self-Check: PASSED

- All 3 created files verified present on disk
- Both commits (4f5abc0, ef56e5c) verified in git history

---
*Phase: 10-python-js-ts-parsers*
*Completed: 2026-03-31*
