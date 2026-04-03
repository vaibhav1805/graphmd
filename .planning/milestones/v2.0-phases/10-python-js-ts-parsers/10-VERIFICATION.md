---
phase: 10-python-js-ts-parsers
verified: 2026-03-31T00:00:00Z
status: passed
score: 12/12 must-haves verified
re_verification: false
---

# Phase 10: Python + JS/TS Parsers Verification Report

**Phase Goal:** Operators can analyze Python and JavaScript/TypeScript source code to detect the same categories of infrastructure dependencies as Go
**Verified:** 2026-03-31
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | PythonParser detects HTTP calls from requests and httpx libraries with URL target extraction | VERIFIED | 9 HTTP patterns in DefaultPythonPatterns; 26 tests pass including TestHTTPRequests, TestHTTPHttpx |
| 2  | PythonParser detects database connections from psycopg2, asyncpg, pymongo, sqlalchemy with connection string targets | VERIFIED | 6 DB patterns; TestDatabasePsycopg2, TestDatabaseAsyncpg, TestDatabaseMongo, TestDatabaseSQLAlchemy all pass |
| 3  | PythonParser detects cache client usage from redis and pymemcache | VERIFIED | 4 cache patterns in DefaultPythonPatterns; cache_client signals emitted |
| 4  | PythonParser detects queue clients from kafka-python, pika, and boto3 SQS | VERIFIED | 4 queue patterns; boto3 SQS guard (boto3SQSArgRe) correctly filters non-SQS calls |
| 5  | PythonParser handles import aliasing and from-imports correctly | VERIFIED | Two-phase import scan: aliasedImportRe, bareImportRe, fromImportRe all implemented; TestFromImportAliased passes |
| 6  | PythonParser skips test files and comment lines without false positives | VERIFIED | isTestFile() checks test_*.py, *_test.py, conftest.py; stripInlineComment() implemented; TestTestFileSkipped, TestCommentLineSkipped, TestDecoratorLineSkipped pass |
| 7  | PythonParser emits comment_hint signals from Python # comments | VERIFIED | commentHintRe matches "# Calls/Depends on/Uses/Connects to"; TestCommentHint passes |
| 8  | JSParser detects HTTP calls from axios, fetch (global), node-fetch, and got libraries with URL target extraction | VERIFIED | 8 HTTP patterns; fetch as globalBarePattern (no import required); all related tests pass |
| 9  | JSParser detects database connections from pg, mysql2, mongoose, mongodb, prisma, sequelize | VERIFIED | 8 DB patterns with IsConstructor support for new Pool(), new PrismaClient(), etc.; tests pass |
| 10 | JSParser detects cache client usage from ioredis and redis | VERIFIED | 2 cache patterns; TestRedisCreateClient, ioredis constructor pattern tested |
| 11 | JSParser detects queue clients from kafkajs, amqplib, and AWS SQS SDK | VERIFIED | 3 queue patterns; TestSQSClient, TestCommentHintVariants all pass |
| 12 | JSParser handles both ESM imports and CommonJS require() including destructured imports | VERIFIED | 4 import regexes (esmDefaultRe, esmNamedRe, cjsDefaultRe, cjsDestructuredRe); TestDestructuredESMImport passes |

**Additional truths from Plan 03 (integration layer):**

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| A  | Running --analyze-code on a directory with Python files produces Python CodeSignal output | VERIFIED | TestRunCodeAnalysisPython in integration_test.go passes; signals with Language="python" confirmed |
| B  | Running --analyze-code on a directory with JS/TS files produces JavaScript CodeSignal output | VERIFIED | TestRunCodeAnalysisJavaScript passes; signals with Language="javascript" confirmed |
| C  | Running --analyze-code on mixed-language directory produces signals from all three parsers | VERIFIED | TestRunCodeAnalysisMultiLanguage passes; all three langs confirmed present |
| D  | InferSourceComponent returns the package name from pyproject.toml, setup.py, or package.json | VERIFIED | All three manifest types implemented in analyzer.go; TestInferSourceComponentPython, TestInferSourceComponentJS pass |
| E  | AnalyzeDir skips __tests__, __pycache__, .venv, venv, .tox directories | VERIFIED | skipDirs map contains all 10 entries including all required Python/JS dirs |

**Score:** 12/12 core truths verified (+ 5/5 integration truths verified)

---

### Required Artifacts

#### Plan 10-01: Python Parser

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/code/pyparser/parser.go` | PythonParser implementing LanguageParser interface | VERIFIED | 396 lines; compile-time check `var _ code.LanguageParser = (*PythonParser)(nil)`; ParseFile fully implemented |
| `internal/code/pyparser/patterns.go` | Python detection patterns for HTTP, DB, cache, queue libraries | VERIFIED | 127 lines; DefaultPythonPatterns with 24 patterns; all regex vars defined |
| `internal/code/pyparser/parser_test.go` | Comprehensive tests with inline Python source fixtures (min 100 lines) | VERIFIED | 554 lines; 26 test functions |

#### Plan 10-02: JS/TS Parser

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/code/jsparser/parser.go` | JSParser implementing LanguageParser interface | VERIFIED | 395 lines; compile-time check present; ParseFile with full block comment tracking |
| `internal/code/jsparser/patterns.go` | JS/TS detection patterns for HTTP, DB, cache, queue libraries | VERIFIED | 126 lines; DefaultJSPatterns with 25 patterns; 4 import regexes defined |
| `internal/code/jsparser/parser_test.go` | Comprehensive tests with inline JS/TS source fixtures (min 100 lines) | VERIFIED | 543 lines; 24 test functions |

