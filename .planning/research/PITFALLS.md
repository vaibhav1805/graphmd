# Pitfalls Research

**Domain:** Adding multi-language code analysis and MCP server to an existing Go CLI dependency graph tool
**Researched:** 2026-03-29
**Confidence:** MEDIUM-HIGH (specific to graphmd's architecture; MCP ecosystem still evolving)

## Critical Pitfalls

### Pitfall 1: Stdout Pollution Breaks MCP STDIO Transport

**What goes wrong:**
The MCP spec requires that in stdio transport, the server MUST NOT write anything to stdout that is not a valid JSON-RPC message. Any log output, debug print, fmt.Println, or library that writes to stdout will corrupt the protocol stream and cause client disconnects. The official Go SDK has a known open issue (#572) where it does not enforce this rule.

**Why it happens:**
graphmd already uses stdout for JSON output in query commands. When wrapping these as MCP tools, developers naturally reuse existing print paths. Go libraries (including SQLite drivers) may write warnings to stdout. A single stray `fmt.Println` during debugging breaks the entire transport.

**How to avoid:**
- Redirect all logging to stderr from day one of MCP work. Use `log.SetOutput(os.Stderr)` globally.
- Wrap os.Stdout in a guard that panics or errors in test mode if non-JSON-RPC content is written.
- Separate the query execution layer (returns structured data) from the CLI rendering layer (writes to stdout). MCP tools call the execution layer directly, never the rendering layer.
- Test MCP tools by capturing stdout and validating every line is valid JSON-RPC.

**Warning signs:**
- MCP client receives parse errors intermittently
- Tools work in CLI mode but fail when called via MCP
- Logs appearing in MCP client output stream

**Phase to address:**
MCP server implementation phase — must be the first thing validated before any tool registration.

---

### Pitfall 2: Regex-Based Dependency Detection Produces Unmanageable False Positives

**What goes wrong:**
Regex patterns for detecting HTTP clients (`http.Get`, `requests.get`), database connections (`sql.Open`, `psycopg2.connect`), and queue producers (`kafka.NewProducer`) match in comments, string literals, test fixtures, dead code, and variable names that happen to contain the pattern. False positive rates of 30-60% are common with naive regex approaches, making the dependency graph unreliable — the opposite of graphmd's value proposition.

**Why it happens:**
Developers start with regex because it's fast to implement and works for obvious cases. But code has structure that text patterns cannot capture: a `sql.Open` in a comment is not a dependency, `http.Get` in a test helper is not a production dependency, and `redis.NewClient` assigned to `_` is dead code.

**How to avoid:**
- Use AST-level parsing, not regex. For Go, use `go/parser` + `go/ast` (stdlib, no CGo). For Python and JS/TS, use tree-sitter via Go bindings or a pure-Go alternative.
- Match on AST node types: function calls, import declarations, assignment targets — not raw text.
- Filter by context: skip comments, skip test files (configurable), skip dead code paths where practical.
- Assign lower confidence (0.4-0.5) to detections that lack context (e.g., import exists but no call-site found). Assign higher confidence (0.7-0.9) when both import and call-site are confirmed.

**Warning signs:**
- Test suite has many `// this is a known false positive` suppressions
- Users report phantom dependencies between unrelated services
- Confidence distribution skews heavily toward low-confidence edges

**Phase to address:**
Language parser phases — each parser must produce AST-level results, never raw text matches. Validate with precision/recall metrics against known test codebases.

---

### Pitfall 3: Signal Merging Destroys Provenance and Creates Confidence Confusion

**What goes wrong:**
graphmd v1 has a 6-tier confidence system for markdown-derived signals. When code-derived signals are merged with markdown signals for the same relationship, naive approaches (averaging, taking max, summing) lose critical information. If markdown says "auth-service → postgres" at 0.6 confidence and code analysis confirms `sql.Open("postgres://...")` in auth-service at 0.8 confidence, the merged result should be stronger than either alone — but how much stronger? And crucially, when an agent queries the relationship, it needs to know *why* the confidence is what it is (both sources agreed vs. one source only).

**Why it happens:**
Signal merging is a deceptively hard problem. Developers implement it as an afterthought ("just take the max") rather than designing the data model for it upfront. The existing v1 schema stores a single confidence value per relationship, with no room for per-source confidence breakdown.

**How to avoid:**
- Extend the schema to store per-signal-source confidence: `relationship_signals(rel_id, source_type, confidence, detection_method)` where `source_type` is `markdown` or `code`.
- Define a merging function that is:
  - **Monotonically increasing** when sources agree (two sources saying "yes" is stronger than one)
  - **Transparent** — the merged value can be decomposed back to its inputs
  - **Bounded** — never exceeds 1.0
- A simple formula: `merged = 1 - (1 - conf_markdown) * (1 - conf_code)` (probabilistic OR). Two independent 0.6 signals → 0.84. This is intuitive and well-understood.
- Preserve provenance: query results should include `signal_sources: [{type: "markdown", confidence: 0.6}, {type: "code", confidence: 0.8}]` when `--include-provenance` is used.

**Warning signs:**
- Relationships that existed in v1 change confidence after v2 with no visible reason
- Agents cannot distinguish "detected in code only" from "detected in both code and markdown"
- The merging function has special cases or hardcoded thresholds

**Phase to address:**
Signal merging phase — must come after both markdown and code detection are stable. Schema migration must be designed before implementation begins.

---

### Pitfall 4: CGo Tree-Sitter Dependency Breaks Build Portability

**What goes wrong:**
The standard Go tree-sitter binding (`smacker/go-tree-sitter`) uses CGo, which requires a C compiler toolchain. This breaks `go install` for downstream users without gcc, prevents cross-compilation without a C cross-toolchain per target, and makes the Go race detector and fuzzer unable to see across the CGo boundary. graphmd currently builds as pure Go (modernc.org/sqlite is also pure Go) — adding CGo would be a significant regression in build portability.

**Why it happens:**
Tree-sitter is the natural choice for multi-language AST parsing, and the most mature Go binding uses CGo. Developers reach for the obvious solution without considering build-chain implications.

**How to avoid:**
- For Go: Use `go/parser` + `go/ast` from the standard library. Zero dependencies, pure Go, well-maintained. This is the correct choice for Go parsing.
- For Python and JS/TS: Evaluate options in order of preference:
  1. **Regex on AST-like structures** — parse import statements and common patterns with targeted regex (imports are syntactically simple even without full AST). This is limited but avoids CGo entirely.
  2. **Pure-Go tree-sitter runtime** (`odvcencio/gotreesitter` or `malivvan/tree-sitter` via wazero/WASM) — no CGo, cross-compiles everywhere. Newer and less battle-tested.
  3. **CGo tree-sitter** (`smacker/go-tree-sitter`) — most mature but breaks pure-Go build. Only if other options prove inadequate.
- If CGo is unavoidable, document the requirement clearly and provide pre-built binaries for common platforms.

**Warning signs:**
- `go install` fails on CI or user machines without gcc
- Cross-compilation targets suddenly fail
- Race condition bugs that cannot be reproduced with `-race`

**Phase to address:**
First language parser phase — the parsing technology decision cascades through the entire v2 milestone. Decide and validate before writing any parser code.

---

### Pitfall 5: Language Parsers Coupled to Detection Logic

**What goes wrong:**
Each language parser (Go, Python, JS/TS) detects the same categories of dependencies (HTTP calls, DB connections, queue usage, cache connections) but the detection patterns are completely different per language. Without a clean abstraction boundary, developers end up with three monolithic parsers that each implement their own detection, confidence scoring, and output formatting — tripling maintenance burden and making it impossible to add new dependency categories without touching every parser.

**Why it happens:**
It feels natural to write a "Go analyzer" that does everything for Go. The patterns look different enough per language that developers think shared abstractions don't apply. But the *output* is identical: "component A has a dependency on component B of type X at confidence Y."

**How to avoid:**
- Define a common `Signal` interface that all parsers emit:
  ```go
  type Signal struct {
      SourceComponent string    // inferred from package/module/directory
      TargetComponent string    // what's being connected to
      DependencyType  string    // "database", "http", "queue", "cache"
      Confidence      float64
      Evidence        Evidence  // file, line, detection method
  }
  ```
- Each language parser only does: parse source → walk AST → emit Signals.
- Detection rules (what constitutes an HTTP call, a DB connection) are data-driven where possible: lists of known functions/patterns per language.
- Signal aggregation, confidence computation, and graph building happen in a language-agnostic layer downstream.

**Warning signs:**
- Adding "detect Redis connections" requires changes in 3+ files
- Parser tests duplicate the same assertion logic across languages
- Confidence scoring logic is scattered across parser packages

**Phase to address:**
Must be established in the first parser phase (Go parser) and validated by the second parser (Python). If the abstraction doesn't hold for two languages, fix it before the third.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Regex for Python/JS detection instead of AST | Ships faster, avoids CGo question | High false positives, fragile to code style variations, can't detect dead code | MVP/prototype only — must plan migration to AST |
| Single confidence value per relationship (no per-source breakdown) | Simpler schema migration | Cannot explain confidence to agents, loses signal source information | Never — design the schema correctly from the start |
| Hardcoding known function names for detection | Works for common cases | Misses custom wrappers, alternative libraries, aliased imports | Acceptable as starting point if accompanied by plugin/config mechanism roadmap |
| Testing parsers only against synthetic examples | Fast to write, easy to understand | Real code has edge cases (build tags, conditional imports, dynamic requires) that synthetic tests miss | Early development only — must add real-world corpus tests before shipping |
| MCP server as thin CLI wrapper (shell out to graphmd commands) | Reuses existing CLI, minimal new code | Process spawn overhead per query, stdout/stderr handling fragile, loses type safety | Never — call the Go API layer directly |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Code signals → existing graph schema | Inserting code-detected relationships with markdown-era confidence values | Add `source_type` column to distinguish provenance; use separate confidence computation for code vs markdown |
| Go `go/ast` parser | Parsing only `.go` files, missing `go.mod` for module name resolution | Parse `go.mod` first to establish module path; use it to resolve import paths to component names |
| Python imports | Treating `import redis` as "this service uses Redis" | Verify call-site exists (import + usage), not just import. An unused import is not a dependency. |
| JS/TS imports | Assuming `require()` and `import` are equivalent | Handle both CommonJS and ESM patterns; also handle dynamic `import()` and re-exports |
| Connection strings | Parsing URLs with regex | Use `net/url` for URL parsing; handle DSN formats separately (e.g., `postgres://` vs `host=x port=5432 dbname=y`) |
| MCP tool registration | Registering one tool per query type (impact, dependencies, path, list) | Fine — but ensure tool descriptions are LLM-readable. Agents select tools based on descriptions, not names. |
| Existing `.graphmdignore` | Applying ignore rules only to markdown files | Extend ignore rules to code files too — users expect consistent behavior |
| Named graphs | Code analysis producing different component names than markdown for the same entity | Leverage existing component aliasing (`aliases.yaml`) to unify names across detection sources |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Parsing entire codebase on every export | Export takes minutes instead of seconds for large repos | Implement file-level caching with mtime checks; only re-parse changed files | >10K source files or >500K LOC |
| Loading all ASTs into memory simultaneously | OOM on large monorepos | Parse files sequentially or in bounded worker pools; emit signals as you go, don't accumulate ASTs | >50K files or >2GB source |
| Tree-sitter grammar loading per file | Grammar compilation overhead repeated for every file | Load grammars once at startup, reuse parser instances per language | >1K files of same language |
| SQLite write contention during code signal ingestion | Slow export with "database is locked" errors | Batch signal inserts in transactions (1000+ per batch); single-writer pattern already established in v1 | >100K signals |
| MCP server holding SQLite connection open indefinitely | Connection leak, database lock prevents export | Use connection pooling or open/close per query; production is read-only so contention is minimal | Long-running MCP server process |
| Recursive directory walking without early termination | Scanning `node_modules/`, `.git/`, `vendor/` | Honor `.graphmdignore` and hardcode exclusions for known large directories (node_modules, .git, vendor, __pycache__) | Any JS/TS project with node_modules |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Connection strings in graph output | Leaking database passwords, API keys from parsed code | Strip credentials from detected connection strings before storing; store only host/port/dbname |
| MCP server without input validation | Malicious tool arguments causing path traversal or command injection | Validate all MCP tool inputs against allowlists; component names should match `[a-zA-Z0-9_-]+` |
| Executing code during "analysis" | Accidentally running `go build` or `npm install` during scanning | Pure static analysis only — never invoke language toolchains during parsing |
| Exposing file paths in MCP responses | Information disclosure about server filesystem structure | Return relative paths from project root, never absolute paths |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Code analysis silently produces no results for unsupported patterns | Users think the tool is broken | Emit warnings for files that were parsed but yielded no signals; distinguish "no dependencies found" from "parser doesn't cover this pattern" |
| Mixed confidence semantics between v1 and v2 | Agents see confidence values change meaning across versions | Document confidence semantics clearly; keep v1 confidence scale for markdown, add explicit `source_type` field so agents can filter |
| MCP tool errors return generic messages | Agents cannot recover or try alternative queries | Return structured error responses with error codes, suggested alternatives, and affected component names |
| Export time regression | Users accustomed to fast markdown-only export are surprised by slow code analysis | Make code analysis opt-in via `--analyze-code` flag; default export behavior unchanged from v1 |
| No progress indication during code analysis | Users think the tool hung on large repos | Emit progress to stderr: `Analyzing Go files: 142/500` |

## "Looks Done But Isn't" Checklist

- [ ] **Go parser:** Often missing build tag handling (`//go:build`) — verify parser processes all build-tag variants, not just default
- [ ] **Go parser:** Often missing `internal/` package visibility rules — verify detected dependencies respect Go's internal package convention
- [ ] **Python parser:** Often missing virtual environment detection — verify parser doesn't follow imports into `site-packages/`
- [ ] **Python parser:** Often missing `__init__.py` package structure — verify module names resolve correctly for package imports
- [ ] **JS/TS parser:** Often missing TypeScript path aliases (`tsconfig.json` paths) — verify `@app/foo` resolves to actual module
- [ ] **JS/TS parser:** Often missing `package.json` workspaces — verify monorepo internal dependencies detected
- [ ] **Connection strings:** Often missing environment variable references (`os.Getenv("DB_URL")`) — verify these are flagged as "unresolvable at static analysis time" rather than silently skipped
- [ ] **MCP server:** Often missing graceful shutdown — verify SIGTERM handling closes SQLite connections cleanly
- [ ] **MCP server:** Often missing tool schema validation — verify JSON Schema for tool inputs matches actual parameter handling
- [ ] **Signal merging:** Often missing idempotency — verify re-running export with same inputs produces identical graph (no confidence drift)

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| False positive flood from regex detection | MEDIUM | Add suppression config (`false_positives.yaml`), retro-fit AST parsing for worst offenders, re-export |
| Stdout pollution in MCP | LOW | Global stderr redirect, add stdout-guard test, re-deploy |
| Confidence confusion after signal merging | HIGH | Schema migration to add per-source confidence, backfill existing data, update all query paths |
| CGo dependency in build chain | HIGH | Rewrite parser to use pure-Go alternative, update CI, re-test all platforms |
| Parser coupled to detection logic | MEDIUM | Extract Signal interface, refactor parsers to emit signals, centralize aggregation — but touching all parsers |
| Connection string credentials leaked in graph | HIGH | Scrub existing exported graphs, add credential stripping to parser, re-export all affected graphs |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Stdout pollution breaks MCP | MCP server phase | Test that captures stdout during tool execution; zero non-JSON-RPC output |
| Regex false positives | Each language parser phase | Precision/recall metrics against annotated test corpus; >90% precision target |
| Signal merging confidence confusion | Signal merging phase | Round-trip test: signals in → merged confidence → decompose back to per-source values |
| CGo build portability | First parser phase (technology selection) | CI matrix: Linux amd64, macOS arm64, Windows amd64 — all cross-compile without C toolchain |
| Parsers coupled to detection | First parser phase (Go), validated in second (Python) | Adding a new dependency type requires changes in exactly one detection-rules file plus one test |
| Connection string credential leak | Each parser phase + integration testing | Test that no exported graph contains strings matching credential patterns |
| Export time regression | Code analysis phase | Benchmark: export of 10K-file repo completes in <30s; opt-in flag preserves v1 performance |
| MCP graceful shutdown | MCP server phase | Test: SIGTERM during active query → clean exit, no corrupted state |

## Sources

- [MCP Go SDK — Official Repository](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Go SDK stdout enforcement issue #572](https://github.com/modelcontextprotocol/go-sdk/issues/572)
- [MCP Specification — Transports](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)
- [MCP Error Handling Best Practices](https://mcpcat.io/guides/error-handling-custom-mcp-servers/)
- [MCP 2026 Roadmap](http://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/)
- [MCP Production Growing Pains — The New Stack](https://thenewstack.io/model-context-protocol-roadmap-2026/)
- [mark3labs/mcp-go — Community Go SDK](https://github.com/mark3labs/mcp-go)
- [smacker/go-tree-sitter — CGo Bindings](https://github.com/smacker/go-tree-sitter)
- [odvcencio/gotreesitter — Pure Go Runtime](https://github.com/odvcencio/gotreesitter)
- [malivvan/tree-sitter — WASM-based Go Bindings](https://github.com/malivvan/tree-sitter)
- [Tree-sitter Parsing Performance — Symflower](https://symflower.com/en/company/blog/2023/parsing-code-with-tree-sitter/)
- [AST Parsing at Scale — Dropstone Research](https://www.dropstone.io/blog/ast-parsing-tree-sitter-40-languages)
- [DeusData/codebase-memory-mcp — Code Intelligence MCP Server](https://github.com/DeusData/codebase-memory-mcp)
- [Go analysis package — golang.org/x/tools](https://pkg.go.dev/golang.org/x/tools/go/analysis)

---
*Pitfalls research for: graphmd v2.0 — code analysis + MCP server*
*Researched: 2026-03-29*
