# Phase 5: Crawl Exploration - Research

**Researched:** 2026-03-24
**Domain:** CLI command implementation, graph statistics, confidence distribution display
**Confidence:** HIGH

## Summary

Phase 5 transforms the existing `crawl` command from a targeted graph traversal tool into a full graph inspection/preview tool. The current `crawl` command (in `cmd/graphmd/main.go` lines 182-261) operates on a pre-indexed `.bmd/knowledge.db` from a specific set of starting files. The new crawl command needs to run the **same pipeline as export** (scan with ignore patterns, load aliases, detect components, run discovery) but instead of packaging a ZIP, it displays statistics, component inventory grouped by type, confidence distribution, and quality warnings.

All building blocks exist: `ScanDirectory` with `ScanConfig`, `LoadGraphmdIgnore`, `LoadAliasConfig`, `NewComponentDetector`, `DiscoverRelationships`, the 6-tier confidence system (`confidence.go`), and `ComponentType` taxonomy (`types.go`). The work is composing these into a new `CmdCrawl` function in the `knowledge` package (following the `CmdExport`/`CmdImport`/`CmdQuery` pattern) and building text + JSON formatters.

**Primary recommendation:** Implement `CmdCrawl` in `internal/knowledge/` following the exact `CmdExport` pipeline (steps 1-7) but replacing the ZIP packaging with stats computation and formatted output. Reuse existing scanning, ignore, alias, detection, and discovery infrastructure. Add confidence distribution computation and quality warning detection as new helper functions.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Detail level:** Full detail by default -- show complete component listing and relationship stats, not just summary counts
- **Component grouping:** Group components by type (e.g., "Services: payment-api, user-service / Databases: primary-db")
- **Output format:** Text default, `--format json` available for CI pipelines and scripts
- **Quality warnings:** Include a quality warnings section highlighting orphan nodes (no relationships), dangling edge references, and components with only weak-confidence edges
- **Text format for confidence:** Counts + percentages + ASCII bar chart per tier (e.g., `strong (0.8-0.9):  23 edges (46%) ||||||||||||`)
- **Empty tiers:** Only show tiers that have edges (skip zero-count tiers)
- **Quality score:** Include overall graph quality indicator -- weighted average of confidence scores as percentage (e.g., "Quality: 78%")
- **JSON format:** Include score ranges for each tier: `{"tier": "strong", "range": [0.8, 0.9], "count": 23, "percentage": 46}`. Self-documenting for scripts
- **Ignore patterns:** Respect `.graphmdignore` (same as export)
- **Input targeting:** Support `--input` flag for targeting subdirectories (same flag as export)
- **Aliases:** Apply `graphmd-aliases.yaml` aliases (same as export)
- **Discovery algorithms:** Run all algorithms (same as export) -- crawl is an accurate preview of what export would produce

### Claude's Discretion
- ASCII bar chart scaling and width
- Quality score weighting formula
- Quality warning thresholds (e.g., what % of weak edges triggers a warning)
- Text output formatting and spacing
- JSON structure nesting

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CRAWL-01 | Implement `crawl` command for local graph exploration (build in-memory graph, display stats) | CmdExport pipeline (export.go steps 1-7) provides the exact scan->detect->discover pipeline to reuse. CmdCrawl follows same pattern as CmdExport/CmdImport/CmdQuery -- function in knowledge package, delegated from main.go |
| CRAWL-02 | Display relationship confidence distribution and summary statistics | 6-tier confidence system (confidence.go) with ScoreToTier(), AllConfidenceTiers(), tier score ranges. Edge.Confidence field on all edges. Existing output.go formatters show pattern for text/JSON dual output |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `flag` | go1.21+ | CLI argument parsing | Same as all other graphmd commands |
| Go stdlib `fmt`/`strings` | go1.21+ | Text formatting, ASCII bar charts | No external dependencies needed |
| Go stdlib `encoding/json` | go1.21+ | JSON output formatting | Same as all other output formatters |
| Go stdlib `sort` | go1.21+ | Deterministic output ordering | Same pattern used in output.go |
| Go stdlib `math` | go1.21+ | Weighted average computation | For quality score calculation |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go stdlib `text/tabwriter` | go1.21+ | Aligned text columns | Already used in query_cli.go for tabular output |

