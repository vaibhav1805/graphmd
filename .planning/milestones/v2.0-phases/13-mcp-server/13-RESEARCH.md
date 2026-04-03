# Phase 13: MCP Server - Research

**Researched:** 2026-04-03
**Domain:** MCP server implementation wrapping existing query interface via official Go SDK
**Confidence:** HIGH

## Summary

Phase 13 wraps graphmd's existing query interface as 5 MCP tools using the official Go SDK (`github.com/modelcontextprotocol/go-sdk@v1.4.1`) with stdio transport. The MCP server is a thin adapter layer — tool handlers validate input, call existing query/traversal functions in `internal/knowledge`, and return the JSON envelope as text content. No query logic lives in the MCP layer.

The primary architectural challenge is that the current query execution logic in `query_cli.go` is tightly coupled to CLI flag parsing and stdout rendering. The core traversal functions (`executeImpactReverse`, `executeForwardTraversal`) are **unexported** (lowercase), and the path/list logic is embedded directly in CLI handler functions. The MCP layer cannot call these directly from a separate package. The implementation must refactor these into exported, CLI-independent functions that both the CLI and MCP layers can call.

The dominant risk is stdout pollution corrupting the MCP stdio transport. The SDK communicates JSON-RPC over stdin/stdout, so any stray `fmt.Println` or `log.Print` (which defaults to stderr but is worth verifying) breaks the protocol. The graphmd codebase has several places that write to stdout (e.g., `outputEnvelope` uses `fmt.Println`). The MCP command must redirect `os.Stdout` to `os.Stderr` or equivalent before starting the server.

**Primary recommendation:** Refactor `query_cli.go` to export core query execution functions (separate from CLI flag parsing and output formatting), then build `internal/mcp/` as a thin adapter that calls those exported functions and wraps results as MCP tool responses.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

User delegated all implementation decisions to Claude. The following guidelines from milestone research should be followed:

**MCP Server Design:**
- Use official Go SDK: `github.com/modelcontextprotocol/go-sdk/mcp` (v1.4.1+)
- Stdio transport for v2.0 (Streamable HTTP deferred to v2.1)
- `graphmd mcp` CLI command starts the server
- New package: `internal/mcp/` — thin adapter wrapping existing query functions

**5 MCP Tools:**
1. `query_impact` — wraps `graphmd query impact --component X`
2. `query_dependencies` — wraps `graphmd query dependencies --component X`
3. `query_path` — wraps `graphmd query path --from A --to B`
4. `list_components` — wraps `graphmd query list`
5. `graphmd_graph_info` — returns graph metadata (name, version, component count, relationship count)

**Tool Design Principles:**
- Tool responses match equivalent CLI query output (JSON envelope with query, results, metadata)
- Tool descriptions should be clear enough for LLM agents to select the right tool
- Parameters mirror CLI flags where possible (component, from, to, depth, min-confidence, source-type, format)
- Default graph used when no graph parameter specified

**Critical Constraint:**
- **No stdout pollution** — all logging goes to stderr. Only MCP JSON-RPC protocol messages on stdout. Research flagged SDK issue #572.
- Graceful SIGTERM shutdown without corrupting state

### Claude's Discretion

