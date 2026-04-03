# Architecture Research

**Domain:** Code analysis parsers + MCP server integration into existing Go CLI tool
**Researched:** 2026-03-29
**Confidence:** HIGH

## System Overview

### Current Architecture (v1.1)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          CLI Layer (cmd/graphmd)                        │
│  export │ import │ query │ crawl │ discover │ list │ clean              │
├─────────────────────────────────────────────────────────────────────────┤
│                    internal/knowledge (single package)                   │
│                                                                         │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐   │
│  │ Scanner  │  │  Component   │  │  Discovery   │  │   Query &     │   │
│  │ (md only)│  │  Detector    │  │  Pipeline    │  │   Traversal   │   │
│  └────┬─────┘  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘   │
│       │               │                │                   │           │
│  ┌────┴─────┐  ┌──────┴───────┐  ┌──────┴───────┐  ┌───────┴───────┐   │
│  │ Document │  │ Types/Seed   │  │ Aggregator   │  │ Graph/        │   │
│  │ Extractor│  │ Config       │  │ CoOcc/Struct │  │ ComponentGraph│   │
│  └──────────┘  └──────────────┘  └──────┬───────┘  └───────────────┘   │
│                                         │                               │
├─────────────────────────────────────────┴───────────────────────────────┤
│                        SQLite Persistence (graph.db)                    │
│  documents │ graph_nodes │ graph_edges │ component_mentions │ metadata  │
└─────────────────────────────────────────────────────────────────────────┘
```

### v2.0 Architecture (with code analysis + MCP)

```
┌────────────────────────────────────────────────────────────────────────────┐
│              CLI Layer (cmd/graphmd)          MCP Server                   │
│  export │ import │ query │ crawl │ ...   ┌──────────────────┐             │
│                                          │ internal/mcp     │             │
│                                          │ StdioTransport   │             │
│                                          │ Tools: query,    │             │
│                                          │  impact, list    │             │
│                                          └────────┬─────────┘             │
├───────────────────────────────────────────────────┤                        │
│                    internal/knowledge                                      │
│                                                                            │
│  ┌────────────┐  ┌──────────────┐  ┌───────────────────────┐              │
│  │  Scanner   │  │  Component   │  │   Discovery Pipeline  │              │
│  │  (unified) │  │  Detector    │  │   (md + code signals) │              │
│  └──┬─────┬───┘  └──────┬───────┘  └───────┬───────────────┘              │
│     │     │              │                  │                              │
│  ┌──┴──┐ ┌┴────────────┐│  ┌───────────────┴──────────────────┐           │
│  │ md  │ │internal/code││  │         Signal Merger             │           │
│  │scan │ │             ││  │  md signals + code signals        │           │
│  └─────┘ │ ┌─────────┐ ││  │  → confidence-weighted aggregate │           │
│          │ │Go parser │ ││  └──────────────────────────────────┘           │
│          │ │Py parser │ ││                                                 │
│          │ │JS parser │ ││                                                 │
│          │ └─────────┘ ││                                                  │
│          │ connstrings  ││                                                  │
│          └──────────────┘│                                                  │
│                          │                                                  │
├──────────────────────────┴──────────────────────────────────────────────────┤
│                        SQLite Persistence (graph.db)                       │
│  + code_signals table │ + source_type column on edges                      │
└────────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Current vs New |
|-----------|----------------|----------------|
| `cmd/graphmd/main.go` | CLI dispatch, flag parsing | Modify: add `mcp` subcommand |
| `internal/knowledge/scanner.go` | Directory walking, file discovery | Modify: walk `.go`, `.py`, `.js/.ts` alongside `.md` |
| `internal/knowledge/discovery_orchestration.go` | Run 4 algorithms in parallel, merge | Modify: add code signal source as 5th algorithm |
| `internal/knowledge/algo_aggregator.go` | Signal aggregation, weighted averaging | Modify: add `"code"` algorithm weight |
| `internal/code/` | **NEW**: Language-specific AST parsers | New package |
| `internal/code/goparser/` | **NEW**: Go source parser (go/ast) | New sub-package |
| `internal/code/pyparser/` | **NEW**: Python source parser (regex/tree-sitter) | New sub-package |
| `internal/code/jsparser/` | **NEW**: JS/TS source parser (regex/tree-sitter) | New sub-package |
| `internal/code/connstring/` | **NEW**: Connection string/URL/DSN detection | New sub-package |
| `internal/code/signal.go` | **NEW**: Code signal types, normalization | New file |
| `internal/mcp/` | **NEW**: MCP server wrapping query interface | New package |
| `internal/knowledge/types.go` | Component types, confidence tiers, traversal | Modify: add `SignalCode` signal type |
| `internal/knowledge/edge.go` | Edge types, provenance | Modify: add `"code-analysis"` extraction method |
| `internal/knowledge/db.go` | SQLite schema, read/write | Modify: schema v6 for code signals |