#### Plan 10-03: Integration

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/code/analyzer.go` | Extended skipDirs and InferSourceComponent for Python and JS | VERIFIED | Contains `__pycache__`, `.venv`, `venv`, `.tox`, `__tests__`, `dist`, `build`; InferSourceComponent handles pyproject.toml, setup.py, package.json |
| `internal/code/integration_test.go` | End-to-end tests for Python and JS parser registration | VERIFIED | Contains `pyparser` import; 10 test functions including TestRunCodeAnalysisPython, TestRunCodeAnalysisJavaScript, TestRunCodeAnalysisMultiLanguage |
| `cmd/graphmd/main.go` | All three parsers registered at CLI call site | VERIFIED | Contains `jsparser.NewJSParser()` and `pyparser.NewPythonParser()` calls |

---

### Key Link Verification

#### Plan 10-01 Links

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/code/pyparser/parser.go` | `internal/code/signal.go` | `code.CodeSignal` type usage | VERIFIED | `code.CodeSignal` used in 5+ return sites throughout ParseFile |
| `internal/code/pyparser/parser.go` | `internal/code/analyzer.go` | implements `code.LanguageParser` | VERIFIED | Compile-time check at line 12; `var _ code.LanguageParser = (*PythonParser)(nil)` |

#### Plan 10-02 Links

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/code/jsparser/parser.go` | `internal/code/signal.go` | `code.CodeSignal` type usage | VERIFIED | `code.CodeSignal` used in multiple signal append sites |
| `internal/code/jsparser/parser.go` | `internal/code/analyzer.go` | implements `code.LanguageParser` | VERIFIED | Compile-time check at line 11; `var _ code.LanguageParser = (*JSParser)(nil)` |

#### Plan 10-03 Links

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/graphmd/main.go` | `internal/code/pyparser/parser.go` | `pyparser.NewPythonParser()` call | VERIFIED | Import at line 15, call at line 185 |
| `cmd/graphmd/main.go` | `internal/code/jsparser/parser.go` | `jsparser.NewJSParser()` call | VERIFIED | Import at line 14, call at line 186 |
| `internal/knowledge/export.go` | `internal/code/pyparser/parser.go` | `pyparser.NewPythonParser()` call | VERIFIED | Import at line 22, call at line 232 |
| `internal/knowledge/crawl_cmd.go` | `internal/code/jsparser/parser.go` | `jsparser.NewJSParser()` call | VERIFIED | Import at line 16, call at line 156 |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CODE-02 | 10-01, 10-03 | Python language parser detecting imports, HTTP calls (requests/httpx), DB connections (SQLAlchemy/psycopg), queue clients (kafka/pika/boto3 SQS), and cache clients (redis/memcache) | SATISFIED | PythonParser exists with 24 patterns covering all listed libraries; 26 tests pass; registered at all 3 CLI call sites |
| CODE-03 | 10-02, 10-03 | JavaScript/TypeScript language parser detecting imports, HTTP calls (fetch/axios), DB connections (pg/mysql2/mongoose), queue clients, and cache clients | SATISFIED | JSParser exists with 25 patterns covering all listed libraries; 24 tests pass; registered at all 3 CLI call sites |

No orphaned requirements found. REQUIREMENTS.md maps both CODE-02 and CODE-03 to Phase 10, and both are claimed in plan frontmatter and implemented.

---

### Anti-Patterns Found

No blockers or warnings identified.

The `return nil, nil` occurrences in parser.go files are correct behavior — they represent deliberate early returns for test file skipping (not placeholder stubs). All signal-producing code paths are fully implemented with real detection logic.

---

### Human Verification Required

None — all verifiable behaviors are confirmed programmatically through the test suite.

The following behaviors have test coverage but could optionally be validated by inspecting test output:

**1. Comment hint case sensitivity**
- Test: Run `go test ./internal/code/pyparser/ -v -run TestCommentHint`
- Expected: Signal with TargetComponent matching the named dependency
- Why noted: Case sensitivity of "Calls/Depends on/Uses/Connects to" keywords is regex-controlled; tests confirm exact matches

**2. fetch() without import (JS global)**
- Test: Run `go test ./internal/code/jsparser/ -v -run TestFetch`
- Expected: http_call signal produced even when no axios import exists
- Why noted: Global bare call path is a distinct code path from import-resolved calls

---

### Test Suite Results

```
ok  github.com/graphmd/graphmd/internal/code/pyparser    0.431s  (26 tests)
ok  github.com/graphmd/graphmd/internal/code/jsparser    0.870s  (24 tests)
ok  github.com/graphmd/graphmd/internal/code             0.262s  (10 tests, incl. 5 new integration tests)
ok  github.com/graphmd/graphmd/internal/code/goparser    1.067s  (no regressions)
go build ./cmd/graphmd/: BUILD OK
go vet ./...: CLEAN
```

All 72 tests across all code packages pass. No import cycles. Binary builds cleanly.

---

### Verified Commits

| Commit | Description |
|--------|-------------|
| `4f5abc0` | test(10-01): add failing tests for Python language parser |
| `ef56e5c` | feat(10-01): implement Python language parser with pattern detection |
| `3699a1a` | test(10-02): add failing tests for JS/TS parser |
| `bf2d774` | feat(10-02): implement JS/TS parser with full pattern detection |
| `7905873` | feat(10-03): extend skipDirs and InferSourceComponent for Python/JS |
| `1fc7612` | feat(10-03): register Python/JS parsers at all CLI call sites with integration tests |

---

_Verified: 2026-03-31_
_Verifier: Claude (gsd-verifier)_
