# Phase 9: Code Analysis Foundation - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the Go language parser and define the `LanguageParser` interface + `CodeSignal` type in a new `internal/code/` package. Detects infrastructure dependencies from Go source code: HTTP client calls, database connections, message queue producers/consumers, and cache client usage. This phase proves the abstraction that Python and JS/TS parsers will implement in Phase 10.

</domain>

<decisions>
## Implementation Decisions

### Detection Categories & Confidence

- **Confidence tiered by detection strength:**
  - Direct API call (e.g., `http.Get("http://payment-api/...")`) → 0.9
  - Connection string usage (e.g., `sql.Open("postgres", connStr)`) → 0.85
  - Comment hint (e.g., `// Calls payment-api`) → 0.4
  - Note: import-only detections (e.g., `import "database/sql"` without usage) are NOT emitted as signals
- **Require usage evidence:** Import-only detections excluded. A signal is only emitted when actual function calls, connection creation, or client initialization is found
- **Test files excluded:** Files matching `*_test.go` (and equivalent patterns for Python/JS in Phase 10) are skipped. Test dependencies are not production dependencies
- **Target name extraction:** Parser should extract specific queue/cache names (e.g., topic name, queue name) when possible. Falls back to generic type name (e.g., "kafka", "redis") when specific name can't be determined

### CodeSignal Output Structure

- **Fields per signal:**
  - `source_file` — file path where the signal was detected
  - `line_number` — line in the file
  - `target_component` — what dependency was found (e.g., "primary-db", "payment-api")
  - `target_type` — component type (database, service, cache, queue, etc.)
  - `detection_kind` — how it was found (http_call, db_connection, queue_producer, cache_client, etc.)
  - `evidence` — the actual code snippet as evidence
  - `language` — which parser found it (go, python, javascript)
  - `confidence` — detection confidence score
- **Source component inference:** Infer from `go.mod` module name or directory name. No explicit mapping config required

### CLI Integration & Invocation

- **Opt-in flag:** `--analyze-code` available on export, crawl, and index commands
- **Default behavior unchanged:** Without the flag, all three commands stay markdown-only (preserves v1 performance)
- **All detected languages:** When `--analyze-code` is set, run all available parsers on files found. No per-language filtering
- **Consistent across commands:** Same `--analyze-code` flag behavior on export, crawl, and index

### Claude's Discretion

- LanguageParser interface design (method signatures, return types)
- New `internal/code/` package structure
- Go AST traversal patterns for detecting HTTP/DB/queue/cache usage
- How go.mod module name is resolved for source component inference
- Detection pattern specifics (which function calls match which detection kinds)

</decisions>

<specifics>
## Specific Ideas

- Research confirmed: Go stdlib `go/ast` + `go/parser` give full semantic analysis with zero new dependencies
- The LanguageParser interface must not leak Go-specific concepts (Phase 10 parsers for Python/JS must implement it cleanly)
- Source component = go.mod module name is the natural mapping for Go projects

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 09-code-analysis-foundation*
*Context gathered: 2026-03-30*
