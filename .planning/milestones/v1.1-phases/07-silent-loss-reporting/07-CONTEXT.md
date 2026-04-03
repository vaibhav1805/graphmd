# Phase 7: Silent Loss Reporting - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Surface information that was previously silently dropped: cycle detection in CLI query results (DEBT-02) and operator warnings when edges are dropped during export (DEBT-03). Both items make hidden data loss visible without changing core behavior.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

User delegated all implementation decisions to Claude. The following guidelines from research should be followed:

**Cycle reporting (DEBT-02):**
- Detect cycle back-edges inline during existing BFS traversal (no separate algorithm pass)
- Add `cycles_detected` field to JSON envelope metadata with `omitempty`
- Each cycle entry should include the cycle path (component names)
- Keep BFS (correct distances) — do NOT switch to DFS
- Cycle info is informational, not an error — queries still succeed with results
- Both impact and dependencies queries should report cycles

**Edge drop warnings (DEBT-03):**
- Use `fmt.Fprintf(os.Stderr)` matching existing operator messaging patterns
- Print count of dropped edges + list the edge source→target pairs
- Do NOT change SaveGraph return signature — stderr warning is sufficient
- Warning should not break JSON output on stdout
- Consider `--verbose` for detailed edge listing, summary count by default

**Cross-cutting constraints:**
- JSON envelope must remain backward-compatible (all new fields `omitempty`)
- No existing field names, types, or positions may change
- Warnings go to stderr only, never stdout (agents parse stdout)

</decisions>

<specifics>
## Specific Ideas

- Research confirmed: BFS already has a `visited` map — record back-edges when a visited neighbor is encountered (<1% overhead)
- SaveGraph has two `continue` statements at db.go:926-931 that silently skip edges — add counter + warning there
- These two items touch independent files (query_cli.go vs db.go/export.go) and can be implemented in parallel

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 07-silent-loss-reporting*
*Context gathered: 2026-03-29*
