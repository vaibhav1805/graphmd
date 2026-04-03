# Phase 12: Signal Integration - Research

**Researched:** 2026-04-01
**Domain:** Code signal merging into existing markdown discovery pipeline with per-source provenance
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Confidence Merging Formula:** Probabilistic OR: `1 - (1-a)*(1-b)` when both markdown and code detect the same relationship. Stay within 0.4-1.0 range. New code-only edges added at their original confidence (no penalty).
- **Source Type in Query Results:** `source_type` field always present on every relationship — values: `"markdown"`, `"code"`, `"both"`. Always present (not omitempty). Schema v6 adds `source_type TEXT NOT NULL DEFAULT 'markdown'`. `--source-type` filter available on all query types.

### Claude's Discretion
- Schema v6 migration details (ALTER TABLE vs new columns)
- `code_signals` provenance table structure (file, line, language, evidence per signal)
- How code signals feed into the existing `DiscoverAndIntegrateRelationships` pipeline
- Integration point in `CmdExport` and `CmdCrawl` for combining markdown + code graphs
- How `--source-type` filter interacts with existing `--min-confidence` filter

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SIG-01 | Merge code-detected dependency signals with markdown-detected signals using confidence-weighted aggregation (code as 5th discovery source) | Existing `MergeDiscoveredEdges` + `AlgorithmWeight` infrastructure extends additively; probabilistic OR formula for merging; `DiscoverRelationships`/`DiscoverAndIntegrateRelationships` accept variadic edge sets |
| SIG-02 | Schema v6 migration supporting multi-source provenance per relationship (which sources detected each edge) | ALTER TABLE pattern proven in v3→v4 and v4→v5 migrations; `source_type` column on `graph_edges` + new `code_signals` table; existing SaveGraph/LoadGraph patterns show exactly how to wire new columns |
</phase_requirements>

## Summary

Phase 12 merges the code analysis signals (produced by Phases 9-11) into the existing markdown-derived dependency graph. The codebase is well-prepared for this: `CmdExport` and `CmdCrawl` already call `code.RunCodeAnalysis` and print diagnostic output, but the signals are currently discarded after printing. This phase converts those `code.CodeSignal` values into `DiscoveredEdge` values, feeds them through the existing merge pipeline, applies the probabilistic OR confidence formula for edges detected by both markdown and code, adds a `source_type` column to `graph_edges` for provenance, creates a `code_signals` table for raw signal storage, and adds `--source-type` filtering to the query interface.

The existing architecture makes this straightforward. `MergeDiscoveredEdges` accepts variadic `[]*DiscoveredEdge` sets and merges by `source+target+type` key. The `AlgorithmWeight` map just needs a `"code": 0.85` entry. The schema migration pattern is established (4 prior migrations, all using `ALTER TABLE ADD COLUMN` for additive changes). The main complexity is the confidence merging formula change: currently `MergeDiscoveredEdges` takes the max confidence, but the user decision requires probabilistic OR when both sources detect the same edge. This requires modifying the merge logic or applying the formula as a post-merge step.

**Primary recommendation:** Convert `code.CodeSignal` to `DiscoveredEdge` in a new helper function within `internal/knowledge`, feed the converted edges as the 5th argument to `MergeDiscoveredEdges`, modify the merge or post-process step to apply probabilistic OR for multi-source edges, and add `source_type` as a column on both `graph_edges` (schema) and `Edge` (struct).

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `modernc.org/sqlite` | (existing) | SQLite driver, schema migrations | Already in use; pure-Go, no CGo |
| `database/sql` | stdlib | SQL operations | Already in use for all DB operations |
| `internal/code` | (existing) | Code signal types and analysis | Phases 9-11 already built and tested |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | JSON serialization for query output | Adding `source_type` to JSON output |
| `flag` | stdlib | CLI flag parsing | Adding `--source-type` filter flag |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| ALTER TABLE migration | Drop+recreate table | ALTER TABLE is simpler, proven in 4 prior migrations; drop+recreate loses data |
| Probabilistic OR in MergeDiscoveredEdges | Post-merge pass over combined results | Post-merge is cleaner; avoids modifying shared merge function behavior |
| Storing source_type on Edge struct | Separate lookup table | Edge struct is simpler; source_type is a per-edge attribute, not a many-to-many |