### Alternatives Considered
None -- this phase uses only Go stdlib, consistent with all existing graphmd code.

**Installation:**
No new dependencies required.

## Architecture Patterns

### Recommended Project Structure
```
internal/knowledge/
  crawl_cmd.go        # CmdCrawl function + ParseCrawlArgs (new)
  crawl_stats.go      # Stats computation: confidence distribution, quality score, warnings (new)
  crawl_stats_test.go # Unit tests for stats computation (new)
  crawl_cmd_test.go   # Integration tests for CmdCrawl (new)
  output.go           # Add FormatCrawlStats() text/JSON formatters (extend existing)
  crawl.go            # Existing BFS traversal (untouched)
cmd/graphmd/
  main.go             # Update cmdCrawl() to delegate to knowledge.CmdCrawl (modify existing)
```

### Pattern 1: CmdX Delegation Pattern
**What:** CLI commands in `cmd/graphmd/main.go` delegate to `knowledge.CmdX()` functions that accept `[]string` args.
**When to use:** All new CLI commands.
**Example:**
```go
// In cmd/graphmd/main.go
func cmdCrawl() {
    if err := knowledge.CmdCrawl(os.Args[2:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```
Source: Existing pattern in `cmdExport()`, `cmdImport()`, `cmdQueryMain()` (main.go lines 652-671).

### Pattern 2: Export Pipeline Reuse
**What:** CmdCrawl runs the same steps 1-7 as CmdExport (load ignore, load aliases, scan, build graph, detect components, apply aliases, run discovery) but diverges at step 8 -- instead of saving to DB and packaging ZIP, it computes and displays stats.
**When to use:** This is the core implementation pattern for CRAWL-01.
**Example:**
```go
func CmdCrawl(args []string) error {
    a, err := ParseCrawlArgs(args)
    // Steps 1-7: identical to CmdExport
    // Load .graphmdignore
    // Load alias config
    // Scan directory with ignore patterns
    // Build graph (nodes + link edges)
    // Detect component types
    // Apply aliases
    // Run discovery algorithms

    // Step 8: Compute stats (replaces ZIP packaging)
    stats := ComputeCrawlStats(graph)

    // Step 9: Format and display
    if a.Format == "json" {
        // JSON output
    } else {
        // Text output (default)
    }
    return nil
}
```

### Pattern 3: Confidence Distribution Computation
**What:** Iterate all edges, bucket by tier using `ScoreToTier()`, compute counts and percentages.
**When to use:** CRAWL-02 implementation.
**Example:**
```go
type TierStats struct {
    Tier       ConfidenceTier
    RangeLow   float64
    RangeHigh  float64
    Count      int
    Percentage float64
}

func computeConfidenceDistribution(edges map[string]*Edge) []TierStats {
    tierCounts := make(map[ConfidenceTier]int)
    for _, edge := range edges {
        tier := ScoreToTier(edge.Confidence)
        tierCounts[tier]++
    }
    total := len(edges)
    var stats []TierStats
    for _, tier := range AllConfidenceTiers() {
        count := tierCounts[tier]
        if count == 0 {
            continue // Skip zero-count tiers per user decision
        }
        stats = append(stats, TierStats{
            Tier:       tier,
            RangeLow:   tierLowerBound(tier),
            RangeHigh:  tierUpperBound(tier),
            Count:      count,
            Percentage: float64(count) / float64(total) * 100,
        })
    }
    return stats
}
```

