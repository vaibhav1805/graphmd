# Phase 8: Provenance Access - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Surface component detection provenance (source files, detection methods, confidence) from the component_mentions table in query results via an opt-in `--include-provenance` flag. Only impact and dependencies queries get this flag. Default output remains unchanged.

</domain>

<decisions>
## Implementation Decisions

### Mention Data Structure

- **Placement:** Inline on each component node in query results. Each component carries its own `mentions` array (e.g., `{"name": "payment-api", "mentions": [...]}`)
- **Fields per mention:**
  - `file_path` — source markdown file where the component was mentioned
  - `detection_method` — how it was found (heading, NER, co-occurrence, etc.)
  - `confidence` — detection confidence score for this specific mention
  - `line_number` or `context` — where in the file (line number or surrounding text)
- **Omitempty:** Yes — `mentions` field is completely absent from JSON without `--include-provenance`. Zero schema change for existing consumers
- **Supported commands:** `--include-provenance` on impact and dependencies queries only (not list or path)

### Volume Control

- **Default limit:** Top 5 mentions per component, sorted by confidence (highest first)
- **Total count indicator:** Include `mention_count` field alongside truncated mentions array (e.g., `"mention_count": 23` when showing 5 of 23)
- **Override flag:** `--max-mentions N` to adjust the limit. `--max-mentions 0` for unlimited

### Claude's Discretion

- How to load component_mentions from the database (new method on Database, separate from LoadGraph)
- Whether to load mentions lazily (per-component) or eagerly (all at once)
- How to integrate mentions into the existing ImpactNode struct
- Table format rendering of mention data (if `--format table` is used with `--include-provenance`)

</decisions>

<specifics>
## Specific Ideas

- Research noted: naive JOIN can produce 50+ mention rows per component — the top-5 limit addresses this
- The `mention_count` field lets agents know there's more evidence without receiving it all
- `--max-mentions 0` is the escape hatch for agents that want complete provenance

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 08-provenance-access*
*Context gathered: 2026-03-29*