## Architecture Patterns

### Recommended Project Structure
```
internal/knowledge/
├── discovery_orchestration.go  # Modified: add code signals as 5th source
├── discovery.go                # Modified: MergeDiscoveredEdges may need source-aware merge
├── algo_aggregator.go          # Modified: add "code": 0.85 to AlgorithmWeight
├── db.go                       # Modified: schema v6 migration, SaveGraph/LoadGraph with source_type
├── edge.go                     # Modified: add SourceType field, "code-analysis" extraction method
├── relationship_types.go       # Modified: add SignalCode constant
├── registry.go                 # Modified: add SignalCode SignalSource constant
├── export.go                   # Modified: wire code signals into graph (not just print)
├── crawl_cmd.go                # Modified: wire code signals into graph (not just print)
├── query_cli.go                # Modified: add --source-type filter, source_type in JSON output
└── signal_convert.go           # NEW: convertCodeToDiscovered() helper
```

### Pattern 1: Code Signal Conversion
**What:** Convert `code.CodeSignal` to `*DiscoveredEdge` for feeding into the existing merge pipeline.
**When to use:** At the point where code analysis results enter the knowledge pipeline.
**Example:**
```go
// internal/knowledge/signal_convert.go
func convertCodeSignalsToDiscovered(signals []code.CodeSignal, sourceComponent string) []*DiscoveredEdge {
    byKey := make(map[string]*DiscoveredEdge)
    for _, sig := range signals {
        source := sourceComponent // from code.InferSourceComponent
        target := sig.TargetComponent
        edgeType := mapDetectionKindToEdgeType(sig.DetectionKind)
        key := source + "\x00" + target + "\x00" + string(edgeType)

        if existing, ok := byKey[key]; ok {
            // Accumulate signals, keep highest confidence
            existing.Signals = append(existing.Signals, Signal{
                SourceType: SignalCode,
                Confidence: sig.Confidence,
                Evidence:   sig.Evidence,
                Weight:     AlgorithmWeight["code"],
            })
            if sig.Confidence > existing.Confidence {
                existing.Confidence = sig.Confidence
                existing.Evidence = sig.Evidence
            }
        } else {
            edge, _ := NewEdge(source, target, edgeType, sig.Confidence, sig.Evidence)
            if edge != nil {
                edge.ExtractionMethod = "code-analysis"
                edge.SourceFile = sig.SourceFile
                byKey[key] = &DiscoveredEdge{
                    Edge: edge,
                    Signals: []Signal{{
                        SourceType: SignalCode,
                        Confidence: sig.Confidence,
                        Evidence:   sig.Evidence,
                        Weight:     AlgorithmWeight["code"],
                    }},
                }
            }
        }
    }
    // collect values from map
    result := make([]*DiscoveredEdge, 0, len(byKey))
    for _, de := range byKey {
        result = append(result, de)
    }
    return result
}
```

### Pattern 2: Probabilistic OR Confidence Merge (Post-Merge)
**What:** After `MergeDiscoveredEdges` combines all signals, apply probabilistic OR to edges that have signals from both markdown and code sources.
**When to use:** As a post-processing step after `MergeDiscoveredEdges`, before adding to the graph.
**Rationale:** Cleaner than modifying `MergeDiscoveredEdges` directly. The existing function uses max-confidence, which is correct for single-source multi-algorithm merging. The probabilistic OR only applies when markdown AND code independently detect the same edge.

