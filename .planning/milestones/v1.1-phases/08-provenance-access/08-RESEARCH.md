# Phase 8: Provenance Access - Research

**Researched:** 2026-03-29
**Domain:** Go CLI — surfacing SQLite provenance data in JSON query output
**Confidence:** HIGH

## Summary

Phase 8 surfaces component detection provenance from the existing `component_mentions` SQLite table into impact and dependencies query results. The `component_mentions` table already exists (schema v5), is populated during `graphmd export`, and has indexes on `component_id` and `file_path`. The `SaveComponentMentions` method exists but there is no corresponding `LoadComponentMentions` read method — that is the primary new code. The query layer (`query_cli.go`) needs a new `--include-provenance` flag on impact and dependencies subcommands, a new `--max-mentions` flag for volume control, and the `ImpactNode` struct needs a `Mentions` field with `omitempty` to carry inline provenance data.

The implementation is straightforward: add a `LoadComponentMentions` method to `Database` that returns `map[string][]ComponentMention`, modify `LoadStoredGraph` to optionally load mentions alongside the graph, thread the mentions map into the BFS traversal functions so each `ImpactNode` can be decorated with its provenance, and add the two new CLI flags. The `ComponentMention` struct already has JSON tags and maps directly to the table columns. No schema changes, no new dependencies, no new tables.

**Primary recommendation:** Add `LoadComponentMentions` as a new `Database` method with SQL-level `ORDER BY confidence DESC LIMIT N` per component, thread mentions through `LoadStoredGraph` as a separate return value, and attach to `ImpactNode` via a new `omitempty` field. Keep provenance completely absent from output when `--include-provenance` is not set.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Mention Data Structure:**
- Placement: Inline on each component node in query results. Each component carries its own `mentions` array (e.g., `{"name": "payment-api", "mentions": [...]}`)
- Fields per mention: `file_path`, `detection_method`, `confidence`, `line_number` or `context` (where in the file)
- Omitempty: Yes — `mentions` field is completely absent from JSON without `--include-provenance`. Zero schema change for existing consumers
- Supported commands: `--include-provenance` on impact and dependencies queries only (not list or path)

**Volume Control:**
- Default limit: Top 5 mentions per component, sorted by confidence (highest first)
- Total count indicator: Include `mention_count` field alongside truncated mentions array (e.g., `"mention_count": 23` when showing 5 of 23)
- Override flag: `--max-mentions N` to adjust the limit. `--max-mentions 0` for unlimited

### Claude's Discretion

- How to load component_mentions from the database (new method on Database, separate from LoadGraph)
- Whether to load mentions lazily (per-component) or eagerly (all at once)
- How to integrate mentions into the existing ImpactNode struct
- Table format rendering of mention data (if `--format table` is used with `--include-provenance`)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DEBT-04 | Surface component_mentions data in query results via --include-provenance flag (opt-in to avoid output bloat) | ComponentMention struct exists in db.go with JSON tags; component_mentions table is populated during export; LoadComponentMentions read method is missing and must be created; ImpactNode struct needs Mentions field with omitempty |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `database/sql` | 1.24 | SQL queries for LoadComponentMentions | Already used by all DB methods in db.go |
| Go stdlib `flag` | 1.24 | --include-provenance and --max-mentions flags | Already used by all query subcommands |
| Go stdlib `encoding/json` | 1.24 | JSON serialization with omitempty | Already used for QueryEnvelope output |
| modernc.org/sqlite | 1.46.1 | Pure-Go SQLite driver | Already the project's sole DB driver |

### Supporting
No additional libraries needed.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Eager load (all mentions at once) | Lazy load (per-component SQL query) | Eager is better: single SQL query + index scan vs N+1 queries during BFS. The BFS visits at most ~100 nodes in typical use; loading all mentions for those components in one query is simpler and faster |
| Separate LoadStoredGraph signature | Pass options struct | Options struct adds unnecessary abstraction for a single boolean; separate function or extra return value is cleaner for this scope |

## Architecture Patterns

### Recommended Approach

```
LoadStoredGraph (import.go)
  ├── db.LoadGraph(graph)        [existing]
  └── db.LoadComponentMentions() [NEW - returns map[string][]ComponentMention]

cmdQueryImpact / cmdQueryDependencies (query_cli.go)
  ├── Parse --include-provenance, --max-mentions flags
  ├── LoadStoredGraph() → graph, meta, mentions (or nil)
  ├── executeImpactReverse/executeForwardTraversal() → nodes, rels, cycles
  └── if provenance: decorate each ImpactNode with mentions[node.Name]
```