User delegated all implementation decisions to Claude.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MCP-01 | MCP server with stdio transport wrapping query interface — 5 tools: impact, dependencies, path, list, graph_info | SDK API fully documented (AddTool, StdioTransport, Server.Run); query functions identified in query_cli.go; refactoring approach defined for extracting reusable query logic |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.4.1 | MCP server + stdio transport | Official SDK maintained by Anthropic + Google; v1.0 stability guarantee; auto JSON Schema from Go structs; supports stdio transport |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/signal` | stdlib | SIGTERM/SIGINT handling | Graceful shutdown of MCP server |
| `context` | stdlib | Cancellation propagation | Server.Run context; tool handler timeouts |
| `encoding/json` | stdlib | JSON marshaling for tool results | Tool responses return JSON text content |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Official MCP Go SDK | `github.com/mark3labs/mcp-go` | Higher-level API, less boilerplate, but no Google backing or v1.0 stability guarantee. Official SDK is the right choice for a tool whose value is reliability. |

**Installation:**
```bash
go get github.com/modelcontextprotocol/go-sdk@v1.4.1
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── mcp/
│   ├── server.go        # NewServer(), Run(), signal handling, stdout guard
│   ├── tools.go         # 5 tool registrations + handler functions
│   └── tools_test.go    # Tool handler unit tests
├── knowledge/
│   ├── query_cli.go     # CLI layer (flag parsing + output) — calls query_exec
│   ├── query_exec.go    # NEW: exported query execution functions
│   └── ...
cmd/graphmd/
│   └── main.go          # Add "mcp" case to command switch
```

### Pattern 1: Extract Query Execution Layer

**What:** Separate CLI flag parsing/output from core query logic. Create exported functions in `internal/knowledge/` that the MCP layer can import.

**When to use:** Always — this is the prerequisite for the entire MCP implementation.

**Why needed:** Currently `cmdQueryImpact()` does flag parsing, graph loading, traversal, and JSON output all in one function. The traversal functions (`executeImpactReverse`, `executeForwardTraversal`) are unexported. Path finding and list logic are inline in their CLI handlers. The MCP package (`internal/mcp/`) cannot call unexported functions in `internal/knowledge/`.

**Approach:** Create a new file `query_exec.go` with exported functions:

```go
// query_exec.go — exported query execution functions

// QueryImpactParams holds parameters for impact query execution.
type QueryImpactParams struct {
    Component     string
    Depth         int
    MinConfidence float64
    SourceType    string
    GraphName     string
    IncludeProvenance bool
    MaxMentions   int
}

// ExecuteImpactQuery runs an impact query and returns the envelope.
func ExecuteImpactQuery(params QueryImpactParams) (*QueryEnvelope, error) {
    // Load graph, validate component, call executeImpactReverse, build envelope
}

// ExecuteDependenciesQuery runs a dependencies query and returns the envelope.
func ExecuteDependenciesQuery(params QueryDependenciesParams) (*QueryEnvelope, error) { ... }

// ExecutePathQuery runs a path query and returns the envelope.
func ExecutePathQuery(params QueryPathParams) (*QueryEnvelope, error) { ... }

// ExecuteListQuery runs a list query and returns the envelope.
func ExecuteListQuery(params QueryListParams) (*QueryEnvelope, error) { ... }

// GraphInfo returns metadata about the currently loaded graph.
func GraphInfo(graphName string) (*GraphInfoResult, error) { ... }
```

Then refactor `cmdQueryImpact()` etc. to parse flags and call these exported functions.

### Pattern 2: MCP Tool Registration with Typed Input Structs

**What:** Use the SDK's generic `mcp.AddTool[In, Out]` to auto-generate JSON Schema from Go structs.

**When to use:** For all 5 tools.

**Example:**
```go
// Source: pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp

type ImpactArgs struct {
    Component     string  `json:"component" jsonschema:"required,the component to analyze for downstream impact"`
    Depth         int     `json:"depth" jsonschema:"traversal depth (default 1, use 0 for unlimited)"`
    MinConfidence float64 `json:"min_confidence" jsonschema:"minimum confidence threshold (0.0-1.0)"`
    SourceType    string  `json:"source_type" jsonschema:"filter by detection source: markdown, code, or both"`
    Graph         string  `json:"graph" jsonschema:"named graph to query (default: most recent import)"`
}

