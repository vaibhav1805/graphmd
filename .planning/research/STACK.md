# Stack Research

**Domain:** Code analysis + MCP server additions to existing Go CLI (graphmd v2.0)
**Researched:** 2026-03-29
**Confidence:** HIGH

## Design Constraint: No CGO

graphmd currently builds as pure Go (modernc.org/sqlite is a CGO-free SQLite port). Introducing CGO would break cross-compilation simplicity and add C toolchain requirements. **All recommendations below preserve the CGO-free build.**

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `go/ast` + `go/parser` + `go/packages` | stdlib (Go 1.24) | Go source code parsing | Native stdlib, zero dependencies, full type info, no CGO. The gold standard for Go analysis — used by `golangci-lint`, `staticcheck`, and every Go tool. |
| `github.com/odvcencio/gotreesitter` | latest (2025) | Python + JS/TS parsing | Pure Go tree-sitter runtime — no CGO, no C toolchain. Ships 206 grammars including Python, JavaScript, TypeScript. ~2.4x slower than C tree-sitter but sufficient for static analysis (not real-time editing). |
| `github.com/modelcontextprotocol/go-sdk` | v1.4.1 | MCP server | Official Go SDK maintained by Anthropic + Google. Stable v1.0+ API. Supports stdio, SSE, and streamable HTTP transports. |
| `net/url` | stdlib (Go 1.24) | Connection string/URL parsing | Standard library handles `scheme://user:pass@host:port/path?query` format covering postgres://, redis://, amqp://, mysql://, etc. No external dependency needed. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/xo/dburl` | v0.24.2 | Database URL scheme normalization | When you need to recognize 50+ database URL schemes and their aliases (e.g., `pg` = `pgsql` = `postgresql`). Useful for classifying detected connection strings by database type. |
| `regexp` | stdlib | Connection string pattern matching | For non-URL connection patterns: DSN strings (`host=X port=Y dbname=Z`), Redis `host:port` shorthand, environment variable patterns (`DATABASE_URL`, `REDIS_URL`). |
| `go/types` | stdlib | Go type resolution | When you need to resolve types to understand what a function call connects to (e.g., is this `Dial()` from `net/http` or `grpc`?). Use with `go/packages` for full type info. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test -race` | Concurrency safety | MCP server is concurrent (multiple sessions); race detector catches bugs. Works because no CGO. |
| `go test -cover` | Coverage tracking | Maintain >85% target for new packages. |

## Installation

```bash
# MCP server SDK
go get github.com/modelcontextprotocol/go-sdk@v1.4.1

# Pure Go tree-sitter (Python + JS/TS parsing)
go get github.com/odvcencio/gotreesitter@latest

# Database URL parsing (optional, for scheme classification)
go get github.com/xo/dburl@v0.24.2

