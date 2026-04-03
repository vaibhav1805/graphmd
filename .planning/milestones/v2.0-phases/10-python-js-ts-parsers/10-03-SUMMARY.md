---
phase: 10-python-js-ts-parsers
plan: 03
subsystem: code-analysis
tags: [integration, multi-language, python, javascript, typescript, cli]

# Dependency graph
requires:
  - phase: 10-python-js-ts-parsers
    provides: "PythonParser (10-01) and JSParser (10-02) implementing LanguageParser"
  - phase: 09-code-analysis-foundation
    provides: "LanguageParser interface, CodeAnalyzer, RunCodeAnalysis"
provides:
  - "All three parsers (Go, Python, JS/TS) registered at all CLI call sites"
  - "InferSourceComponent for pyproject.toml, setup.py, package.json"
  - "Extended skipDirs for Python/JS ecosystem directories"
  - "Integration tests for multi-language code analysis"
affects: [12-signal-merging]

# Tech tracking
tech-stack:
  added: []
  patterns: [multi-parser-registration, manifest-based-project-inference]

key-files:
  created: []
  modified:
    - internal/code/analyzer.go
    - internal/code/integration_test.go
    - cmd/graphmd/main.go
    - internal/knowledge/export.go
    - internal/knowledge/crawl_cmd.go

key-decisions:
  - "Manifest check order: go.mod > pyproject.toml > setup.py > package.json (first match wins per directory level)"
  - "Regex for pyproject.toml and setup.py name extraction (no TOML parser dependency needed)"
  - "JSON unmarshal for package.json name extraction (stdlib encoding/json)"

patterns-established:
  - "Multi-parser registration pattern: pass all parsers as variadic args to RunCodeAnalysis"
  - "Manifest-based project name inference generalizes across Go, Python, and JS ecosystems"

requirements-completed: [CODE-02, CODE-03]

# Metrics
duration: 2min
completed: 2026-03-31
---

# Phase 10 Plan 03: Parser Integration Summary

**Multi-language parser registration at all CLI call sites with pyproject.toml/setup.py/package.json project name inference and Python/JS skip directory support**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-31T04:12:04Z
- **Completed:** 2026-03-31T04:14:20Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- All three parsers (Go, Python, JS/TS) registered at index, export, and crawl CLI call sites
- InferSourceComponent extended to detect project names from pyproject.toml, setup.py, and package.json
- skipDirs extended with 7 new Python/JS ecosystem directories (__tests__, __pycache__, .venv, venv, .tox, dist, build)
- 5 new integration tests proving multi-language analysis works end-to-end
- All 72 tests pass across all code packages with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend skipDirs and InferSourceComponent** - `7905873` (feat)
2. **Task 2: Register parsers at CLI call sites and add integration tests** - `1fc7612` (feat)

## Files Created/Modified
- `internal/code/analyzer.go` - Extended skipDirs map (7 new entries), InferSourceComponent with pyproject.toml/setup.py/package.json support
- `internal/code/integration_test.go` - 5 new integration tests: Python analysis, JS analysis, multi-language, Python InferSource, JS InferSource
- `cmd/graphmd/main.go` - Register pyparser and jsparser alongside goparser in index command
- `internal/knowledge/export.go` - Register pyparser and jsparser in export command
- `internal/knowledge/crawl_cmd.go` - Register pyparser and jsparser in crawl command

## Decisions Made
- Manifest check order: go.mod > pyproject.toml > setup.py > package.json at each directory level. First match wins, then stop walking up. This prioritizes Go modules (existing behavior) while gracefully falling back to Python/JS manifests.
- Used regex for pyproject.toml and setup.py name extraction to avoid adding a TOML parser dependency. Sufficient for single-field extraction.
- Used stdlib encoding/json for package.json (already available, no new dependency).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 10 (Python/JS/TS Parsers) is now complete: all three language parsers built and integrated
- Code analysis via --analyze-code now detects Go, Python, and JS/TS dependencies in a single pass
- Ready for Phase 12 (Signal Merging) to integrate code signals into the dependency graph

## Self-Check: PASSED

- All 5 modified files verified present on disk
- Both commits (7905873, 1fc7612) verified in git history

---
*Phase: 10-python-js-ts-parsers*
*Completed: 2026-03-31*
