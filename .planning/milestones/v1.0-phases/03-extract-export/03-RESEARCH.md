# Phase 3: Extract & Export Pipeline - Research

**Researched:** 2026-03-19
**Domain:** End-to-end pipeline wiring: markdown crawl → component extraction → relationship discovery → SQLite graph → ZIP archive
**Confidence:** HIGH

## Summary

Phase 3 wires together existing, tested components into a unified export pipeline. The scanner (`ScanDirectory`), component detector (`ComponentDetector`), discovery algorithms (`DiscoverAndIntegrateRelationships`), graph builder (`GraphBuilder`), and SQLite persistence (`Database.SaveGraph`) all exist and are individually tested. The `cmdExport` stub in `main.go` prints a placeholder message and is not connected to `CmdExport` in `export.go`.

The primary technical work is: (1) adding `.graphmdignore` file support to the scanner, (2) implementing component name aliasing, (3) converting the archive format from tar.gz to ZIP per user decision, (4) adding a `metadata.json` to the ZIP alongside `graph.db`, (5) wiring the full pipeline (scan → detect → discover → aggregate → save → package) into a single `CmdExport` invocation, and (6) adding the indexes required for agent query performance.

**Primary recommendation:** Refactor `CmdExport` to orchestrate the full pipeline (reusing `cmdIndex` logic but outputting a ZIP instead of writing to `.bmd/`), convert from tar.gz to `archive/zip`, and add `.graphmdignore` support to the existing `ScanConfig` mechanism.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **File exclusions:** Auto-generated `.graphmdignore` file with common patterns (vendor, node_modules, .git, etc.). Operators can customize; crawl respects the ignore file.
- **File types:** `.md` files only (no `.markdown` or other extensions).
- **Hidden files:** Always excluded (files starting with `.`).
- **Component provenance:** Include source file path in database schema so agents can trace components back to source markdown.
- **Deduplication:** When multiple extraction methods find the same component, produce a single entry with aggregated confidence score.
- **Component name aliasing:** Support user-defined aliases in config (e.g., `payment-api` ↔ `PaymentAPI` ↔ `Payment-Service` map to canonical name). Aliases applied during export pipeline, before graph building.
- **Type resolution:** When extraction methods suggest different types for the same component, use the highest confidence score to determine type.
- **Confidence threshold:** Include all extracted components regardless of confidence level; agents filter based on confidence.
- **Unknown types:** Components with undetermined types default to `unknown` type.
- **Method attribution:** Include `extraction_method` metadata for each component.
- **Algorithm set:** Run all available discovery algorithms by default (co-occurrence, structural, NER, semantic, LLM). Operators can disable expensive algorithms in config if needed.
- **Algorithm disagreement:** Include all relationship findings. Confidence score reflects aggregation: higher confidence = more algorithms found the relationship.
- **LLM algorithm handling:** No time/cost limits; LLM runs to completion. Log all API calls so operators can monitor and cancel.
- **Confidence in results:** Single aggregated confidence score per relationship (no per-algorithm attribution).
- **ZIP contents:** `graph.db` (SQLite) + `metadata.json` only.
- **Metadata fields:** Required: `version`, `created_at` (ISO 8601), `component_count`, `relationship_count`. Extended: `input_path`, `ignore_patterns`, `aliases_applied`, `algorithm_versions`, `schema_version`.
- **SQLite indexing:** Create indexes on component name, component type, relationship source/target, relationship confidence.
- **Schema versioning:** Include schema version in metadata.json. Phase 4 import validates schema compatibility.