### Pattern 4: Quality Warnings Detection
**What:** Scan graph for structural quality issues: orphan nodes, dangling edges, weak-only components.
**When to use:** Quality warnings section in crawl output.
**Example:**
```go
type QualityWarning struct {
    Type    string   // "orphan_node", "dangling_edge", "weak_only"
    Message string
    Items   []string // affected component IDs
}

func detectQualityWarnings(graph *Graph) []QualityWarning {
    var warnings []QualityWarning

    // Orphan nodes: no incoming or outgoing edges
    var orphans []string
    for id := range graph.Nodes {
        if len(graph.BySource[id]) == 0 && len(graph.ByTarget[id]) == 0 {
            orphans = append(orphans, id)
        }
    }
    if len(orphans) > 0 {
        warnings = append(warnings, QualityWarning{
            Type:    "orphan_node",
            Message: fmt.Sprintf("%d orphan nodes (no relationships)", len(orphans)),
            Items:   orphans,
        })
    }
    // ... dangling edges, weak-only components
    return warnings
}
```

### Anti-Patterns to Avoid
- **Building a separate scan pipeline:** CmdCrawl must use the exact same `ScanDirectory`, `LoadGraphmdIgnore`, `LoadAliasConfig`, `DiscoverRelationships` functions as CmdExport. Do not create separate scanning logic.
- **Hardcoding tier score ranges:** Use the existing `ScoreToTier()` boundaries from `confidence.go`. Derive range bounds from the tier constants, don't duplicate the thresholds.
- **Mutating existing crawl.go:** The existing `CrawlMulti` BFS traversal in `crawl.go` serves a different purpose (targeted traversal from start nodes). The new stats-based crawl is a separate concern. Keep crawl.go untouched.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Confidence tier classification | Custom tier boundaries | `ScoreToTier()` from confidence.go | Tier boundaries are defined once; reuse prevents drift |
| Directory scanning with ignore patterns | Custom file walker | `ScanDirectory()` + `LoadGraphmdIgnore()` | Complex ignore pattern matching already implemented |
| Alias resolution | Custom name mapping | `LoadAliasConfig()` + `applyAliases()` | Lazy reverse lookup with sync.Once already handles edge cases |
| Component type detection | Custom type inference | `NewComponentDetector().DetectComponents()` | 12-type taxonomy with longest-match already implemented |
| Relationship discovery | Custom inference | `DiscoverRelationships()` | Multi-algorithm pipeline with signal aggregation |

**Key insight:** The crawl command is architecturally a "headless export" -- same pipeline, different output. Every graph-building step already exists; the only new code is stats computation and formatting.

## Common Pitfalls

### Pitfall 1: Pipeline Divergence from Export
**What goes wrong:** Crawl produces different results than export because it uses a slightly different pipeline (e.g., forgetting to apply aliases, skipping discovery, missing ignore patterns).
**Why it happens:** Copy-pasting export code and accidentally omitting steps.
**How to avoid:** Extract shared pipeline steps into a helper function if possible, or carefully mirror CmdExport steps 1-7 with clear comments referencing the export counterpart.
**Warning signs:** Crawl shows different component counts than export on the same input directory.

### Pitfall 2: Tier Score Range Boundaries
**What goes wrong:** Displaying incorrect score ranges in the confidence distribution (e.g., showing "strong: 0.8-1.0" when strong-inference is actually 0.75-0.95).
**Why it happens:** Hardcoding range values instead of deriving from `ScoreToTier()` thresholds.
**How to avoid:** Derive range bounds from the actual tier thresholds in confidence.go. The tiers are: explicit (>=0.95), strong-inference (>=0.75), moderate (>=0.55), weak (>=0.45), semantic (>=0.42), threshold (>=0.4).
**Warning signs:** JSON output ranges don't match the actual tier assignment behavior.

### Pitfall 3: ASCII Bar Chart Scaling
**What goes wrong:** Bar charts look wrong when one tier dominates (e.g., 95% in one tier makes all other bars invisible).
**Why it happens:** Linear scaling without considering the display width.
**How to avoid:** Scale bars relative to the maximum count in any tier, not relative to total. Use a fixed max width (e.g., 30 characters) and scale proportionally.
**Warning signs:** Single-character bars that are indistinguishable.