### Pattern 1: Eager Load with SQL-Level Limiting

**What:** Load all mentions in a single SQL query with `ORDER BY confidence DESC`, then group in Go. Apply per-component limit (default 5) in Go after grouping — this avoids complex SQL windowing while keeping the query simple.

**When to use:** When the mention volume is bounded by the number of components (typically < 100) and each component has < 100 mentions.

```go
// In db.go
func (db *Database) LoadComponentMentions() (map[string][]ComponentMention, error) {
    rows, err := db.conn.Query(
        `SELECT component_id, file_path, heading_hierarchy, detected_by, confidence
         FROM component_mentions
         ORDER BY component_id, confidence DESC`)
    // ... scan rows, group by component_id into map
}
```

### Pattern 2: Mention Decoration After BFS

**What:** After BFS traversal produces `[]ImpactNode`, iterate over the nodes and attach mentions from the pre-loaded map. This keeps the BFS functions unchanged — provenance is a post-processing step.

**When to use:** Always — avoids modifying BFS traversal logic, which was recently updated for cycle detection.

```go
// After BFS returns nodes:
if includeProv && mentions != nil {
    for i := range nodes {
        if ms, ok := mentions[nodes[i].Name]; ok {
            limited := ms
            if maxMentions > 0 && len(limited) > maxMentions {
                limited = limited[:maxMentions]
            }
            nodes[i].Mentions = limited
            nodes[i].MentionCount = len(ms) // total before truncation
        }
    }
}
```

### Pattern 3: ImpactNode Extension with omitempty

**What:** Add `Mentions` and `MentionCount` fields to `ImpactNode` with `json:",omitempty"` tags so they are completely absent from JSON when not populated.

```go
type ImpactNode struct {
    Name           string              `json:"name"`
    Type           string              `json:"type"`
    Distance       int                 `json:"distance"`
    ConfidenceTier string              `json:"confidence_tier"`
    Mentions       []MentionDetail     `json:"mentions,omitempty"`
    MentionCount   int                 `json:"mention_count,omitempty"`
}

// MentionDetail is the per-mention provenance record in query output.
type MentionDetail struct {
    FilePath        string  `json:"file_path"`
    DetectionMethod string  `json:"detection_method"`
    Confidence      float64 `json:"confidence"`
    Context         string  `json:"context,omitempty"`
}
```

### Pattern 4: LoadStoredGraph Signature Extension

**What:** Add mentions as a third return value to `LoadStoredGraph`, or create a new `LoadStoredGraphWithMentions` function.

**Recommendation:** Modify `LoadStoredGraph` to return `(*Graph, *ExportMetadata, map[string][]ComponentMention, error)`. This keeps all data loading in one place. The mentions map is nil-safe — callers that don't need mentions simply ignore the third return value.

**Alternative:** Keep `LoadStoredGraph` unchanged and have the query commands call `LoadComponentMentions` separately when `--include-provenance` is set. This avoids touching `LoadStoredGraph` callers (import validation in `ImportZIP` also calls it). **This alternative is preferred** — it avoids changing a function signature that has multiple callers, and only loads mentions when the flag is set.

### Anti-Patterns to Avoid

- **Modifying BFS traversal to carry mentions:** The BFS functions (`executeImpactReverse`, `executeForwardTraversal`) were recently updated for cycle detection. Adding mention logic inside BFS creates coupling and makes both features harder to test. Decorate after BFS instead.

- **Using SQL window functions (ROW_NUMBER):** modernc.org/sqlite supports window functions, but the per-component limiting is simple enough to do in Go. SQL windowing adds complexity for no meaningful performance gain with < 100 components.

- **Loading mentions inside LoadGraph:** The Graph struct represents topology. Mentions are metadata about detection provenance. Mixing them violates the existing separation between graph topology and detection metadata.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQL-level per-group limiting | Custom SQL with ROW_NUMBER() OVER | Go-side slice truncation after ORDER BY confidence DESC | Simpler, debuggable, no window function dependency |
| JSON field conditional inclusion | Custom MarshalJSON | `json:",omitempty"` struct tags | Standard Go pattern; ImpactNode already uses it for other fields |
| Mention deduplication | Custom dedup logic | Rely on SaveComponentMentions' DELETE-then-INSERT pattern | Data is already deduplicated at write time |