mcp.AddTool(server, &mcp.Tool{
    Name:        "query_impact",
    Description: "Find all components that would be affected if the specified component fails. Returns downstream dependents with confidence scores and relationship details.",
}, func(ctx context.Context, req *mcp.CallToolRequest, args ImpactArgs) (*mcp.CallToolResult, any, error) {
    envelope, err := knowledge.ExecuteImpactQuery(knowledge.QueryImpactParams{
        Component:     args.Component,
        Depth:         args.Depth,
        MinConfidence: args.MinConfidence,
        SourceType:    args.SourceType,
        GraphName:     args.Graph,
    })
    if err != nil {
        return nil, nil, err
    }
    data, _ := json.MarshalIndent(envelope, "", "  ")
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: string(data)},
        },
    }, nil, nil
})
```

### Pattern 3: Stdout Guard for Stdio Transport

**What:** Redirect os.Stdout to os.Stderr before starting the MCP server to prevent accidental stdout pollution.

**When to use:** In the `graphmd mcp` command handler, before `server.Run()`.

**Example:**
```go
func startMCPServer() error {
    // CRITICAL: Redirect stdout to stderr to prevent pollution.
    // The MCP SDK will use its own mechanism for stdio transport.
    // Any fmt.Println, log.Print, etc. from library code goes to stderr.
    origStdout := os.Stdout
    os.Stdout = os.Stderr
    log.SetOutput(os.Stderr)

    server := mcp.NewServer(&mcp.Implementation{
        Name:    "graphmd",
        Version: "2.0.0",
    }, nil)

    // Register tools...

    // Restore stdout for the transport (SDK needs it).
    os.Stdout = origStdout

    // Handle graceful shutdown.
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    return server.Run(ctx, &mcp.StdioTransport{})
}
```

**Important:** The SDK's `StdioTransport` reads from `os.Stdin` and writes to `os.Stdout`. The guard must redirect stdout *before* any tool registration code runs (to catch init-time prints), then the transport itself needs the real stdout. The exact mechanism needs testing — the SDK may capture `os.Stdout` at `StdioTransport{}` construction time or at `Run()` time. Verify which during implementation.

### Pattern 4: Graceful SIGTERM Shutdown

**What:** Use `signal.NotifyContext` to create a cancellable context for `server.Run()`.

**When to use:** Always — required by CONTEXT.md.

**Note:** SDK issue #224 reports that `Server.Run` may not stop cleanly when context is cancelled. The MCP spec says for stdio: client closes stdin, server exits; or client sends SIGTERM. Test that context cancellation actually stops `server.Run()` during implementation. If not, may need to close stdin/stdout manually in the signal handler.

### Anti-Patterns to Avoid

- **MCP server shelling out to CLI:** Never `exec.Command("graphmd", "query", ...)`. Call Go functions directly. Shelling out adds latency, loses type safety, and creates a brittle interface.
- **Business logic in MCP handlers:** Tool handlers should only: validate MCP-specific concerns, call an exported knowledge function, format the result. No graph traversal, no filtering, no provenance decoration in the MCP layer.
- **Returning errors as tool errors vs. tool results:** MCP distinguishes between transport errors (tool handler returns error) and tool-level errors (tool returns result with isError=true). User-facing errors like "component not found" should be tool results, not handler errors. Handler errors indicate infrastructure failure.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema for tool inputs | Manual schema construction | `mcp.AddTool[In, Out]` generic + struct tags | SDK auto-generates schema from Go struct + `jsonschema` tags. Manual schemas drift from code. |
| Stdio JSON-RPC framing | Custom stdin/stdout reader | `mcp.StdioTransport{}` | Protocol framing is tricky (content-length headers, newline handling). SDK handles it. |
| Tool input validation | Manual type checking of `req.Params.Arguments` | SDK's typed handler + struct validation | SDK unmarshals and validates against schema before handler is called. |
| Signal handling boilerplate | Custom signal channel management | `signal.NotifyContext()` | Stdlib function, one line, correct behavior. |

**Key insight:** The MCP server is genuinely a thin adapter. All the hard work (graph loading, traversal, confidence scoring, provenance) already exists in `internal/knowledge`. The MCP layer is ~200-300 lines of registration + handler code, plus the query extraction refactor.

## Common Pitfalls

### Pitfall 1: Stdout Pollution Breaks Stdio Transport
**What goes wrong:** Any write to `os.Stdout` that isn't a valid MCP JSON-RPC message corrupts the transport. The client receives malformed data and disconnects.
**Why it happens:** Go's `fmt.Println`, `log.Print` (if log output not redirected), and library code that assumes stdout is a terminal. The existing `outputEnvelope` function in `query_cli.go` writes to stdout via `fmt.Println`.
**How to avoid:** (1) Redirect `os.Stdout` to `os.Stderr` at MCP server startup. (2) Ensure the extracted query functions return data, not write to stdout. (3) Set `log.SetOutput(os.Stderr)`. (4) Add a test that verifies no non-MCP data appears on stdout during tool execution.
**Warning signs:** MCP client disconnects immediately or returns parse errors. Tool calls that work via CLI fail via MCP.

### Pitfall 2: Coupling MCP Handlers to CLI Query Functions
**What goes wrong:** MCP tool handlers call `cmdQueryImpact([]string{...})` by constructing fake CLI args. This is fragile, hard to test, and breaks when CLI flags change.
**Why it happens:** The query logic is currently only accessible through CLI handler functions that parse flags and write to stdout.
**How to avoid:** Extract query execution into exported functions that accept typed params and return typed results. Both CLI and MCP call these functions.
**Warning signs:** MCP tests that construct string slices of fake CLI arguments.

### Pitfall 3: Unexported Functions Block Cross-Package Access
**What goes wrong:** `internal/mcp/` cannot call `executeImpactReverse()` or `executeForwardTraversal()` because they're unexported (lowercase) in `internal/knowledge/`.
**Why it happens:** The functions were designed for intra-package use by CLI handlers.
**How to avoid:** Create exported wrapper functions (see Pattern 1 above). The refactoring is mechanical — the logic stays the same, just wrapped in exported functions with typed parameter structs.
**Warning signs:** Compilation errors when `internal/mcp/` tries to call `internal/knowledge` functions.

### Pitfall 4: Error Handling — Tool Errors vs. Transport Errors
**What goes wrong:** Returning all errors from the handler function causes MCP clients to see infrastructure errors for user-level issues like "component not found."
**Why it happens:** Conflating "the tool couldn't find the component" (user error, should be tool result) with "the database is corrupted" (infrastructure error, should be handler error).
**How to avoid:** Return user-facing errors as `CallToolResult` with `IsError: true` and a descriptive text content. Only return Go errors from the handler for actual infrastructure failures (can't open DB, can't load graph).
**Warning signs:** MCP clients showing raw Go error messages instead of structured error JSON.

### Pitfall 5: Graph Loading on Every Tool Call
**What goes wrong:** Each tool call runs `LoadStoredGraph()` which opens the SQLite DB, loads all nodes and edges into memory, then closes the DB. For a large graph queried frequently, this is wasteful.
**Why it happens:** The CLI model is one-shot: load graph, query, exit. MCP server is long-running.
**How to avoid:** For v2.0, loading per-call is acceptable (graphs are small, SQLite is fast). Document that caching is a v2.1 optimization. If performance becomes an issue, cache the loaded graph in the server and invalidate on a timer or signal.
**Warning signs:** High latency on tool calls with large graphs. For now, accept this tradeoff.

## Code Examples

### Complete MCP Server Setup
```go
// Source: verified against pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp

