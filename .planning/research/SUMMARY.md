# Project Research Summary

**Project:** graphmd v2.0
**Domain:** Multi-language code analysis + MCP server integration into existing Go CLI dependency graph tool
**Researched:** 2026-03-29
**Confidence:** HIGH

## Executive Summary

graphmd v2.0 extends an existing, production-quality Go CLI tool (v1.1) with two substantial capabilities: multi-language source code analysis that detects service dependencies directly from code (HTTP calls, database connections, queue producers/consumers), and an MCP server that allows LLM agents to query the dependency graph via structured tools. The v1 architecture is already well-designed for this extension — the `internal/knowledge` package handles markdown-derived signals through a pluggable aggregation pipeline, and the new code analysis feeds into that same pipeline as a 5th signal source. The MCP server is an independent protocol adapter that wraps the existing query interface without touching core logic.

The recommended approach is additive: new `internal/code/` and `internal/mcp/` packages slot alongside the existing `internal/knowledge/` package without restructuring it. For language parsing, Go source code uses the stdlib `go/ast` + `go/parser` (zero dependencies, full semantic analysis), while Python and JS/TS start with targeted regex patterns and defer tree-sitter to v2.1 — dependency detection patterns like imports, HTTP calls, and connection strings are syntactically regular enough that regex is sufficient for v2.0 accuracy targets. The MCP server uses the official `modelcontextprotocol/go-sdk@v1.4.1` with stdio transport. All recommendations preserve the existing pure-Go (CGo-free) build constraint.

The dominant risks are: (1) stdout pollution breaking the MCP stdio transport, which requires separating query execution from CLI rendering before any MCP work begins; (2) false positives from regex-based detection, which requires AST-level parsing for Go and calibrated confidence scoring that discounts regex matches below 0.5; and (3) signal merging erasing provenance when code and markdown signals combine for the same relationship, which requires designing the schema for per-source confidence storage upfront rather than retrofitting it. All three are preventable with upfront design decisions — none require novel technical solutions.

## Key Findings

### Recommended Stack

The stack choices are well-constrained by graphmd's existing CGo-free build requirement. For Go source analysis, `go/ast` and `go/parser` from the standard library provide full semantic analysis with zero dependencies. For Python and JS/TS, `gotreesitter` (pure-Go tree-sitter runtime with 206 grammars, no CGo) is available if regex proves insufficient, but targeted regex is the recommended starting point for v2.0 given the regularity of dependency detection patterns. The official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk@v1.4.1`) is the clear choice for the MCP server — it has Google backing, v1.0 stability guarantee, automatic JSON Schema generation from Go structs, and stdio transport support. Connection string parsing uses `net/url` from stdlib for URL-format strings and `regexp` for DSN formats, with optional `xo/dburl@v0.24.2` for database scheme alias resolution.

**Core technologies:**
- `go/ast` + `go/parser` (stdlib): Go source parsing — full semantic analysis, zero dependencies, no CGo
- `modelcontextprotocol/go-sdk@v1.4.1`: MCP server — official SDK, v1.0 stable, stdio transport, schema auto-generation
- `regexp` + `net/url` (stdlib): Connection string detection — three-stage pipeline (find → parse → classify)
- `gotreesitter` (optional, pure Go): Python/JS/TS AST parsing — defer to v2.1 if regex is sufficient

**Critical constraint:** Never use `smacker/go-tree-sitter` or `tree-sitter/go-tree-sitter` — both require CGo, which breaks the pure-Go build, cross-compilation, and `go install` for users without a C compiler.

### Expected Features

**Must have (table stakes):**
- Import/package detection (Go, Python, JS/TS) — baseline signal, every code analysis tool provides this
- HTTP client call detection — service-to-service calls are the primary inter-service dependency type
- Database connection detection — DB connections are the second most common dependency type
- Connection string/URL parsing — shared infrastructure for extracting targets from detected connections
- Signal merging (code + markdown) — without this, code and markdown are separate worlds; the hybrid approach is graphmd's core differentiator
- MCP server with stdio transport — standard MCP transport for Claude Code, Cursor, and similar local LLM clients
- MCP tools: `query_impact`, `query_dependencies`, `query_path`, `list_components` — direct mapping of existing CLI queries