### Claude's Discretion
- Exact index column combinations and cardinality tuning
- `.graphmdignore` default patterns (implement sensible defaults)
- Alias config file format and parsing
- Metadata.json formatting and nesting structure
- Error handling and recovery during export (partial failures, timeouts)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EXTRACT-01 | Crawl markdown files recursively with component extraction | `ScanDirectory` exists with `ScanConfig` ignore patterns. Need to add `.graphmdignore` file loading. `ComponentDetector.DetectComponents` handles extraction. Wire scanner output to detector. |
| EXTRACT-02 | Infer relationships from markdown references (service names, not just explicit links) | `DiscoverAndIntegrateRelationships` runs co-occurrence, structural, NER, and LLM algorithms concurrently. Mention extraction via `NERRelationships` and `CoOccurrenceRelationships` already infers from service names. |
| EXTRACT-03 | Apply multiple discovery algorithms with signal aggregation | `DiscoverAndIntegrateRelationships` runs 4 algorithms in parallel. `AggregateSignalsByLocation` implements weighted averaging. `AlgorithmWeight` map defines weights. `FilterDiscoveredEdges` applies quality gates. |
| EXPORT-01 | Build graph, create SQLite database with indexed schema, package as ZIP | `GraphBuilder.Build` creates graph from docs. `Database.SaveGraph` persists to SQLite. Schema has indexes on type, source, target. Need: additional indexes on component name and confidence. Convert archive from tar.gz to ZIP. |
| EXPORT-02 | ZIP includes database file + metadata (version, timestamp, component count) | `KnowledgeMetadata` struct exists but targets tar.gz. Need to create new metadata format per CONTEXT.md decisions. Package as ZIP with `graph.db` + `metadata.json`. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `archive/zip` | stdlib | ZIP archive creation | Go stdlib; replaces current `archive/tar` + `compress/gzip` per user decision |
| `modernc.org/sqlite` | v1.46.1 | Pure-Go SQLite driver | Already in go.mod; no CGO dependency |
| `encoding/json` | stdlib | metadata.json serialization | Already used throughout codebase |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `bufio` | stdlib | `.graphmdignore` file parsing | Line-by-line ignore pattern loading |
| `path/filepath` | stdlib | Cross-platform path handling | Already used for scanning |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `archive/zip` (stdlib) | `archive/tar` + `compress/gzip` (current) | User decision specifies ZIP; ZIP is simpler for agents to consume |
| Custom ignore parser | `gitignore`-compatible library | Custom is simpler; `.graphmdignore` only needs basic glob patterns, not full gitignore semantics |

## Architecture Patterns

### Recommended Pipeline Structure
```
cmdExport (CLI entry point)
  └── ExportPipeline (new orchestrator function)
        ├── 1. Load .graphmdignore → ScanConfig
        ├── 2. Load alias config → alias map
        ├── 3. ScanDirectory(root, config) → []Document
        ├── 4. ComponentDetector.DetectComponents(graph, docs) → []Component
        ├── 5. Apply aliases (canonical name resolution)
        ├── 6. DiscoverAndIntegrateRelationships(...) → edges
        ├── 7. GraphBuilder.Build + merge discovered edges → Graph
        ├── 8. Database.SaveGraph + SaveComponentMentions → graph.db
        ├── 9. Add extra indexes (component name, confidence)
        ├── 10. Build metadata.json
        └── 11. Package graph.db + metadata.json → output.zip
```

### Pattern 1: Existing CmdExport Refactoring
**What:** The existing `CmdExport` in `export.go` already does scan → index → tar.gz. Refactor to use the full pipeline and output ZIP.
**When to use:** This is the primary pattern for the phase.
**Key change:** Currently `CmdExport` calls `buildIndex` (which only creates link-based edges) and then archives markdown files + DB. The new version must: run component detection, run discovery algorithms, apply aliases, create ZIP with only `graph.db` + `metadata.json`.

### Pattern 2: .graphmdignore Loading
**What:** Parse a `.graphmdignore` file (one pattern per line, `#` comments, blank lines skipped) and feed patterns into `ScanConfig.IgnoreDirs` / `ScanConfig.IgnoreFiles`.
**When to use:** Before scanning; loaded once at export start.
**Implementation:** Similar to `parseComponentYAML` — simple line-by-line parser. Auto-generate the file with defaults if it doesn't exist (or use defaults silently when file is absent).

### Pattern 3: Component Name Aliasing
**What:** Load a config file mapping alias names to canonical component IDs. During pipeline execution, after component detection but before graph building, normalize all component references through the alias map.
**When to use:** When operators have naming inconsistencies across their documentation.
**Implementation:** Similar to `ComponentConfig` — simple YAML/JSON config with `canonical_name → [aliases]` mapping. Applied as a map lookup during edge source/target resolution.