package mcp

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/graphmd/graphmd/internal/knowledge"
)

func NewGraphMDServer() *mcp.Server {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "graphmd",
        Version: "2.0.0",
    }, nil)

    registerTools(server)
    return server
}

func Run() error {
    // Guard stdout.
    log.SetOutput(os.Stderr)

    server := NewGraphMDServer()

    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    return server.Run(ctx, &mcp.StdioTransport{})
}
```

### Tool Handler Returning Structured Error
```go
func handleImpact(ctx context.Context, req *mcp.CallToolRequest, args ImpactArgs) (*mcp.CallToolResult, any, error) {
    if args.Component == "" {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: `{"error":"component is required","code":"MISSING_ARG"}`},
            },
            IsError: true,
        }, nil, nil  // nil error — this is a tool-level error, not infrastructure
    }

    envelope, err := knowledge.ExecuteImpactQuery(knowledge.QueryImpactParams{
        Component: args.Component,
        Depth:     args.Depth,
        // ...
    })
    if err != nil {
        // Infrastructure error — return as handler error
        return nil, nil, fmt.Errorf("execute impact query: %w", err)
    }

    data, _ := json.MarshalIndent(envelope, "", "  ")
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: string(data)},
        },
    }, nil, nil
}
```

### Tool Description Best Practices
```go
// Source: MCP best practices — tool descriptions significantly affect agent behavior

mcp.AddTool(server, &mcp.Tool{
    Name: "query_impact",
    Description: `Analyze downstream impact of a component failure. Given a component name,
returns all components that directly or transitively depend on it, with confidence
scores indicating detection certainty. Use this to answer "if X fails, what breaks?"`,
}, handleImpact)

mcp.AddTool(server, &mcp.Tool{
    Name: "query_dependencies",
    Description: `Find what a component depends on. Given a component name, returns all
components it requires to function, with confidence scores. Use this to answer
"what does X need to work?"`,
}, handleDependencies)