## Recommended Project Structure

```
graphmd/
├── cmd/graphmd/
│   └── main.go                    # Add "mcp" command dispatch
├── internal/
│   ├── knowledge/                 # EXISTING — modify, don't restructure
│   │   ├── scanner.go             # Extend: scan code files alongside markdown
│   │   ├── discovery_orchestration.go  # Extend: add code signals as input
│   │   ├── algo_aggregator.go     # Extend: add "code" weight
│   │   ├── types.go               # Extend: SignalCode constant
│   │   ├── edge.go                # Extend: "code-analysis" extraction method
│   │   ├── db.go                  # Extend: schema v6
│   │   └── ...                    # Existing files unchanged
│   ├── code/                      # NEW — code analysis package
│   │   ├── analyzer.go            # CodeAnalyzer: orchestrates per-language parsers
│   │   ├── signal.go              # CodeSignal type: {source, target, type, confidence, evidence, file, line}
│   │   ├── connstring/            # Connection string detection
│   │   │   ├── detector.go        # URL/DSN pattern matching
│   │   │   └── detector_test.go
│   │   ├── goparser/              # Go-specific parser
│   │   │   ├── parser.go          # Uses go/ast, go/parser (stdlib)
│   │   │   └── parser_test.go
│   │   ├── pyparser/              # Python-specific parser
│   │   │   ├── parser.go          # Regex + tree-sitter
│   │   │   └── parser_test.go
│   │   └── jsparser/              # JS/TS-specific parser
│   │       ├── parser.go          # Regex + tree-sitter
│   │       └── parser_test.go
│   └── mcp/                       # NEW — MCP server package
│       ├── server.go              # Server setup, tool registration
│       ├── tools.go               # Tool handlers (query, impact, list, etc.)
│       ├── resources.go           # Resource handlers (graph metadata)
│       └── server_test.go
└── ...
```

### Structure Rationale

- **`internal/code/` as separate package:** Code analysis has no dependency on markdown parsing. Clean boundary. The `internal/knowledge` package is already ~30K LOC; adding parsers there would make it unwieldy.
- **Per-language sub-packages:** Each parser has different dependencies (Go stdlib AST vs tree-sitter). Sub-packages prevent import contamination.
- **`internal/mcp/` as separate package:** MCP is a protocol adapter, not core logic. It imports `internal/knowledge` for query execution but knowledge should not import mcp.
- **No restructuring of existing code:** The `internal/knowledge` package works well as a monolith for graph operations. Breaking it up would create churn with zero value.

## Architectural Patterns

### Pattern 1: Parser Interface (Strategy Pattern)

**What:** Define a common interface for all language parsers. The `CodeAnalyzer` orchestrates them.
**When to use:** Adding new language support (Rust, Java, etc.) in future.
**Trade-offs:** Small overhead of interface dispatch; huge extensibility win.