### Anti-Patterns to Avoid
- **Running discovery twice:** `cmdIndex` already runs `DiscoverRelationships` — the export pipeline should NOT call `cmdIndex` and then re-run discovery. Build the full pipeline once.
- **Including markdown files in ZIP:** User decision explicitly states ZIP contains only `graph.db` + `metadata.json`. Don't carry over the tar.gz pattern of including source markdown.
- **Modifying shared graph during traversal:** Edge copies must be used (pattern already established in `TraverseDFS`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ZIP creation | Custom binary format | `archive/zip` stdlib | Standard, well-tested, agents can unzip trivially |
| Ignore file parsing | Complex glob engine | Simple line-by-line with `filepath.Match` | `.graphmdignore` only needs basic patterns, not full gitignore |
| SQLite FTS or custom search | Full-text search engine | Existing indexed columns + WHERE clauses | Agent queries use exact match and range filters, not full-text |
| Signal aggregation | New aggregation logic | `AggregateSignalsByLocation` + `AlgorithmWeight` | Already implemented and tested in Phase 2 |
| Component detection | New detector | `ComponentDetector.DetectComponents` | Already implemented and tested in Phase 1 |

**Key insight:** Almost all pipeline components exist. The work is orchestration and wiring, not algorithm development. Resist the urge to rewrite existing algorithms — wire them together.

## Common Pitfalls

### Pitfall 1: Dangling Node References in Edges
**What goes wrong:** Discovery algorithms produce edges referencing nodes not in the graph (e.g., document IDs from mentions that don't have corresponding scanned files).
**Why it happens:** `DiscoverRelationships` can create edges for mentioned services that aren't actual documents.
**How to avoid:** `SaveGraph` already skips edges with dangling references (lines 904-909 of `db.go`). Ensure the full graph has nodes for all documents before adding discovered edges. The existing `pruneDanglingEdges` helper also handles this.
**Warning signs:** Edge counts much higher than expected; FK constraint errors from SQLite.

### Pitfall 2: Duplicate Component Entries After Aliasing
**What goes wrong:** Same component appears multiple times under different names after aliasing is applied incorrectly.
**Why it happens:** Aliases applied after graph nodes are created, leaving orphaned nodes.
**How to avoid:** Apply alias resolution BEFORE adding nodes to the graph. Map document IDs to canonical names first, then build graph nodes with canonical IDs.
**Warning signs:** Component count in metadata higher than expected; duplicate entries in `graph_nodes`.

### Pitfall 3: Archive Format Mismatch with Import
**What goes wrong:** Phase 4 import expects one format (tar.gz currently in `importtar.go`) but Phase 3 now produces ZIP.
**Why it happens:** Existing `ImportKnowledgeTar` reads tar.gz; new export writes ZIP.
**How to avoid:** Phase 3 changes the export format only. Phase 4 will implement a new ZIP-based import. Don't modify `importtar.go` — it's legacy and can remain for backward compat.
**Warning signs:** Import command fails on new archives.

### Pitfall 4: Missing Indexes for Agent Query Performance
**What goes wrong:** Queries slow on large graphs because indexes don't cover the access patterns.
**Why it happens:** Current schema has indexes on `source_id`, `target_id`, `type`, and `component_type` — but not on confidence or component name text.
**How to avoid:** Add indexes per user decision: confidence on `graph_edges`, component name on `graph_nodes` (add a `name` column or index on `title`). Create indexes in the schema migration or during export.
**Warning signs:** Full table scans on query (visible via `EXPLAIN QUERY PLAN`).

### Pitfall 5: LLM Discovery Blocking Export
**What goes wrong:** Export takes very long or hangs waiting for LLM API calls.
**Why it happens:** User decision says "no time/cost limits; LLM runs to completion."
**How to avoid:** Log all LLM API calls so operators can see progress. Make LLM discovery opt-in via CLI flag (default off for `export`, since it can be expensive). Provide clear documentation that `--llm-discovery` enables it.
**Warning signs:** Export command appears to hang with no output.

## Code Examples

### Existing Pipeline Components (verified from codebase)

### Scanning with ScanConfig
```go
// Source: internal/knowledge/scanner.go
config := ScanConfig{
    UseDefaultIgnores: true,
    IgnoreDirs:  []string{"vendor", "node_modules"},  // from .graphmdignore
    IgnoreFiles: []string{"CLAUDE.md", "*.lock"},      // from .graphmdignore
}
docs, err := ScanDirectory(absRoot, config)
```

### Full Discovery Orchestration
```go
// Source: internal/knowledge/discovery_orchestration.go
// Runs co-occurrence, structural, NER, and LLM in parallel
filtered, all := DiscoverAndIntegrateRelationships(
    documents,
    nil,            // BM25 index (deprecated param)
    explicitGraph,  // baseline graph from link extraction
    DefaultDiscoveryFilterConfig(),
    llmCfg,
    trees,
    knownComponents,
)
```

### Signal Aggregation with Weighted Averaging
```go
// Source: internal/knowledge/algo_aggregator.go
// AlgorithmWeight: cooccurrence=0.3, ner=0.5, structural=0.6, semantic=0.7, llm=1.0
aggregated := AggregateSignalsByLocation(signals)
edges := AggregatedToEdges(aggregated)
```

### ZIP Archive Creation (new for this phase)
```go
// Source: Go stdlib archive/zip
zipFile, _ := os.Create(outputPath)
defer zipFile.Close()
zw := zip.NewWriter(zipFile)
defer zw.Close()

// Add graph.db
dbWriter, _ := zw.Create("graph.db")
dbData, _ := os.ReadFile(dbPath)
dbWriter.Write(dbData)

// Add metadata.json
metaWriter, _ := zw.Create("metadata.json")
metaJSON, _ := json.MarshalIndent(meta, "", "  ")
metaWriter.Write(metaJSON)
```

### Component Detection + Type Classification
```go
// Source: internal/knowledge/components.go + main.go cmdIndex
detector := NewComponentDetector()
components := detector.DetectComponents(graph, docs)
for _, comp := range components {
    if node, ok := graph.Nodes[comp.File]; ok {
        node.ComponentType = comp.Type
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tar.gz archive with markdown + DB | ZIP with graph.db + metadata.json only | This phase (user decision) | Smaller artifact, simpler for agents to consume |
| `buildIndex` (link-based edges only) | Full pipeline with all discovery algorithms | This phase | Much richer graph with confidence-scored relationships |
| No file exclusion config | `.graphmdignore` file | This phase | Operators can control what's scanned |
| Max-confidence signal aggregation | Weighted average aggregation | Phase 2 (already done) | Better reflects combined evidence strength |

## Open Questions

1. **Alias config file format**
   - What we know: User wants `payment-api` ↔ `PaymentAPI` ↔ `Payment-Service` mapping to canonical names.
   - What's unclear: Exact file format (YAML like `components.yaml`? JSON? New section in existing config?)
   - Recommendation: Use YAML similar to existing `components.yaml` pattern. File name: `aliases.yaml` or new section in a unified `graphmd.yaml` config. Claude's discretion per CONTEXT.md.

2. **LLM discovery default behavior in export**
   - What we know: User says "no time/cost limits" and "log all API calls."
   - What's unclear: Should LLM discovery run by default in `export`, or be opt-in via flag?
   - Recommendation: Opt-in via `--llm-discovery` flag (same as existing `cmdIndex` pattern). Export should be fast by default; LLM adds unpredictable latency and cost.

3. **Schema version for new metadata**
   - What we know: Current `SchemaVersion = 4`. User wants schema version in `metadata.json`.
   - What's unclear: Should this bump to 5 (for any new indexes), or stay 4 (no DDL changes needed)?
   - Recommendation: If new indexes are added via `CREATE INDEX`, bump to 5 and add migration. The existing migration system handles this cleanly.

4. **Index on component name**
   - What we know: User wants "component name (fast lookups)" index. Current schema has `graph_nodes.title` but no `name` column.
   - What's unclear: Index on `title` column? Or add a dedicated `name` column?
   - Recommendation: Index on `title` column (it already holds the component's display name). Adding a `name` column would require schema migration and changes to `SaveGraph`/`LoadGraph`.

## Sources

### Primary (HIGH confidence)
- Codebase analysis of `internal/knowledge/` — all Go source files read directly
- `export.go` — existing `CmdExport` implementation (tar.gz format, full pipeline stub)
- `discovery_orchestration.go` — existing 4-algorithm parallel discovery
- `algo_aggregator.go` — existing weighted average signal aggregation
- `scanner.go` — existing `ScanDirectory` with `ScanConfig` filtering
- `db.go` — existing schema DDL (SchemaVersion 4), `SaveGraph`, `SaveComponentMentions`
- `graph.go` — existing `GraphBuilder.Build`, BFS/DFS traversal
- `components.go` — existing `ComponentDetector`, `ComponentConfig` parsing
- `go.mod` — Go 1.24, modernc.org/sqlite v1.46.1

### Secondary (MEDIUM confidence)
- Go stdlib `archive/zip` documentation — standard ZIP creation API (verified via training data, stdlib is stable)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all libraries already in use or Go stdlib
- Architecture: HIGH - pipeline components exist and are tested; wiring pattern is clear from `cmdIndex` and `CmdExport`
- Pitfalls: HIGH - identified from direct codebase analysis of existing edge cases and FK constraints

**Research date:** 2026-03-19
**Valid until:** 2026-04-19 (stable — all components are internal Go code, no external API changes expected)