mcp.AddTool(server, &mcp.Tool{
    Name: "query_path",
    Description: `Find dependency paths between two components. Returns all paths from
source to target with per-hop confidence scores. Use this to answer "how does X
connect to Y?"`,
}, handlePath)

mcp.AddTool(server, &mcp.Tool{
    Name: "list_components",
    Description: `List all components in the dependency graph with optional type and
confidence filters. Returns component names, types, and edge counts. Use this to
explore the graph before targeted queries.`,
}, handleListComponents)

mcp.AddTool(server, &mcp.Tool{
    Name: "graphmd_graph_info",
    Description: `Get metadata about the loaded dependency graph: name, version, creation
date, component count, and relationship count. Use this first to verify a graph is
loaded and assess its scope before querying.`,
}, handleGraphInfo)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mcp-go` (mark3labs) community SDK | Official `modelcontextprotocol/go-sdk` | Jan 2025 | Official SDK with Google backing, v1.0 stability guarantee |
| Manual JSON Schema construction | `mcp.AddTool[In, Out]` generic with struct tags | go-sdk v1.0.0 | Auto-generates schema from Go structs, validates input automatically |
| SSE transport for remote access | Streamable HTTP transport | MCP spec 2025-11-25 | SSE deprecated in favor of Streamable HTTP; stdio remains standard for local |

**Deprecated/outdated:**
- SSE transport: Deprecated in MCP spec 2025-11-25. Use stdio (local) or Streamable HTTP (remote, v2.1).
- Manual tool input parsing via `req.Params.Arguments`: Use typed generic handlers instead.

## Open Questions

1. **StdioTransport stdout capture timing**
   - What we know: `StdioTransport{}` reads from stdin and writes to stdout for MCP messages.
   - What's unclear: Does the SDK capture `os.Stdout` at struct creation time, or at `Run()` time? This affects when it's safe to redirect stdout.
   - Recommendation: Test empirically during implementation. If the SDK captures at creation time, create the transport *before* redirecting stdout. If at `Run()` time, ensure stdout is restored before `Run()`.

2. **Server.Run context cancellation behavior**
   - What we know: SDK issue #224 reports `Server.Run` may not stop when context is cancelled.
   - What's unclear: Whether this was fixed in v1.4.1.
   - Recommendation: Test during implementation. If `Run()` doesn't respond to context cancellation, add a manual stdin close in the signal handler as a fallback.

3. **Error representation in tool results**
   - What we know: The existing CLI writes structured error JSON (`queryErrorJSON` type) to stdout for user errors.
   - What's unclear: Whether MCP best practices prefer `IsError: true` with structured JSON, or returning errors as normal tool results that happen to contain error information.
   - Recommendation: Use `IsError: true` for input validation errors (missing args, invalid values) and `NOT_FOUND` errors. Return infrastructure errors as handler errors. Return empty-but-valid results (e.g., no paths found) as normal tool results.

## Sources

### Primary (HIGH confidence)
- [Official MCP Go SDK](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — v1.4.1 API reference: NewServer, AddTool, StdioTransport, CallToolResult types
- [MCP Go SDK GitHub](https://github.com/modelcontextprotocol/go-sdk) — README examples, version info, transport docs
- Existing codebase `internal/knowledge/query_cli.go` — Direct analysis of query execution functions, type definitions, and output formatting

### Secondary (MEDIUM confidence)
- [MCP Go SDK issue #572](https://github.com/modelcontextprotocol/go-sdk/issues/572) — Stdout pollution gap, open as of Feb 2026, labeled for v2
- [MCP Go SDK issue #224](https://github.com/modelcontextprotocol/go-sdk/issues/224) — Server.Run context cancellation behavior
- [MCP Specification Lifecycle](https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle) — Stdio shutdown protocol (client closes stdin, then SIGTERM, then SIGKILL)

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Official SDK v1.4.1, well-documented API, confirmed via pkg.go.dev
- Architecture: HIGH — Based on direct analysis of existing codebase; refactoring approach is mechanical
- Pitfalls: HIGH — Stdout pollution confirmed by open SDK issue; unexported functions verified by grep; error handling patterns from SDK docs

**Research date:** 2026-04-03
**Valid until:** 2026-05-03 (stable SDK, unlikely to change significantly)