```go
// internal/code/analyzer.go

// CodeSignal represents a dependency detected from source code.
type CodeSignal struct {
    Source     string  // component/service name (inferred or detected)
    Target     string  // target component name
    Type       string  // "http-call", "db-connection", "queue-publish", etc.
    Confidence float64 // [0.4, 1.0]
    Evidence   string  // e.g. "http.Get("http://user-service/api/users")"
    File       string  // source file path (relative)
    Line       int     // line number
    Language   string  // "go", "python", "javascript", "typescript"
}

// LanguageParser extracts dependency signals from source files.
type LanguageParser interface {
    // Extensions returns file extensions this parser handles (e.g. [".go"]).
    Extensions() []string

    // ParseFile analyzes a single file and returns dependency signals.
    ParseFile(path string, content []byte) ([]CodeSignal, error)
}

// CodeAnalyzer orchestrates all registered language parsers.
type CodeAnalyzer struct {
    parsers map[string]LanguageParser // extension → parser
}

func (a *CodeAnalyzer) AnalyzeDirectory(root string) ([]CodeSignal, error) {
    // Walk directory, dispatch to parsers by extension, collect signals
}
```

### Pattern 2: Signal Merging (Existing Pattern Extended)

**What:** Code signals feed into the existing `DiscoverAndIntegrateRelationships` pipeline as a 5th signal source alongside co-occurrence, structural, NER, and LLM.
**When to use:** Always — this is the integration point.
**Trade-offs:** Reuses existing aggregation and confidence-weighted merging. No new merge logic needed.

```go
// In discovery_orchestration.go — extend DiscoverAndIntegrateRelationships

// Add code analysis as 5th parallel algorithm:
var codeEdges []*DiscoveredEdge

go func() {
    defer wg.Done()
    if codeAnalyzer != nil {
        signals, _ := codeAnalyzer.AnalyzeDirectory(rootDir)
        codeEdges = convertCodeToDiscovered(signals)
    }
}()

// Merge with existing: MergeDiscoveredEdges(coOcc, struct, llm, ner, codeEdges)
```

The existing `AlgorithmWeight` map gets a new entry:

```go
var AlgorithmWeight = map[string]float64{
    "cooccurrence": 0.3,
    "ner":          0.5,
    "structural":   0.6,
    "semantic":     0.7,
    "code":         0.85, // Code analysis is high signal, just below LLM
    "llm":          1.0,
}
```

### Pattern 3: MCP Server as Thin Adapter

**What:** The MCP server is a protocol translation layer. It accepts MCP tool calls and delegates to existing `internal/knowledge` query functions.
**When to use:** For the MCP server implementation.
**Trade-offs:** Keeps MCP concerns isolated; query logic stays in knowledge package.

```go
// internal/mcp/server.go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/graphmd/graphmd/internal/knowledge"
)

func NewGraphMCPServer(dbPath string) (*mcp.Server, error) {
    db, err := knowledge.OpenDB(dbPath)
    // ...

    server := mcp.NewServer(
        &mcp.Implementation{Name: "graphmd", Version: "2.0.0"}, nil,
    )

    // Register tools that map directly to existing query patterns
    mcp.AddTool(server, &mcp.Tool{
        Name: "query_impact",
        Description: "Find components affected when a given component fails",
    }, makeImpactHandler(db))

    mcp.AddTool(server, &mcp.Tool{
        Name: "query_dependencies",
        Description: "List dependencies of a component",
    }, makeDependenciesHandler(db))

    mcp.AddTool(server, &mcp.Tool{
        Name: "query_path",
        Description: "Find dependency paths between two components",
    }, makePathHandler(db))

    mcp.AddTool(server, &mcp.Tool{
        Name: "list_components",
        Description: "List all components, optionally filtered by type",
    }, makeListHandler(db))

    return server, nil
}
```

### Pattern 4: Go Parser via Stdlib AST

**What:** For Go files, use `go/parser` and `go/ast` from the standard library. No external dependencies.
**When to use:** Go source analysis.
**Trade-offs:** Rock-solid, zero dependencies, but Go-only. Other languages need different approaches.