# Go parsing — no install needed, stdlib:
# go/ast, go/parser, go/packages, go/types, net/url, regexp
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `go/ast` (Go parsing) | `github.com/odvcencio/gotreesitter` with Go grammar | Never for Go. Stdlib gives you type info, package resolution, and import graph — tree-sitter gives you syntax only. |
| `gotreesitter` (Python/JS) | `github.com/smacker/go-tree-sitter` | If you accept CGO dependency. Smacker's lib bundles 30+ grammars and has wider adoption (542 stars). But CGO breaks the pure-Go build. |
| `gotreesitter` (Python/JS) | `github.com/tree-sitter/go-tree-sitter` (official) | Same CGO tradeoff as smacker. Official bindings, but requires manual `Close()` calls and C toolchain. |
| `gotreesitter` (Python/JS) | `github.com/T14Raptor/go-fAST` (JS only) | If you only need JavaScript (not Python/TS). Pure Go, fast. But you'd still need a separate Python parser. |
| `gotreesitter` (Python/JS) | Regex-only approach (no AST) | If parsing accuracy doesn't matter. Faster to implement but misses nested structures, multi-line patterns, and produces false positives. Not recommended for a tool whose value is accuracy. |
| `net/url` (connection strings) | `github.com/xo/dburl` | Use dburl alongside net/url when you need scheme alias resolution (knowing `pg://` = PostgreSQL). For just parsing the URL structure, stdlib suffices. |
| Official MCP Go SDK | `github.com/mark3labs/mcp-go` | If you want a higher-level API with less boilerplate. But official SDK has Google backing, v1.0 stability guarantee, and spec compliance. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/smacker/go-tree-sitter` | Requires CGO — breaks pure-Go build, cross-compilation, and `go install` for users without a C compiler | `github.com/odvcencio/gotreesitter` (pure Go) |
| `github.com/tree-sitter/go-tree-sitter` | Same CGO issue as smacker; also requires manual `Close()` calls (memory leak risk) | `github.com/odvcencio/gotreesitter` (pure Go) |
| Full JS/Python interpreters (goja, otto) | Massive overkill — graphmd needs AST structure, not code execution | tree-sitter for syntax trees |
| `go/ast` for Python/JS | Go's stdlib only parses Go source code | tree-sitter for non-Go languages |
| ANTLR4 Go runtime | Heavy runtime, complex grammar management, slower than tree-sitter for this use case | tree-sitter grammars |
| Custom regex-based parsers for Python/JS | Brittle, misses edge cases, maintenance burden grows with language complexity | tree-sitter for reliable parsing |

## Stack Patterns by Variant

**For Go source analysis:**
- Use `go/packages` to load, `go/ast` to traverse, `go/types` to resolve
- Because: Full semantic analysis (imports, types, function signatures) with zero dependencies

**For Python/JS/TS source analysis:**
- Use `gotreesitter` with language-specific grammars + tree-sitter queries
- Because: Syntactic analysis is sufficient for dependency detection (import statements, function calls, connection strings)

**For MCP server:**
- Use official SDK with `StdioTransport` for CLI integration, `StreamableServerTransport` for HTTP
- Because: Stdio is the standard MCP transport (works with Claude, Cursor, etc.); HTTP enables web-based agents

**For connection string detection:**
- Use `regexp` to find candidates in source → `net/url` to parse URLs → `xo/dburl` to classify database type
- Because: Three-stage pipeline (find → parse → classify) handles both URL-format and non-URL-format connection strings

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `modelcontextprotocol/go-sdk@v1.4.1` | Go 1.22+ | Requires generics (Go 1.18+), but tested on 1.22+ |
| `odvcencio/gotreesitter` | Go 1.21+ | Pure Go, no platform constraints |
| `xo/dburl@v0.24.2` | Go 1.18+ | Minimal dependencies |
| `go/packages` (stdlib) | Go 1.24 (already in go.mod) | Part of `golang.org/x/tools` — may need `go get golang.org/x/tools/go/packages` |

**Note on `go/packages`:** While `go/ast` and `go/parser` are stdlib, `go/packages` (the recommended loader) is in `golang.org/x/tools`. This is the standard approach — all major Go analysis tools use it.

## Integration with Existing Codebase

**Existing dependencies preserved:**
- `modernc.org/sqlite` — continues to handle all SQLite operations
- `github.com/yuin/goldmark` — continues to handle markdown parsing
- `gopkg.in/yaml.v3` — continues to handle YAML config (aliases, seed config)

**New packages slot in alongside existing pipeline:**
- Code parsers produce the same `(name, type, confidence, detection_methods)` tuples as markdown detection
- Signal merging combines code signals with markdown signals using existing confidence aggregation
- MCP server wraps existing query interface — no query logic changes needed

**Package organization suggestion:**
```
internal/
  analysis/
    goparser/     # go/ast-based Go analysis
    pyparser/     # tree-sitter Python analysis
    jsparser/     # tree-sitter JS/TS analysis
    connstring/   # connection string detection + parsing
    merge/        # signal merging (code + markdown)
  mcp/
    server.go     # MCP server setup, tool registration
    tools.go      # MCP tool handlers wrapping query interface
```

## Sources

- [Official MCP Go SDK](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — v1.4.1, API reference (HIGH confidence)
- [MCP Go SDK GitHub](https://github.com/modelcontextprotocol/go-sdk) — v1.0.0 stability guarantee, transport options (HIGH confidence)
- [gotreesitter (pure Go)](https://github.com/odvcencio/gotreesitter) — 206 grammars, no CGO, performance benchmarks (MEDIUM confidence — newer project, 428 stars)
- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) — 30+ bundled grammars, CGO-based alternative (HIGH confidence — 542 stars, widely used)
- [official go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) — v0.25.0, CGO bindings (HIGH confidence)
- [xo/dburl](https://pkg.go.dev/github.com/xo/dburl) — v0.24.2, 50+ database URL schemes (HIGH confidence)
- [Go AST static analysis](https://blog.cloudflare.com/building-the-simplest-go-static-analysis-tool/) — patterns for go/ast usage (HIGH confidence)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — confirmed pure Go, no CGO (HIGH confidence)

---
*Stack research for: graphmd v2.0 code analysis + MCP*
*Researched: 2026-03-29*
