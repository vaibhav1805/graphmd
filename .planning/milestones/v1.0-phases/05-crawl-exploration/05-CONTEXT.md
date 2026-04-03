# Phase 5: Crawl Exploration - Context

**Gathered:** 2026-03-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Provide a local exploration mode for engineers to inspect the dependency graph before export. Displays component inventory, type distribution, relationship quality metrics, and confidence distribution. This is a pre-export diagnostic tool — engineers use it to assess whether the graph is worth exporting.

</domain>

<decisions>
## Implementation Decisions

### Stats Output & Information Density

- **Detail level:** Full detail by default — show complete component listing and relationship stats, not just summary counts
- **Component grouping:** Group components by type (e.g., "Services: payment-api, user-service / Databases: primary-db")
- **Output format:** Text default, `--format json` available for CI pipelines and scripts
- **Quality warnings:** Include a quality warnings section highlighting orphan nodes (no relationships), dangling edge references, and components with only weak-confidence edges

### Confidence Distribution Display

- **Text format:** Counts + percentages + ASCII bar chart per tier
  - e.g., `strong (0.8-0.9):  23 edges (46%) ████████████`
- **Empty tiers:** Only show tiers that have edges (skip zero-count tiers)
- **Quality score:** Include an overall graph quality indicator — weighted average of confidence scores as a percentage (e.g., "Quality: 78%")
- **JSON format:** Include score ranges for each tier: `{"tier": "strong", "range": [0.8, 0.9], "count": 23, "percentage": 46}`. Self-documenting for scripts

### Crawl Scope & Behavior

- **Ignore patterns:** Respect `.graphmdignore` (same as export) — what you see in crawl is what you get in export
- **Input targeting:** Support `--input` flag for targeting subdirectories (same flag as export)
- **Aliases:** Apply `graphmd-aliases.yaml` aliases (same as export) — crawl output reflects normalized names
- **Discovery algorithms:** Run all algorithms (same as export) — crawl is an accurate preview of what export would produce

### Claude's Discretion

- ASCII bar chart scaling and width
- Quality score weighting formula
- Quality warning thresholds (e.g., what % of weak edges triggers a warning)
- Text output formatting and spacing
- JSON structure nesting

</decisions>

<specifics>
## Specific Ideas

- Crawl should be a "preview of export" — same pipeline, same filters, same normalization, just stats instead of a ZIP
- The quality score gives operators a quick pass/fail signal without needing to interpret raw distributions
- Quality warnings are actionable: "5 orphan nodes found" tells operators their docs need cross-referencing

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-crawl-exploration*
*Context gathered: 2026-03-24*