```go
// internal/code/goparser/parser.go

func (p *GoParser) ParseFile(path string, content []byte) ([]code.CodeSignal, error) {
    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, path, content, parser.ImportsOnly|parser.ParseComments)
    // ...

    // 1. Extract import paths → identify known frameworks/libraries
    // 2. Walk AST for http.Get/Post calls → service-to-service
    // 3. Walk for sql.Open, gorm.Open → database connections
    // 4. Walk for redis.NewClient → cache connections
    // 5. Detect connection string patterns in string literals
}
```

### Pattern 5: Regex-First for Python/JS (No Tree-Sitter Initially)

**What:** Use regex pattern matching for Python and JS/TS initial parsers. Add tree-sitter later if regex proves insufficient.
**When to use:** Python and JS/TS code analysis in v2.0.
**Trade-offs:** Faster to implement, fewer dependencies (no CGo), but less accurate for complex cases. Good enough for dependency detection patterns (imports, connection strings, HTTP calls) which are syntactically regular.

**Rationale:** The dependency patterns we detect (import statements, `requests.get()`, `fetch()`, `createClient()`, connection strings) are highly regular and grep-able. Full AST parsing is overkill for v2.0 scope. Tree-sitter can be added as an optimization later.

```go
// internal/code/pyparser/parser.go
// Detect: import X, from X import Y, requests.get/post, psycopg2.connect,
//         sqlalchemy.create_engine, redis.Redis, pika.BlockingConnection

var pyImportPattern = regexp.MustCompile(`(?m)^(?:import|from)\s+(\S+)`)
var pyHTTPCallPattern = regexp.MustCompile(`requests\.(get|post|put|delete|patch)\s*\(\s*["']([^"']+)["']`)
var pyDBConnectPattern = regexp.MustCompile(`(?:psycopg2\.connect|create_engine|MongoClient)\s*\(\s*["']([^"']+)["']`)
```

## Data Flow

### Export Pipeline (Extended)

```
Source Directory
    │
    ├── .md files ──→ ScanDirectory() ──→ []Document
    │                      │
    │                      ├── Extractor ──→ explicit edges (links, mentions)
    │                      │
    │                      └── Discovery algorithms (4 existing)
    │                              │
    ├── .go/.py/.js files ──→ CodeAnalyzer ──→ []CodeSignal
    │                              │
    │                              ├── GoParser (go/ast)
    │                              ├── PyParser (regex)
    │                              └── JSParser (regex)
    │
    ├── config files ──→ ConnStringDetector ──→ []CodeSignal
    │   (.env, .yaml,       │
    │    docker-compose)     └── URL/DSN pattern matching
    │
    └──→ Signal Merger (extended algo_aggregator)
              │
              ├── md signals (co-occ, structural, NER, LLM)
              ├── code signals (parser-detected)
              └── connstring signals
              │
              ▼
         Confidence-weighted aggregate
              │
              ▼
         FilterDiscoveredEdges (quality gate)
              │
              ▼
         SaveGraph (SQLite) → packageZIP → graph.zip
```

### MCP Query Flow

```
LLM Agent
    │
    ▼
MCP Client (stdio transport)
    │
    ▼
graphmd mcp (cmd/graphmd)
    │
    ▼
internal/mcp/server.go
    │
    ├── query_impact     ──→ knowledge.GetImpact()
    ├── query_dependencies ──→ knowledge.TransitiveDeps()
    ├── query_path       ──→ knowledge.FindPaths()
    └── list_components  ──→ knowledge.ListComponents()
    │
    ▼
SQLite (graph.db via XDG storage)
    │
    ▼
JSON result → MCP response → LLM Agent
```

### Signal Merging Detail

```
Code signals arrive as DiscoveredEdge values with:
  - SignalType: SignalCode (new constant)
  - Algorithm:  "code"
  - Weight:     0.85 (in AlgorithmWeight map)
  - Confidence: varies by detection type:
      - import statement:    0.90 (strong structural signal)
      - HTTP client call:    0.80 (URL may be variable)
      - DB connection init:  0.85 (connection strings are reliable)
      - Queue pub/sub:       0.75 (pattern matching, some ambiguity)
      - Connection string:   0.85 (URL/DSN parsing is deterministic)

Merged with existing md signals via MergeDiscoveredEdges():
  - Same source+target: signals aggregate, highest confidence wins
  - Code + markdown agree: multi-signal edge, passes quality gate easily
  - Code-only signal: passes if confidence >= MinConfidence (0.60)
```

## Integration Points

### Modified Existing Code

| File | Change | Risk |
|------|--------|------|
| `scanner.go` | Add code file extensions to walk | LOW — additive, no behavior change for .md |
| `discovery_orchestration.go` | Add code signals as 5th parallel source | LOW — existing pattern, new goroutine |
| `algo_aggregator.go` | Add `"code": 0.85` to AlgorithmWeight | LOW — single line |
| `types.go` | Add `SignalCode` constant | LOW — additive |
| `edge.go` | Add `"code-analysis"` to validExtractionMethods | LOW — additive |
| `db.go` | Schema v6: add `source_type` column to edges, `code_signals` table | MEDIUM — migration needed |
| `export.go` | Pass code analyzer to discovery pipeline | LOW — threading new parameter |
| `main.go` | Add `mcp` subcommand | LOW — additive |

### New Internal Boundaries

| Boundary | Communication | Direction |
|----------|---------------|-----------|
| `cmd/graphmd` → `internal/mcp` | Function call (NewGraphMCPServer) | cmd imports mcp |
| `internal/mcp` → `internal/knowledge` | Function call (OpenDB, query methods) | mcp imports knowledge |
| `internal/knowledge` → `internal/code` | Function call (CodeAnalyzer.AnalyzeDirectory) | knowledge imports code |
| `internal/code/*parser` → `internal/code` | Interface (LanguageParser) | sub-packages implement code.LanguageParser |

**Dependency direction is strictly one-way:**
```
cmd/graphmd → internal/mcp → internal/knowledge → internal/code
                                                 ↗
                              internal/code/*parser
```

No cycles. `internal/code` never imports `internal/knowledge`. `internal/mcp` never imports `internal/code`.

### External Dependencies

| Dependency | Purpose | Import Path | Notes |
|------------|---------|-------------|-------|
| Official MCP Go SDK | MCP server + transport | `github.com/modelcontextprotocol/go-sdk/mcp` | Maintained by Anthropic + Google |
| tree-sitter (optional, deferred) | Python/JS AST parsing | `github.com/tree-sitter/go-tree-sitter` | CGo dependency; defer to v2.1 if regex suffices |
| go/ast, go/parser (stdlib) | Go source parsing | `go/ast`, `go/parser` | No external dependency |

## Schema Changes (v5 → v6)

```sql
-- New: track which source type produced each edge
ALTER TABLE graph_edges ADD COLUMN source_type TEXT NOT NULL DEFAULT 'markdown';
-- Values: 'markdown', 'code', 'config', 'merged'

-- New: store raw code signals for provenance
CREATE TABLE IF NOT EXISTS code_signals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_component TEXT NOT NULL,
    target_component TEXT NOT NULL,
    signal_type TEXT NOT NULL,       -- 'http-call', 'db-connection', 'queue-publish', etc.
    confidence REAL NOT NULL,
    evidence TEXT NOT NULL,
    file_path TEXT NOT NULL,
    line_number INTEGER NOT NULL,
    language TEXT NOT NULL,           -- 'go', 'python', 'javascript', 'typescript'
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_code_signals_source ON code_signals(source_component);
CREATE INDEX idx_code_signals_target ON code_signals(target_component);
```

## Anti-Patterns

### Anti-Pattern 1: Parsing Code Files in the Knowledge Package

**What people do:** Add Go/Python/JS parsing directly into `internal/knowledge` alongside markdown parsing.
**Why it's wrong:** Bloats an already-large package (30K LOC). Mixes concerns. Makes testing harder — parser tests shouldn't need SQLite.
**Do this instead:** Separate `internal/code/` package with clean interface. Knowledge imports code, never the reverse.

### Anti-Pattern 2: Full AST for Simple Pattern Detection

**What people do:** Pull in tree-sitter or heavy parser for detecting `import` statements and connection strings.
**Why it's wrong:** These patterns are syntactically regular. `requests.get("http://...")` doesn't need an AST — it needs a regex. Adding CGo (tree-sitter) complicates builds and CI.
**Do this instead:** Start with regex for Python/JS. Use Go stdlib AST for Go (zero-cost, already available). Add tree-sitter only if regex proves insufficient for real-world codebases.

### Anti-Pattern 3: MCP Server with Business Logic

**What people do:** Put query logic, graph traversal, or confidence calculations inside MCP tool handlers.
**Why it's wrong:** Duplicates logic. Makes CLI and MCP diverge. Violates DRY.
**Do this instead:** MCP handlers are thin adapters. They validate input, call `internal/knowledge` functions, and format output. All logic stays in knowledge.

### Anti-Pattern 4: Breaking the Existing Package Structure

**What people do:** Split `internal/knowledge` into `internal/knowledge/graph`, `internal/knowledge/query`, etc. to "improve organization" alongside v2.0 changes.
**Why it's wrong:** Massive churn for no user-facing value. Risk of breaking existing tests. The package is cohesive — graph, query, and persistence are tightly coupled by design.
**Do this instead:** Add new packages (`internal/code`, `internal/mcp`) alongside existing structure. Modify existing files minimally.

## Build Order (Dependency-Ordered)

The following build order respects dependencies — each step builds on what came before:

| Order | Component | Depends On | Rationale |
|-------|-----------|------------|-----------|
| 1 | `internal/code/signal.go` | Nothing | Define CodeSignal type first — all parsers produce this |
| 2 | `internal/code/analyzer.go` | signal.go | LanguageParser interface, CodeAnalyzer orchestrator |
| 3 | `internal/code/goparser/` | analyzer.go | Go parser (stdlib only, no external deps) |
| 4 | `internal/code/connstring/` | signal.go | Connection string detector (regex, no parser needed) |
| 5 | `internal/code/pyparser/` | analyzer.go | Python parser (regex) |
| 6 | `internal/code/jsparser/` | analyzer.go | JS/TS parser (regex) |
| 7 | Signal integration into knowledge | code package | Extend discovery_orchestration.go, algo_aggregator.go |
| 8 | Schema v6 migration | db.go | Add source_type column, code_signals table |
| 9 | Export pipeline integration | Steps 7-8 | Wire code analysis into CmdExport |
| 10 | `internal/mcp/server.go` | knowledge (query) | MCP server wrapping existing query interface |
| 11 | `cmd/graphmd` mcp command | internal/mcp | CLI entry point for `graphmd mcp` |

**Why this order:**
- Steps 1-2 are foundational types with no dependencies
- Steps 3-6 are independent parsers that can be parallelized
- Steps 7-8 are the integration point (must follow parsers)
- Steps 9 is the export pipeline update (must follow schema)
- Steps 10-11 are MCP (independent of code analysis, but ordered last because it queries the result of code analysis)

## Sources

- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) — server creation, tool registration, transport (HIGH confidence)
- [MCP Go SDK package docs](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — API reference (HIGH confidence)
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) — community MCP implementation, spec version compatibility (MEDIUM confidence)
- [go/ast package](https://pkg.go.dev/go/ast) — Go standard library AST (HIGH confidence)
- [tree-sitter Go bindings](https://github.com/tree-sitter/go-tree-sitter) — multi-language parsing option (MEDIUM confidence)
- [xo/dburl](https://pkg.go.dev/github.com/xo/dburl) — DSN/URL parsing patterns for connection string detection (HIGH confidence)
- [2026 MCP Roadmap](http://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/) — Streamable HTTP transport direction (MEDIUM confidence)
- Existing codebase analysis: `internal/knowledge/*.go` (~30K LOC) — direct code reading (HIGH confidence)

---
*Architecture research for: graphmd v2.0 code analysis + MCP integration*
*Researched: 2026-03-29*