**Key insight:** The component_mentions table is already populated and indexed. This phase is purely a read-path feature — no write-path changes needed.

## Common Pitfalls

### Pitfall 1: Breaking Existing JSON Contract

**What goes wrong:** Adding `mentions` or `mention_count` fields that appear in default output (without `--include-provenance`) changes the JSON contract for AI agent consumers.
**Why it happens:** Forgetting `omitempty` on the new fields, or using a non-zero default for `MentionCount`.
**How to avoid:** Use `json:",omitempty"` on both `Mentions` and `MentionCount`. Since `MentionCount` is `int`, `omitempty` will suppress it when zero — which is the correct behavior when provenance is not requested.
**Warning signs:** Existing query tests start producing JSON with unexpected fields.

### Pitfall 2: N+1 Query Pattern for Mentions

**What goes wrong:** Loading mentions per-component during BFS traversal generates one SQL query per visited node.
**Why it happens:** Natural impulse to load mentions lazily as each node is processed.
**How to avoid:** Load all mentions eagerly in a single query, group into a map, then look up by component ID during decoration.
**Warning signs:** Query latency increases linearly with graph size.

### Pitfall 3: Column Name Mismatch Between DB and Output

**What goes wrong:** The `component_mentions` table uses `detected_by` but the CONTEXT.md specifies `detection_method` in the output JSON.
**Why it happens:** The DB column name and the user-facing JSON field name differ.
**How to avoid:** Use a separate `MentionDetail` struct for output (with `detection_method` JSON tag) distinct from the `ComponentMention` struct (which has `detected_by` JSON tag). Map between them when decorating nodes.
**Warning signs:** JSON output uses `detected_by` instead of `detection_method` as specified by user.

### Pitfall 4: Missing `context` or `line_number` Data

**What goes wrong:** The CONTEXT.md specifies that each mention should include `line_number` or `context` — but the `component_mentions` table has `heading_hierarchy` as the closest field, and no `line_number` column.
**Why it happens:** The DB schema was designed before provenance access requirements were finalized.
**How to avoid:** Map `heading_hierarchy` to the `context` field in output. This provides meaningful location information ("## Dependencies > ### Database") without requiring a schema migration to add line numbers.
**Warning signs:** `context` field is always empty in output because the code looks for a nonexistent `line_number` column.

### Pitfall 5: Changing LoadStoredGraph Signature

**What goes wrong:** Modifying `LoadStoredGraph` to return mentions as an additional value breaks all existing callers (cmdQueryImpact, cmdQueryDependencies, cmdQueryPath, cmdQueryList, ImportZIP validation).
**Why it happens:** Temptation to load everything in one function.
**How to avoid:** Keep `LoadStoredGraph` unchanged. When `--include-provenance` is set, open the DB separately and call `LoadComponentMentions` directly. The DB path is determinable from the same graph name resolution logic.
**Warning signs:** Compilation errors across multiple files after changing LoadStoredGraph.

## Code Examples

### LoadComponentMentions Method

```go
// In db.go — new read method
func (db *Database) LoadComponentMentions() (map[string][]ComponentMention, error) {
    rows, err := db.conn.Query(
        `SELECT component_id, file_path, heading_hierarchy, detected_by, confidence
         FROM component_mentions
         ORDER BY component_id, confidence DESC`)
    if err != nil {
        return nil, fmt.Errorf("load component mentions: %w", err)
    }
    defer rows.Close()

    result := make(map[string][]ComponentMention)
    for rows.Next() {
        var m ComponentMention
        if err := rows.Scan(&m.ComponentID, &m.FilePath, &m.HeadingHierarchy, &m.DetectedBy, &m.Confidence); err != nil {
            return nil, fmt.Errorf("scan component mention: %w", err)
        }
        result[m.ComponentID] = append(result[m.ComponentID], m)
    }
    return result, rows.Err()
}
```

### Flag Parsing in cmdQueryImpact

```go
// Add to existing flag set in cmdQueryImpact:
includeProv := fs.Bool("include-provenance", false, "Include detection provenance for each component")
maxMentions := fs.Int("max-mentions", 5, "Maximum mentions per component (0 for unlimited)")
```

### Mention Decoration After BFS