**Should have (competitive, for v2.x):**
- Message queue producer/consumer detection — maps async dependencies invisible in call graphs; high value, high complexity
- Cache connection detection (Redis, Memcached) — lower complexity once DB detection pipeline exists
- Env var reference tracking — signals dependencies even when runtime values are unknown
- MCP graph metadata tool (`graphmd_graph_info`) — lets agents assess graph freshness before querying
- MCP structured output (outputSchema) — improves agent result parsing reliability

**Defer (v2.1+):**
- Code flow tracing (intra-service call chains) — different problem domain, already deferred per PROJECT.md
- gRPC/protobuf detection — service definitions as dependency signals
- Additional language parsers (Rust, Java, C#)
- HTTP/SSE MCP transport — remote access beyond local containers

**Anti-features to reject:** Full AST type resolution, runtime/dynamic analysis, real-time file watching, NLP inside graphmd (the LLM agent IS the NLP layer), SSE/HTTP MCP transport for v2.0.

### Architecture Approach

The v2.0 architecture is strictly additive. Two new packages (`internal/code/`, `internal/mcp/`) slot alongside the existing `internal/knowledge/` monolith, which is preserved unchanged except for minimal extensions at integration points (scanner file extensions, aggregator weight table, schema migration). The dependency direction is one-way: `cmd/graphmd → internal/mcp → internal/knowledge → internal/code`. Code analysis feeds into the existing discovery pipeline as a 5th signal source at weight 0.85 (just below LLM at 1.0, above structural at 0.6). The MCP server is a thin adapter — tool handlers validate input, call existing `internal/knowledge` query functions, and format output; no query logic lives in the MCP layer.

**Major components:**
1. `internal/code/` (new) — Language-specific AST parsers (Go via stdlib, Python/JS via regex) orchestrated by a `CodeAnalyzer` that implements the `LanguageParser` strategy interface; emits `CodeSignal` structs to the knowledge pipeline
2. `internal/mcp/` (new) — MCP server protocol adapter; wraps existing query interface as 5 MCP tools with stdio transport; never contains business logic
3. `internal/knowledge/` (extended) — Discovery pipeline extended with code signal source; schema migrated to v6 adding `source_type` column on edges and a `code_signals` provenance table; `AlgorithmWeight` map extended with `"code": 0.85`

**Schema v6 additions:** `ALTER TABLE graph_edges ADD COLUMN source_type TEXT DEFAULT 'markdown'` and a new `code_signals` table with full provenance (file, line, language, evidence, confidence).

### Critical Pitfalls

1. **Stdout pollution breaks MCP stdio transport** — Redirect all logging to stderr globally (`log.SetOutput(os.Stderr)`) before any MCP work. Separate query execution (returns structured data) from CLI rendering (writes to stdout). MCP tool handlers must call the execution layer, never the rendering layer. One stray `fmt.Println` during debugging corrupts the entire transport.

2. **Regex false positives make the dependency graph unreliable** — Regex matches in comments, test files, dead code, and variable names with incidental matches produce 30-60% false positive rates with naive patterns. Use AST-level parsing for Go (stdlib, free). For Python/JS regex, filter matches by context (skip comment lines, skip test files by default) and assign lower confidence (0.4-0.5) to import-only detections without confirmed call sites.

3. **Signal merging destroys provenance** — "Just take the max" loses information about why confidence is what it is. Design the schema for per-source confidence from the start: `source_type` column on edges plus a relationship signals table. Use probabilistic OR for merging: `merged = 1 - (1 - conf_md) * (1 - conf_code)`. Two independent 0.6 signals → 0.84. Never a simple average.

4. **CGo tree-sitter dependency breaks build portability** — The most mature Go tree-sitter bindings require CGo. This breaks `go install`, cross-compilation, and the race detector. Start with regex for Python/JS; use `gotreesitter` (pure Go) if regex proves insufficient. Never use `smacker/go-tree-sitter` or the official `go-tree-sitter` bindings.

5. **Language parsers coupled to detection logic** — Writing monolithic "Go analyzer", "Python analyzer" that each independently implement detection, confidence scoring, and output formatting triples maintenance burden. Define the `CodeSignal` type and `LanguageParser` interface first. Each parser emits signals only; detection rules are data-driven; aggregation is language-agnostic downstream.

## Implications for Roadmap

Based on research, the natural phase structure follows the dependency order established in ARCHITECTURE.md's build order table, grouped by logical deliverables.

### Phase 1: Code Analysis Foundation

**Rationale:** The `CodeSignal` type and `LanguageParser` interface are the foundational types all parsers build on. Establishing these first, validated by the Go parser (which uses only stdlib), proves the abstraction before committing Python and JS/TS to it. Go is also graphmd's own language — the team has the strongest intuition for what the Go detection output should look like, making it the best validation target.

**Delivers:** `internal/code/signal.go`, `internal/code/analyzer.go`, `internal/code/goparser/` (import + HTTP + DB detection), `internal/code/connstring/` (URL/DSN parsing shared infrastructure). End state: Go source files in a test repo produce accurate `CodeSignal` output with >90% precision.

**Addresses:** Import detection, HTTP client detection, DB connection detection, connection string parsing (table stakes)

**Avoids:** Parser-coupled-to-detection-logic pitfall (interface established here must hold for two languages before the third); CGo build portability pitfall (Go parser uses only stdlib)

### Phase 2: Python and JS/TS Parsers

**Rationale:** Python and JS/TS parsers implement the same `LanguageParser` interface proved in Phase 1. Regex-first approach avoids the CGo pitfall while meeting v2.0 accuracy targets for syntactically regular dependency patterns. Building both in the same phase validates that the `CodeSignal` interface generalizes across language paradigms (CommonJS vs ESM, keyword args vs positional).

**Delivers:** `internal/code/pyparser/` and `internal/code/jsparser/` with import + HTTP + DB detection using targeted regex. Test corpus includes real-world code with comments, test files, aliased imports.

**Addresses:** Python parser pitfall (virtual env detection, `__init__.py` resolution); JS/TS pitfall (CommonJS vs ESM, TypeScript path aliases); false positive mitigation via context filtering

**Avoids:** False positive pitfall — parser tests must use real-world corpus, not just synthetic examples; 90% precision target before phase completes

### Phase 3: Signal Integration

**Rationale:** Code signals must merge with existing markdown signals before the graph is exported. This phase wires the code analysis output into the existing discovery pipeline and migrates the schema. Doing this as a dedicated phase (not tacked onto Phase 1 or 2) ensures the provenance design gets full attention — the signal merging pitfall is the highest-recovery-cost failure mode.

**Delivers:** Schema v6 migration (`source_type` on edges, `code_signals` table), `discovery_orchestration.go` extended with code as 5th parallel signal, `algo_aggregator.go` updated with `"code": 0.85` weight, end-to-end export producing hybrid graphs. Provenance preserved: every edge knows whether it came from markdown, code, or both.

**Addresses:** Signal merging (table stakes); multi-source confidence boosting (differentiator); env var reference tracking (should-have)

**Avoids:** Signal merging provenance destruction pitfall — schema for per-source confidence storage designed and validated here, not retrofitted later

### Phase 4: MCP Server

**Rationale:** The MCP server is independent of code analysis (it wraps the existing v1 query interface). It can be built in parallel with Phases 1-3 or sequentially after — ordering it last ensures it queries a richer, hybrid graph from day one. The stdout pollution pitfall makes this phase require careful test infrastructure before any tool registration.

**Delivers:** `internal/mcp/server.go` + `tools.go` + `resources.go`, `cmd/graphmd mcp` subcommand, 5 MCP tools (`query_impact`, `query_dependencies`, `query_path`, `list_components`, `graphmd_graph_info`), graceful SIGTERM shutdown, input validation.

**Addresses:** All MCP table stakes; MCP structured output (outputSchema) differentiator; graph metadata tool (should-have)

**Avoids:** Stdout pollution pitfall — stdout guard test must pass before tool registration begins; thin-adapter pattern enforced (no business logic in MCP layer); MCP server shelling out to CLI anti-pattern

### Phase Ordering Rationale

- **Phases 1-2 ordered by abstraction dependency:** Interface established in Phase 1, validated across two language families in Phase 2. If the `LanguageParser` interface doesn't generalize, it's caught after Go + Python (2 parsers) not after Go + Python + JS/TS (3 parsers).
- **Phase 3 isolated for schema safety:** The schema migration and signal merging are the riskiest integration points in the entire v2.0 milestone. Isolating them into their own phase prevents them from being rushed as an afterthought of Phase 1 or Phase 2.
- **Phase 4 is independently deployable:** The MCP server works on any graph — even v1 markdown-only graphs. This means it could ship before code analysis if needed, or after once the graph is richer.
- **Pitfall avoidance drives phase boundaries:** The CGo decision gates Phase 1; the parser abstraction is validated by the end of Phase 2; provenance design gates Phase 3; stdout isolation gates Phase 4. Each gate prevents the highest-recovery-cost failure modes identified in PITFALLS.md.

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 3 (Signal Integration):** The probabilistic OR merge formula is well-understood, but how the existing `MergeDiscoveredEdges` function handles multi-source edges needs code archaeology. The schema migration strategy for existing deployed graphs (ALTER TABLE vs new schema) needs a decision before implementation.
- **Phase 4 (MCP Server):** The MCP Go SDK is v1.4.1 but the ecosystem is still evolving (2026 roadmap mentions Streamable HTTP transport changes). Tool description phrasing significantly affects agent behavior — worth a dedicated design review before finalizing tool definitions.

Phases with standard patterns (can skip research-phase):

- **Phase 1 (Code Analysis Foundation):** Go stdlib AST parsing is extremely well-documented. The `LanguageParser` strategy pattern is a textbook application. No novel decisions needed.
- **Phase 2 (Python/JS/TS Parsers):** Dependency detection patterns are well-documented per language. The main work is implementing known patterns, not discovering new ones.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All primary recommendations (go/ast, MCP Go SDK, net/url) are stdlib or officially maintained with v1.0+ stability. The pure-Go tree-sitter alternative (gotreesitter) is the only medium-confidence element — newer project, 428 stars, but used only as a fallback option. |
| Features | HIGH | Feature research draws on official MCP spec, well-documented language AST patterns, and competitive analysis. The "table stakes vs differentiator vs anti-feature" classification is well-grounded. |
| Architecture | HIGH | Architecture is based on direct analysis of the existing codebase (~30K LOC in `internal/knowledge/`) plus well-established patterns (strategy interface, adapter pattern). The build order is dependency-ordered and verified against the actual import graph. |
| Pitfalls | MEDIUM-HIGH | The stdout pollution and CGo pitfalls are directly documented in official SDK issue trackers and community sources. The signal merging and false positive pitfalls are based on general code analysis tool experience rather than graphmd-specific incidents — they are well-reasoned but not battle-tested against this specific codebase. |

**Overall confidence:** HIGH

### Gaps to Address

- **go.mod parsing for Go component name resolution:** ARCHITECTURE.md notes that the Go parser must read `go.mod` to establish module paths before resolving import paths to component names. The exact implementation strategy (parse once per repo root, pass module map into parser) needs validation against graphmd's multi-repo export model.
- **TypeScript path alias resolution:** `tsconfig.json` `paths` config means `import { foo } from '@app/foo'` doesn't resolve without reading the tsconfig. Decide at Phase 2 planning whether to support this or document it as a known limitation.
- **Existing graph migration strategy:** Schema v6 adds a `source_type` column with `DEFAULT 'markdown'`. Verify that existing deployed `graph.db` files can be migrated with a simple `ALTER TABLE` (SQLite supports this) and that existing query paths handle the new column gracefully.
- **gotreesitter stability:** The pure-Go tree-sitter alternative has 428 stars and is newer. If Python/JS regex proves insufficient and tree-sitter is needed, validate `gotreesitter` against a sample of real Python and JS codebases before committing to it in a phase plan.

## Sources

### Primary (HIGH confidence)
- `go/ast`, `go/parser`, `net/url` — Go standard library, v1.24
- `github.com/modelcontextprotocol/go-sdk` — Official MCP Go SDK, v1.4.1; Anthropic + Google maintained
- `MCP Specification 2025-06-18` — Tools, transports, outputSchema
- `github.com/xo/dburl@v0.24.2` — Database URL scheme normalization, 50+ schemes
- Existing codebase (`internal/knowledge/*.go`) — Direct analysis, ~30K LOC

### Secondary (MEDIUM confidence)
- `github.com/odvcencio/gotreesitter` — Pure-Go tree-sitter runtime, 428 stars, 206 grammars; newer project
- MCP Go SDK issue #572 — Stdout enforcement gap, confirmed open issue
- `MCP 2026 Roadmap` — Streamable HTTP transport direction
- COD Model (Augment) — Codebase dependency mapping patterns
- MCP Architecture Patterns (IBM) — MCP tool design guidance

### Tertiary (LOW confidence)
- Google importlab, Pyan (Python) — Alternative Python analysis approaches; evaluated and set aside in favor of regex-first
- `malivvan/tree-sitter` (WASM-based Go bindings) — Alternative pure-Go tree-sitter; less researched than gotreesitter

---
*Research completed: 2026-03-29*
*Ready for roadmap: yes*