### Pitfall 4: Empty Graph Edge Cases
**What goes wrong:** Division by zero in percentage calculations, nil map panics when graph has no edges or nodes.
**Why it happens:** Not handling the zero-edge or zero-node graph case.
**How to avoid:** Guard all percentage calculations with `if total == 0` checks. Return early with "No components found" message for empty graphs.
**Warning signs:** Panics or NaN output when running crawl on a directory with no markdown files.

### Pitfall 5: Overwriting Existing Crawl Behavior
**What goes wrong:** Breaking the existing `crawl --from-multiple` functionality by replacing cmdCrawl entirely.
**Why it happens:** Not realizing the existing crawl serves a different purpose (targeted BFS traversal) vs. the new full-graph stats mode.
**How to avoid:** The new stats mode should be the **default** behavior when no `--from-multiple` flag is provided. When `--from-multiple` is set, fall back to the existing targeted traversal. Alternatively, route through the same `CmdCrawl` function but branch on whether start files are specified.
**Warning signs:** Existing crawl tests fail after changes.

## Code Examples

### Confidence Distribution with ASCII Bar Chart (Text Format)
```go
func formatConfidenceDistribution(tiers []TierStats, maxBarWidth int) string {
    var sb strings.Builder
    sb.WriteString("Confidence Distribution\n")
    sb.WriteString(strings.Repeat("-", 60) + "\n")

    // Find max count for bar scaling
    maxCount := 0
    for _, t := range tiers {
        if t.Count > maxCount {
            maxCount = t.Count
        }
    }

    for _, t := range tiers {
        barLen := 0
        if maxCount > 0 {
            barLen = t.Count * maxBarWidth / maxCount
            if barLen == 0 && t.Count > 0 {
                barLen = 1 // minimum visible bar
            }
        }
        bar := strings.Repeat("|", barLen)
        fmt.Fprintf(&sb, "  %-20s %3d edges (%4.1f%%) %s\n",
            fmt.Sprintf("%s (%.2f-%.2f):", t.Tier, t.RangeLow, t.RangeHigh),
            t.Count, t.Percentage, bar)
    }
    return sb.String()
}
```
Source: Follows formatting patterns from `output.go` formatters.

### Quality Score Computation
```go
// Weighted average: each edge contributes its confidence score.
// Result is a percentage (0-100).
func computeQualityScore(edges map[string]*Edge) float64 {
    if len(edges) == 0 {
        return 0
    }
    var sum float64
    for _, edge := range edges {
        sum += edge.Confidence
    }
    return (sum / float64(len(edges))) * 100
}
```

### Component Grouping by Type (Text Format)
```go
func formatComponentsByType(nodes map[string]*Node) string {
    grouped := make(map[ComponentType][]string)
    for _, node := range nodes {
        ct := node.ComponentType
        if ct == "" {
            ct = "unknown"
        }
        grouped[ct] = append(grouped[ct], node.ID)
    }

    var sb strings.Builder
    sb.WriteString("Components by Type\n")
    sb.WriteString(strings.Repeat("-", 40) + "\n")

    // Sort types for deterministic output
    types := make([]string, 0, len(grouped))
    for t := range grouped {
        types = append(types, string(t))
    }
    sort.Strings(types)

    for _, t := range types {
        ids := grouped[ComponentType(t)]
        sort.Strings(ids)
        fmt.Fprintf(&sb, "  %s (%d):\n", t, len(ids))
        for _, id := range ids {
            fmt.Fprintf(&sb, "    - %s\n", id)
        }
    }
    return sb.String()
}
```