```go
// After BFS and before envelope creation:
if *includeProv {
    mentions, err := loadMentionsForGraph(*graphName)
    if err != nil {
        // Non-fatal: log warning, continue without mentions
        fmt.Fprintf(os.Stderr, "Warning: could not load provenance: %v\n", err)
    } else {
        decorateWithMentions(affectedNodes, mentions, *maxMentions)
    }
}

func decorateWithMentions(nodes []ImpactNode, mentions map[string][]ComponentMention, limit int) {
    for i := range nodes {
        ms, ok := mentions[nodes[i].Name]
        if !ok {
            continue
        }
        nodes[i].MentionCount = len(ms)
        if limit > 0 && len(ms) > limit {
            ms = ms[:limit]
        }
        details := make([]MentionDetail, len(ms))
        for j, m := range ms {
            details[j] = MentionDetail{
                FilePath:        m.FilePath,
                DetectionMethod: m.DetectedBy,
                Confidence:      m.Confidence,
                Context:         m.HeadingHierarchy,
            }
        }
        nodes[i].Mentions = details
    }
}
```

### Table Format for Provenance

```go
// In writeTable, extend the impact/dependencies case:
if hasProvenance(result) {
    fmt.Fprintln(tw, "\nPROVENANCE:")
    fmt.Fprintln(tw, "COMPONENT\tFILE\tMETHOD\tCONFIDENCE")
    for _, n := range result.AffectedNodes {
        for _, m := range n.Mentions {
            fmt.Fprintf(tw, "%s\t%s\t%s\t%.2f\n", n.Name, m.FilePath, m.DetectionMethod, m.Confidence)
        }
    }
    tw.Flush()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Mentions write-only (export saves, nothing reads) | Read + surface via --include-provenance | This phase | AI agents can answer "where was this component detected?" |
| Graph + metadata only from LoadStoredGraph | Graph + metadata + optional mentions | This phase | Richer query responses with detection evidence |

**Deprecated/outdated:**
- None — this is greenfield read-path work on existing data

## Open Questions

1. **How to load the DB for mentions when LoadStoredGraph closes it?**
   - What we know: `LoadStoredGraph` opens the DB, loads the graph, and closes the DB before returning. The `--include-provenance` flag needs to load mentions from the same DB.
   - What's unclear: Whether to re-open the DB after `LoadStoredGraph` returns, or refactor to keep it open.
   - Recommendation: The cleanest approach is to have `LoadStoredGraph` return the DB path (or expose a helper that resolves graph name to DB path), then open a second DB connection in the query command when `--include-provenance` is set. Alternatively, create a `LoadStoredGraphWithMentions` variant that loads both before closing. The latter avoids double-open overhead but adds a new function. Either works; the planner should choose based on code simplicity.

2. **Root node inclusion in provenance**
   - What we know: The BFS traversal skips the root node (it's the query target, not an "affected" node). The root doesn't appear in `affectedNodes`.
   - What's unclear: Should the root component's provenance be included somewhere in the output?
   - Recommendation: Do not include root provenance in the initial implementation. The user asked for provenance on affected nodes. If needed later, it can be added as a separate field on the envelope.

## Sources

### Primary (HIGH confidence)
- graphmd codebase, `internal/knowledge/db.go` (lines 176-188) — component_mentions table schema with component_id, file_path, heading_hierarchy, detected_by, confidence columns
- graphmd codebase, `internal/knowledge/db.go` (lines 1053-1092) — ComponentMention struct and SaveComponentMentions write method
- graphmd codebase, `internal/knowledge/query_cli.go` (lines 104-116, 513-582, 588-657) — ImpactNode struct and BFS traversal functions
- graphmd codebase, `internal/knowledge/import.go` (lines 181-225) — LoadStoredGraph loads graph but not mentions, closes DB before return
- graphmd codebase, `internal/knowledge/export.go` (lines 185-242) — mentions are populated during export

### Secondary (MEDIUM confidence)
- Milestone-level research summary (`.planning/research/SUMMARY.md`) — confirms architectural approach and pitfalls for mention surfacing

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all patterns already in codebase
- Architecture: HIGH — based on direct code reading of all affected files (db.go, query_cli.go, import.go)
- Pitfalls: HIGH — pitfalls derived from actual code analysis (column name mismatch, missing line_number, LoadStoredGraph closure pattern)

**Research date:** 2026-03-29
**Valid until:** 2026-04-28 (stable — no external dependencies, internal codebase only)