```go
// Apply probabilistic OR for edges with both markdown and code signals
func applyMultiSourceConfidence(edges []*DiscoveredEdge) {
    for _, de := range edges {
        mdConf, codeConf := 0.0, 0.0
        hasMd, hasCode := false, false
        for _, sig := range de.Signals {
            if sig.SourceType == SignalCode {
                if sig.Confidence > codeConf { codeConf = sig.Confidence }
                hasCode = true
            } else {
                if sig.Confidence > mdConf { mdConf = sig.Confidence }
                hasMd = true
            }
        }
        if hasMd && hasCode {
            // Probabilistic OR: 1 - (1-a)*(1-b)
            merged := 1.0 - (1.0-mdConf)*(1.0-codeConf)
            de.Confidence = merged
            de.Edge.Confidence = merged
        }
    }
}
```

### Pattern 3: Source Type Determination
**What:** Determine `source_type` value ("markdown", "code", "both") from the signals on each edge.
**When to use:** Before saving edges to the database.

```go
func determineSourceType(de *DiscoveredEdge) string {
    hasMd, hasCode := false, false
    for _, sig := range de.Signals {
        if sig.SourceType == SignalCode {
            hasCode = true
        } else {
            hasMd = true
        }
    }
    switch {
    case hasMd && hasCode: return "both"
    case hasCode:          return "code"
    default:               return "markdown"
    }
}
```

### Pattern 4: Schema v6 Migration
**What:** Add `source_type` column to `graph_edges` and create `code_signals` provenance table.
**When to use:** In `Migrate()` function, following the established v1→v2, v2→v3, v3→v4, v4→v5 pattern.

```go
func (db *Database) migrateV5ToV6() error {
    // Add source_type to graph_edges (DEFAULT ensures existing edges get 'markdown')
    alter := `ALTER TABLE graph_edges ADD COLUMN source_type TEXT NOT NULL DEFAULT 'markdown'`
    if _, err := db.conn.Exec(alter); err != nil {
        if !strings.Contains(err.Error(), "duplicate column") {
            return fmt.Errorf("exec %q: %w", alter, err)
        }
    }

    // Create code_signals provenance table
    create := `CREATE TABLE IF NOT EXISTS code_signals (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        source_component TEXT NOT NULL,
        target_component TEXT NOT NULL,
        signal_type TEXT NOT NULL,
        confidence REAL NOT NULL,
        evidence TEXT NOT NULL,
        file_path TEXT NOT NULL,
        line_number INTEGER NOT NULL,
        language TEXT NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now'))
    )`
    if _, err := db.conn.Exec(create); err != nil {
        return fmt.Errorf("exec create code_signals: %w", err)
    }

    // Indexes for provenance queries
    indexes := []string{
        `CREATE INDEX IF NOT EXISTS idx_code_signals_source ON code_signals(source_component)`,
        `CREATE INDEX IF NOT EXISTS idx_code_signals_target ON code_signals(target_component)`,
    }
    for _, stmt := range indexes {
        if _, err := db.conn.Exec(stmt); err != nil {
            return fmt.Errorf("exec %q: %w", stmt, err)
        }
    }
    return nil
}
```

### Anti-Patterns to Avoid
- **Modifying MergeDiscoveredEdges' max-confidence behavior:** This function works correctly for single-source multi-algorithm merging. The probabilistic OR is a multi-source concern and should be applied separately as a post-merge step.
- **Adding source_type to the discovery pipeline internals:** Source type is determined at the point of conversion (code vs markdown), not deep inside the aggregator. Keep it at the integration boundary.
- **Creating a separate "code edge" table instead of extending graph_edges:** Code-detected edges ARE graph edges. They should live in the same table with a `source_type` discriminator, not in a separate world.
- **Breaking the existing DiscoverRelationships call path:** `CmdExport` calls `DiscoverRelationships` (the simpler 3-algorithm version), not `DiscoverAndIntegrateRelationships`. The code signal integration must work with the simpler path too.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal merging | Custom merge logic for code+markdown | Extend existing `MergeDiscoveredEdges` + post-merge probabilistic OR | Existing merge handles dedup, key-based grouping, deterministic ordering |
| Schema migration | Manual SQL script execution | Extend existing `Migrate()` chain with `migrateV5ToV6()` | Pattern proven 4 times; handles idempotency, version tracking |
| Edge provenance in DB | New ORM or abstraction | Follow existing `SaveGraph`/`LoadGraph` pattern with `sql.NullString` | Exact pattern used for source_file, extraction_method already |
| Code signal to edge conversion | Ad-hoc conversion in CmdExport | Dedicated `convertCodeSignalsToDiscovered()` function | Reused by both CmdExport and CmdCrawl; testable in isolation |