### JSON Output Structure
```go
type CrawlStatsJSON struct {
    Summary        CrawlSummaryJSON        `json:"summary"`
    Components     CrawlComponentsJSON     `json:"components"`
    Confidence     CrawlConfidenceJSON     `json:"confidence"`
    QualityWarnings []CrawlWarningJSON     `json:"quality_warnings"`
}

type CrawlSummaryJSON struct {
    ComponentCount    int     `json:"component_count"`
    RelationshipCount int     `json:"relationship_count"`
    QualityScore      float64 `json:"quality_score"`
    InputPath         string  `json:"input_path"`
    IgnorePatterns    []string `json:"ignore_patterns,omitempty"`
    AliasesApplied    int     `json:"aliases_applied"`
}

type CrawlConfidenceJSON struct {
    Tiers []CrawlTierJSON `json:"tiers"`
}

type CrawlTierJSON struct {
    Tier       string    `json:"tier"`
    Range      [2]float64 `json:"range"`
    Count      int       `json:"count"`
    Percentage float64   `json:"percentage"`
}

type CrawlWarningJSON struct {
    Type    string   `json:"type"`
    Message string   `json:"message"`
    Count   int      `json:"count"`
    Items   []string `json:"items,omitempty"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `crawl --from-multiple` only | Stats-based crawl as default + targeted crawl with `--from-multiple` | This phase | Crawl becomes a pre-export diagnostic tool |
| No quality metrics | Quality score + confidence distribution + warnings | This phase | Operators can assess graph quality before exporting |

## Open Questions

1. **Shared pipeline extraction**
   - What we know: CmdExport steps 1-7 and CmdCrawl steps 1-7 are identical
   - What's unclear: Whether to extract a shared `buildGraph()` helper or just duplicate the pipeline
   - Recommendation: Extract a shared helper if duplication exceeds ~30 lines. The pipeline is ~50 lines in CmdExport (steps 1-7), so a shared helper is worthwhile. However, this is implementation detail -- the planner can decide based on code volume.

2. **Existing crawl command coexistence**
   - What we know: Current `cmdCrawl` in main.go requires `--from-multiple` and operates on pre-indexed DB
   - What's unclear: Whether to keep the old behavior accessible or replace it entirely
   - Recommendation: Make stats mode the default (no required flags). If `--from-multiple` is provided, fall back to targeted traversal mode. This preserves backward compatibility while adding the new diagnostic capability.

3. **Quality warning thresholds**
   - What we know: User wants warnings for orphan nodes, dangling edges, weak-only components
   - What's unclear: What counts as "only weak-confidence edges" -- edges below moderate (0.55)?
   - Recommendation: Define "weak-only" as a component where ALL connected edges have confidence < 0.55 (below moderate tier). This is a Claude's Discretion item per CONTEXT.md.

## Sources

### Primary (HIGH confidence)
- `internal/knowledge/export.go` -- CmdExport pipeline (steps 1-12), the authoritative reference for how the scan-detect-discover pipeline works
- `internal/knowledge/confidence.go` -- 6-tier confidence system with ScoreToTier(), AllConfidenceTiers(), tier boundaries
- `internal/knowledge/crawl.go` -- Existing CrawlMulti BFS traversal (will coexist, not be replaced)
- `internal/knowledge/output.go` -- Existing formatter patterns (FormatCrawl, FormatSearchResults, etc.)
- `internal/knowledge/types.go` -- ComponentType taxonomy and AllComponentTypes()
- `internal/knowledge/scanner.go` -- ScanDirectory + ScanConfig
- `internal/knowledge/graphmdignore.go` -- LoadGraphmdIgnore
- `internal/knowledge/aliases.go` -- LoadAliasConfig + AliasConfig
- `cmd/graphmd/main.go` -- Current cmdCrawl implementation (lines 182-261) and CmdX delegation pattern

### Secondary (MEDIUM confidence)
- `internal/knowledge/command_helpers.go` -- Shared helpers: humanBytes, pruneDanglingEdges, etc.
- `internal/knowledge/query_cli.go` -- QueryEnvelope pattern for structured JSON output (may inform crawl JSON envelope design)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go stdlib only, no external dependencies, consistent with existing codebase
- Architecture: HIGH - CmdExport pipeline is the direct blueprint; all building blocks exist and are well-documented in source
- Pitfalls: HIGH - Identified from direct code analysis of existing pipeline and edge cases visible in the codebase

**Research date:** 2026-03-24
**Valid until:** 2026-04-24 (stable codebase, no external dependency changes expected)
