# Phase 13: MCP Server - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement an MCP server with stdio transport that wraps the existing query interface as 5 tools for LLM agent access. `graphmd mcp` command starts the server. Agents query dependency graphs via MCP tool calls instead of CLI invocation.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

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

</decisions>

<specifics>
## Specific Ideas

- The MCP server is a thin adapter — it translates MCP tool calls to existing query function calls and returns the JSON envelope
- Research confirmed 5 tools is within the MCP best-practices sweet spot (5-15 tools)
- Tool descriptions significantly affect agent behavior — invest in clear, specific descriptions

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 13-mcp-server*
*Context gathered: 2026-04-03*