**Key insight:** Phase 12 is 90% wiring — connecting existing code analysis output to the existing merge pipeline. The new code is primarily glue: type conversion, a confidence formula, a schema migration, and a query filter. All patterns are already proven in the codebase.

## Common Pitfalls

### Pitfall 1: Nodes Not Existing for Code-Detected Targets
**What goes wrong:** Code analysis detects targets like "postgres", "redis", "kafka" that don't have corresponding markdown documents (graph nodes). `SaveGraph` drops edges with missing endpoints — currently printing warnings to stderr.
**Why it happens:** Code detects infrastructure dependencies that may not have markdown documentation files.
**How to avoid:** Before adding code edges to the graph, create stub nodes for code-detected targets that don't exist in the graph. Use `graph.AddNode(&Node{ID: target, Type: "infrastructure", ComponentType: mapTargetType(sig.TargetType)})` for targets not already in graph.Nodes.
**Warning signs:** "edges dropped (missing endpoint nodes)" warnings in stderr after enabling `--analyze-code`.

### Pitfall 2: Probabilistic OR Exceeding 1.0 with Three+ Sources
**What goes wrong:** If a future third source is added, cascading probabilistic OR could theoretically exceed 1.0 due to floating point.
**Why it happens:** Probabilistic OR of two values stays under 1.0 mathematically, but floating point edge cases exist.
**How to avoid:** Clamp the result: `min(merged, 1.0)`. This is defensive and costs nothing.
**Warning signs:** Confidence values > 1.0 in test output.

### Pitfall 3: source_type Not Set on Existing Markdown-Only Edges
**What goes wrong:** When `--analyze-code` is NOT used, all edges should still have `source_type: "markdown"` in query output. If the field is only set during code integration, non-code runs produce empty source_type.
**Why it happens:** The schema DEFAULT handles DB-level storage, but the Edge struct in memory needs to be populated too.
**How to avoid:** Initialize `Edge.SourceType = "markdown"` in `NewEdge()` or in `LoadGraph()` when the column value is empty/null. The schema `DEFAULT 'markdown'` handles DB-level; code must handle struct-level.
**Warning signs:** `source_type: ""` in JSON query output when no code analysis was run.

### Pitfall 4: CmdExport and CmdCrawl Diverging on Code Integration
**What goes wrong:** Code integration logic is duplicated between `CmdExport` and `CmdCrawl` with subtle differences.
**Why it happens:** Both functions currently have independent code paths for code analysis (Step 7b in each). If the integration logic is copy-pasted rather than shared, they drift.
**How to avoid:** Extract the "convert code signals to discovered edges + add stub nodes" logic into a shared function. Both CmdExport and CmdCrawl call it.
**Warning signs:** CmdCrawl shows different edge counts than CmdExport for the same input with `--analyze-code`.

### Pitfall 5: --source-type Filter Interacting Badly with --min-confidence
**What goes wrong:** User filters `--source-type code --min-confidence 0.8` but gets no results because code-only edges have their original confidence (e.g. 0.65), not the boosted probabilistic OR confidence.
**Why it happens:** Probabilistic OR only applies when BOTH sources detected the edge. Code-only edges keep their original confidence.
**How to avoid:** This is actually correct behavior — document it clearly. Code-only edges at 0.65 won't pass a 0.8 filter. The filters compose independently: source_type filters by provenance, min-confidence filters by score.
**Warning signs:** User confusion, not a bug. Clear CLI help text prevents this.

