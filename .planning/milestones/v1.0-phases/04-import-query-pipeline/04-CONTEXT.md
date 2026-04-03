# Phase 4: Import & Query Pipeline - Context

**Gathered:** 2026-03-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Enable production containers to load an exported graph ZIP and serve four core query patterns (impact, dependencies, path, list) that AI agents need for incident response. This phase delivers the product — the query interface agents depend on.

Import loads and validates ZIPs into persistent storage. Query commands read the stored graph and return structured JSON results with confidence scores and provenance.

</domain>

<decisions>
## Implementation Decisions

### Import Behavior & Validation

- **Storage location:** XDG data directory (`~/.local/share/graphmd/`) for persistent, cross-session graph storage
- **Named graphs:** Support naming graphs at import (`graphmd import graph.zip --name prod-infra`). Most recently imported graph is the default; `--graph <name>` selects a specific one for queries
- **Schema validation:** Fail with clear error message when ZIP has a newer schema version than the binary supports. No best-effort loading
- **Import validation:** Full validation on import — verify ZIP structure, metadata.json parseable, graph.db has expected tables/schema version. Reject bad imports early

### Query Output Contract

- **JSON structure:** Consistent envelope for all query types: `{query, results, metadata}`
  - `query`: type and parameters used
  - `results`: the actual data (components, relationships, paths)
  - `metadata`: timing, counts, graph name, build info (version, created_at)
- **Confidence presentation:** Both numeric score and tier name on every relationship: `{"confidence": 0.85, "confidence_tier": "strong"}`
- **Provenance:** Inline with each relationship (source_file, extraction_method). No cross-referencing needed
- **Graph metadata:** Always included in metadata section of every response (graph name, version, created_at, component_count). Helps agents detect stale graphs
- **Output format:** JSON by default. `--format table` available for human debugging/inspection

### Query Patterns & Flag Design

- **Impact query:** `graphmd query impact --component X`
  - Default: direct dependents only (depth=1)
  - `--depth N` for transitive traversal; `--depth all` for full downstream
- **Dependencies query:** `graphmd query dependencies --component X` (separate subcommand, not --reverse flag)
  - Same depth semantics as impact
  - "What does X need to work?" vs impact's "What breaks if X fails?"
- **Path query:** `graphmd query path --from A --to B`
  - Returns all paths up to a limit
  - Default limit: 10 paths. `--limit N` to adjust
  - Per-hop confidence scores on each path
- **List query:** `graphmd query list`
  - Composable filters: `--type service --min-confidence 0.7` (AND together)
- **Global flags:**
  - `--min-confidence <float>` available on all query types
  - `--graph <name>` to select which imported graph to query (optional; default is most recent)
  - `--format json|table` (default: json)
- **No type filtering on impact/dependencies** — agents get all results and filter client-side

### Error Responses & Edge Cases

- **Unknown component:** JSON error with fuzzy-match suggestions: `{"error": "component not found", "code": "NOT_FOUND", "suggestions": ["foo-service", "foo-api"]}`
- **No path found:** Success with empty paths array + reason field (not an error — valid query, just no result)
- **No graph imported:** JSON error with actionable message: `{"error": "no graph imported", "code": "NO_GRAPH", "action": "run 'graphmd import <file.zip>' first"}`
- **Exit codes:** Consistent exit code 1 for all errors. Agents parse the JSON `code` field for specifics
- **Error envelope:** `{"error": "<message>", "code": "<ERROR_CODE>"}` — consistent across all error types

### Claude's Discretion

- Exact XDG path resolution and cross-platform fallback (macOS/Linux)
- Graph storage file naming and directory structure under XDG
- Table format column layout and styling
- Fuzzy matching algorithm for component name suggestions
- Path-finding algorithm choice (BFS, DFS, Dijkstra)
- How `--depth all` is bounded internally (safety limits for large graphs)

</decisions>

<specifics>
## Specific Ideas

- The query interface is the product — agents will depend on this JSON contract, so it must be stable and predictable
- Named graphs enable multi-environment setups (prod, staging, dev) without re-importing
- Error responses should be actionable: agents should be able to self-correct from error JSON without human intervention
- "No path found" is information, not an error — the response should reflect that

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 04-import-query-pipeline*
*Context gathered: 2026-03-23*
