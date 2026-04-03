# Phase 12: Signal Integration - Context

**Gathered:** 2026-04-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Merge code-detected dependency signals with markdown-detected signals into a single hybrid graph. Schema v6 migration adding source_type and code_signals provenance table. Code becomes the 5th discovery source alongside co-occurrence, structural, NER, and semantic. Per-source provenance preserved.

</domain>

<decisions>
## Implementation Decisions

### Confidence Merging Formula

- **Probabilistic OR:** `1 - (1-a)*(1-b)` when both markdown and code detect the same relationship
  - Two independent 0.6 signals → 0.84
  - Mathematically sound: independent evidence compounds
- **Stay within 0.4-1.0 range:** No super-confidence. Consistent with existing 6-tier system. Probabilistic OR naturally stays under 1.0
- **New code-only edges:** Added at their original confidence (no penalty). Code-detected DB connection at 0.85 is valid even without markdown corroboration

### Source Type in Query Results

- **source_type field:** Always present on every relationship — values: `"markdown"`, `"code"`, `"both"`
- **Always present (not omitempty):** Schema v6 adds `source_type TEXT NOT NULL DEFAULT 'markdown'`. Existing edges automatically tagged as markdown
- **--source-type filter:** Available on all query types. Agents can request `--source-type code` to see only code-detected edges, or `--source-type both` for corroborated relationships

### Claude's Discretion

- Schema v6 migration details (ALTER TABLE vs new columns)
- `code_signals` provenance table structure (file, line, language, evidence per signal)
- How code signals feed into the existing `DiscoverAndIntegrateRelationships` pipeline
- Integration point in `CmdExport` and `CmdCrawl` for combining markdown + code graphs
- How `--source-type` filter interacts with existing `--min-confidence` filter

</decisions>

<specifics>
## Specific Ideas

- Research identified this as the highest-risk phase — signal merging is where code and markdown pipelines converge
- The existing `AlgorithmWeight` map and `MergeDiscoveredEdges` infrastructure can be extended additively
- Schema v6 with `DEFAULT 'markdown'` ensures zero data loss on migration from v5

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 12-signal-integration*
*Context gathered: 2026-04-01*