## Code Examples

### Integration Point: CmdExport Code Signal Wiring
```go
// In export.go, replace Step 7b (currently only prints summary):

// Step 7b: Run code analysis and integrate into graph.
if a.AnalyzeCode {
    fmt.Fprintf(os.Stderr, "  Analyzing source code...\n")
    signals, codeErr := code.RunCodeAnalysis(absFrom,
        goparser.NewGoParser(),
        pyparser.NewPythonParser(),
        jsparser.NewJSParser(),
    )
    if codeErr != nil {
        fmt.Fprintf(os.Stderr, "  Warning: code analysis failed: %v\n", codeErr)
    } else {
        code.PrintCodeSignalsSummary(os.Stderr, signals)

        // Convert to DiscoveredEdge values
        sourceComponent := code.InferSourceComponent(absFrom)
        codeEdges := convertCodeSignalsToDiscovered(signals, sourceComponent)

        // Create stub nodes for targets not in graph
        ensureCodeTargetNodes(graph, codeEdges)

        // Merge code edges with markdown edges (already in discovered)
        // Then apply probabilistic OR for multi-source edges
        allEdges := MergeDiscoveredEdges(discovered, codeEdges)
        applyMultiSourceConfidence(allEdges)

        // Add passing edges to graph with source_type
        for _, de := range allEdges {
            if de.Edge != nil && de.Edge.Confidence >= a.MinConfidence {
                de.Edge.SourceType = determineSourceType(de)
                _ = graph.AddEdge(de.Edge)
            }
        }
    }
}
```

### Schema v6: Edge Struct Extension
```go
// In edge.go, add to Edge struct:

// SourceType tracks which detection source produced this edge.
// Values: "markdown", "code", "both". Defaults to "markdown".
SourceType string
```

### Schema v6: SaveGraph Extension
```go
// In db.go SaveGraph, extend INSERT:
`INSERT OR REPLACE INTO graph_edges
 (id, source_id, target_id, type, confidence, evidence,
  source_file, extraction_method, detection_evidence, evidence_pointer, last_modified,
  source_type)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
// ... add e.SourceType as 12th parameter (default "markdown" if empty)
```

### SaveCodeSignals: Raw Provenance Storage
```go
func (db *Database) SaveCodeSignals(signals []code.CodeSignal) error {
    return transaction(db.conn, func(tx *sql.Tx) error {
        stmt, err := tx.Prepare(`INSERT INTO code_signals
            (source_component, target_component, signal_type, confidence,
             evidence, file_path, line_number, language)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
        if err != nil { return err }
        defer stmt.Close()

        for _, sig := range signals {
            _, err := stmt.Exec(
                code.InferSourceComponent("."), sig.TargetComponent,
                sig.DetectionKind, sig.Confidence,
                sig.Evidence, sig.SourceFile, sig.LineNumber, sig.Language,
            )
            if err != nil { return err }
        }
        return nil
    })
}
```

### Query Filter: --source-type
```go
// In query_cli.go, add flag:
fs.StringVar(&sourceType, "source-type", "", "Filter by source type: markdown, code, both")

// In query execution, filter edges:
if sourceType != "" {
    // When loading edges from DB, add WHERE clause:
    // WHERE source_type = ? (or source_type IN ('code', 'both') for --source-type code)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Code signals printed to stderr only | Code signals integrated into graph as 5th source | Phase 12 (this phase) | Code-detected deps become queryable edges |
| Max-confidence merging across all signals | Probabilistic OR for multi-source edges | Phase 12 | Two 0.6 signals → 0.84 instead of 0.6 |
| No source provenance on edges | `source_type` column: markdown/code/both | Phase 12 | Agents can filter by detection source |
| Schema v5 (no source tracking) | Schema v6 (source_type + code_signals table) | Phase 12 | Full provenance chain preserved |

## Open Questions

1. **Edge type mapping from code DetectionKind**
   - What we know: Code signals have `DetectionKind` values like "http_call", "db_connection", "cache_client", "queue_producer". The Edge struct uses `EdgeType` values like "depends-on", "communicates-with", etc.
   - What's unclear: What mapping from DetectionKind → EdgeType should be used? Currently the codebase uses edge types like "depends-on", "communicates-with", "uses", etc.
   - Recommendation: Map all code detection kinds to "depends-on" for simplicity. The `source_type` and `code_signals` table provide the fine-grained type info. Edge type taxonomy expansion is out of scope.

2. **SourceComponent identity for code signals**
   - What we know: `code.InferSourceComponent(dir)` infers the source component from go.mod/package.json/etc. All code signals from one analysis run share the same source component.
   - What's unclear: If the source component name doesn't match any existing markdown node ID, the edge has an unresolvable source.
   - Recommendation: Create a stub node for the source component too, not just targets. Or use the closest matching markdown node via alias resolution.

3. **DiscoverRelationships vs DiscoverAndIntegrateRelationships**
   - What we know: `CmdExport` calls `DiscoverRelationships` (simpler, 3 algorithms). `DiscoverAndIntegrateRelationships` is the full 4-algorithm orchestration with quality filtering.
   - What's unclear: Should code signals go through the simpler or full pipeline?
   - Recommendation: Use the simpler path — code signals have already been quality-filtered by the parsers (confidence assignment happens at detection time). Feed them directly into `MergeDiscoveredEdges` alongside the markdown discovery results, then apply probabilistic OR.

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis: `internal/knowledge/db.go` — schema v5, migration pattern (v1→v5), SaveGraph/LoadGraph
- Direct codebase analysis: `internal/knowledge/discovery.go` — MergeDiscoveredEdges implementation
- Direct codebase analysis: `internal/knowledge/discovery_orchestration.go` — DiscoverAndIntegrateRelationships, 4-algorithm parallel pattern
- Direct codebase analysis: `internal/knowledge/algo_aggregator.go` — AlgorithmWeight map, AggregateSignals
- Direct codebase analysis: `internal/knowledge/export.go` — CmdExport pipeline, Step 7b code analysis stub
- Direct codebase analysis: `internal/knowledge/crawl_cmd.go` — CmdCrawl pipeline, Step 7b code analysis stub
- Direct codebase analysis: `internal/code/signal.go` — CodeSignal struct definition
- Direct codebase analysis: `internal/code/integration.go` — RunCodeAnalysis, InferSourceComponent
- Direct codebase analysis: `internal/knowledge/registry.go` — SignalSource type, Signal struct
- Direct codebase analysis: `internal/knowledge/edge.go` — Edge struct, provenance fields, ExtractionMethod validation
- Direct codebase analysis: `internal/knowledge/query_cli.go` — QueryEnvelope, EnrichedRelationship, flag parsing

### Secondary (MEDIUM confidence)
- `.planning/research/ARCHITECTURE.md` — v2.0 architecture design, schema v6 spec
- `.planning/research/SUMMARY.md` — milestone research, signal merging approach
- `12-CONTEXT.md` — user decisions on probabilistic OR and source_type field

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all changes use existing libraries and patterns
- Architecture: HIGH — based on direct reading of all 6 files that need modification; patterns proven in 4 prior migrations and existing discovery pipeline
- Pitfalls: HIGH — all pitfalls identified from actual code paths (SaveGraph dropping edges, Edge struct initialization, CmdExport/CmdCrawl divergence)

**Research date:** 2026-04-01
**Valid until:** 2026-05-01 (stable — internal codebase patterns, no external dependencies)
