# Phase 4: Import & Query Pipeline - Research

**Researched:** 2026-03-23
**Domain:** CLI import/query pipeline for pre-computed dependency graphs (Go, SQLite, ZIP)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Storage location:** XDG data directory (`~/.local/share/graphmd/`) for persistent, cross-session graph storage
- **Named graphs:** Support naming graphs at import (`graphmd import graph.zip --name prod-infra`). Most recently imported graph is the default; `--graph <name>` selects a specific one for queries
- **Schema validation:** Fail with clear error message when ZIP has a newer schema version than the binary supports. No best-effort loading
- **Import validation:** Full validation on import — verify ZIP structure, metadata.json parseable, graph.db has expected tables/schema version. Reject bad imports early
- **JSON structure:** Consistent envelope for all query types: `{query, results, metadata}`
  - `query`: type and parameters used
  - `results`: the actual data (components, relationships, paths)
  - `metadata`: timing, counts, graph name, build info (version, created_at)
- **Confidence presentation:** Both numeric score and tier name on every relationship: `{"confidence": 0.85, "confidence_tier": "strong"}`
- **Provenance:** Inline with each relationship (source_file, extraction_method). No cross-referencing needed
- **Graph metadata:** Always included in metadata section of every response (graph name, version, created_at, component_count). Helps agents detect stale graphs
- **Output format:** JSON by default. `--format table` available for human debugging/inspection
- **Impact query:** `graphmd query impact --component X` with `--depth N` / `--depth all`
- **Dependencies query:** `graphmd query dependencies --component X` (separate subcommand)
- **Path query:** `graphmd query path --from A --to B` with `--limit N` (default 10)
- **List query:** `graphmd query list` with composable filters `--type service --min-confidence 0.7`
- **Global flags:** `--min-confidence`, `--graph <name>`, `--format json|table`
- **No type filtering on impact/dependencies** — agents get all results and filter client-side
- **Unknown component:** JSON error with fuzzy-match suggestions
- **No path found:** Success with empty paths array + reason field
- **No graph imported:** JSON error with actionable message
- **Exit codes:** Consistent exit code 1 for all errors. Agents parse the JSON `code` field
- **Error envelope:** `{"error": "<message>", "code": "<ERROR_CODE>"}`

### Claude's Discretion

- Exact XDG path resolution and cross-platform fallback (macOS/Linux)
- Graph storage file naming and directory structure under XDG
- Table format column layout and styling
- Fuzzy matching algorithm for component name suggestions
- Path-finding algorithm choice (BFS, DFS, Dijkstra)
- How `--depth all` is bounded internally (safety limits for large graphs)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| IMPORT-01 | Implement `import` command: unzip, load SQLite, validate schema | ZIP import logic (Section: Architecture Patterns — Import Pipeline), XDG storage (Section: Standard Stack — XDG), schema validation against `SchemaVersion` constant |
| IMPORT-02 | CLI query interface for four core patterns: impact, dependencies, path, list | Existing `ExecuteImpact`, `ExecuteCrawl`, `Graph.FindPaths`, `Graph.GetIncoming` provide core algorithms; new CLI command routing + JSON envelope needed (Section: Architecture Patterns — Query Router) |
| IMPORT-03 | Return results with confidence scores and metadata (provenance) | Existing `QueryResult`, `QueryEdge`, `AffectedNode` structs have provenance fields; new envelope wrapper adds tier names + graph metadata (Section: Architecture Patterns — Response Envelope) |
</phase_requirements>

## Summary

Phase 4 connects the export pipeline (Phase 3) to the query interface (Phase 2) by adding two CLI commands: `import` and `query`. The import command extracts a ZIP archive (produced by `graphmd export`) into a persistent XDG data directory with named graph support. The query command loads a stored graph into memory and executes one of four query patterns, returning structured JSON with confidence tiers and provenance.

The codebase already contains nearly all the building blocks. The export pipeline produces ZIP files containing `graph.db` + `metadata.json`. The `Database.LoadGraph()` method reconstitutes a full `Graph` struct from SQLite. The `ExecuteImpact` and `ExecuteCrawl` functions handle traversal with confidence filtering. `Graph.FindPaths` finds all simple paths between two nodes. The `QueryResult` struct has JSON tags for agent-facing output. What's missing is: (1) the import CLI command with XDG persistence, (2) the `query` CLI subcommand routing, (3) a response envelope matching the CONTEXT.md contract, (4) the dependencies query (reverse of impact — follow `ByTarget` instead of `BySource`), and (5) fuzzy-match error handling.

**Primary recommendation:** Build the import command first (ZIP extraction + XDG storage + schema validation + named graphs), then layer the query subcommands on top, reusing existing graph algorithms with thin CLI wrappers and a new JSON response envelope.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `archive/zip` | stdlib | ZIP extraction for import | Already used in export.go `packageZIP`; matches export format |
| `modernc.org/sqlite` | v1.46.1 | Pure-Go SQLite for graph persistence | Already the project's DB driver; no CGO dependency |
| `encoding/json` | stdlib | JSON output for query results | Already used throughout for agent-facing output |
| `flag` | stdlib | CLI argument parsing | Consistent with existing commands (export, list, index) |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os` | stdlib | XDG directory resolution, file I/O | Import storage, graph file management |
| `path/filepath` | stdlib | Cross-platform path manipulation | XDG paths, graph directory structure |
| `strings` | stdlib | Fuzzy matching (Levenshtein or substring) | Component name suggestions on NOT_FOUND |
| `text/tabwriter` | stdlib | Table format output | `--format table` human debugging mode |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `flag` | `cobra` or `urfave/cli` | Project uses stdlib `flag` everywhere; adding cobra would be inconsistent and add a dependency |
| Hand-rolled XDG | `os.UserHomeDir()` + env checks | No third-party XDG library needed; `$XDG_DATA_HOME` fallback to `~/.local/share` is trivial |
| Levenshtein distance | `strings.Contains` substring match | Levenshtein gives better fuzzy suggestions but adds complexity; substring + prefix matching is sufficient for MVP |

## Architecture Patterns

### Recommended Project Structure

New files to create:

```
internal/knowledge/
├── import.go          # CmdImport, ImportZIP, XDG storage, named graph management
├── import_test.go     # Import validation, schema checks, named graph tests
├── query_cli.go       # CmdQuery router, subcommand dispatch, response envelope
├── query_cli_test.go  # CLI integration tests for all four query patterns
└── xdg.go             # XDG data directory resolution (shared helper)

cmd/graphmd/main.go    # Add "import" and "query" cases to command switch
```

### Pattern 1: Import Pipeline

**What:** Extract ZIP → validate → copy to XDG storage → register as named graph
**When to use:** `graphmd import <file.zip> [--name <name>]`

```go
// Import pipeline steps:
// 1. Open ZIP file, extract to temp directory
// 2. Parse metadata.json → ExportMetadata
// 3. Validate schema: meta.SchemaVersion <= SchemaVersion (reject if newer)
// 4. Validate graph.db: open, check tables exist via LoadGraph
// 5. Compute destination: XDG_DATA_HOME/graphmd/graphs/<name>/
// 6. Copy graph.db + metadata.json to destination
// 7. Update "current" symlink or marker file

func GraphStorageDir() (string, error) {
    // $XDG_DATA_HOME/graphmd/graphs/ or ~/.local/share/graphmd/graphs/
    dataHome := os.Getenv("XDG_DATA_HOME")
    if dataHome == "" {
        home, err := os.UserHomeDir()
        if err != nil { return "", err }
        dataHome = filepath.Join(home, ".local", "share")
    }
    return filepath.Join(dataHome, "graphmd", "graphs"), nil
}
```

**Key insight:** The export ZIP format is already defined: `graph.db` + `metadata.json`. Import just unpacks, validates, and stores. The legacy `importtar.go` handles tar.gz format and is NOT relevant to the new ZIP-based workflow.

### Pattern 2: Named Graph Management

**What:** Each import creates a directory under `<xdg>/graphmd/graphs/<name>/`. A `current` marker file tracks the most recent import.

```
~/.local/share/graphmd/
└── graphs/
    ├── current           # text file containing name of most recent graph
    ├── prod-infra/
    │   ├── graph.db
    │   └── metadata.json
    └── staging/
        ├── graph.db
        └── metadata.json
```

**Graph name derivation:** If `--name` not provided, derive from ZIP filename stem (e.g., `graph.zip` → `graph`, `prod-infra-v2.zip` → `prod-infra-v2`).

### Pattern 3: Query Router

**What:** `graphmd query <subcommand> [flags]` dispatches to the appropriate handler.
**When to use:** All four query patterns.

```go
func CmdQuery(args []string) error {
    if len(args) < 1 {
        return queryUsageError()
    }
    subcommand := args[0]
    subArgs := args[1:]

    switch subcommand {
    case "impact":      return cmdQueryImpact(subArgs)
    case "dependencies": return cmdQueryDependencies(subArgs)
    case "path":        return cmdQueryPath(subArgs)
    case "list":        return cmdQueryList(subArgs)
    default:
        return unknownSubcommandError(subcommand)
    }
}
```

Each handler: parse flags → load graph from XDG → execute query → wrap in envelope → output JSON.

### Pattern 4: Response Envelope

**What:** Consistent JSON wrapper for all query responses per CONTEXT.md contract.

```go
// QueryEnvelope is the top-level JSON structure returned by all query commands.
// This replaces the existing QueryResult for CLI output (QueryResult remains
// for internal use by ExecuteImpact/ExecuteCrawl).
type QueryEnvelope struct {
    Query    QueryParams            `json:"query"`
    Results  interface{}            `json:"results"`
    Metadata QueryResponseMetadata  `json:"metadata"`
}

type QueryParams struct {
    Type       string  `json:"type"`       // "impact", "dependencies", "path", "list"
    Component  string  `json:"component,omitempty"`
    From       string  `json:"from,omitempty"`
    To         string  `json:"to,omitempty"`
    Depth      int     `json:"depth,omitempty"`
    MinConf    float64 `json:"min_confidence,omitempty"`
    Format     string  `json:"format,omitempty"`
}

type QueryResponseMetadata struct {
    ExecutionTimeMs int64  `json:"execution_time_ms"`
    NodeCount       int    `json:"node_count"`
    EdgeCount       int    `json:"edge_count"`
    GraphName       string `json:"graph_name"`
    GraphVersion    string `json:"graph_version"`
    CreatedAt       string `json:"created_at"`
    ComponentCount  int    `json:"component_count"`
}
```

**Key difference from current QueryResult:** The existing `QueryResult` struct (types.go:400) uses flat fields (`Root`, `Depth`, etc.) rather than the nested `{query, results, metadata}` envelope required by CONTEXT.md. The CLI layer wraps internal results into the envelope rather than modifying the internal struct.

### Pattern 5: Dependencies Query (Reverse Impact)

**What:** "What does X depend on?" follows incoming edges (ByTarget) instead of outgoing (BySource).

```go
// ExecuteDependencies follows incoming edges to find what a component depends on.
// This is the reverse of ExecuteImpact (which follows outgoing edges to find
// what breaks if a component fails).
func ExecuteDependencies(g *Graph, root string, depth int, minConf *float64, minTier *string) (*QueryResult, error) {
    // Use ByTarget[root] to find what root depends on
    // BFS/DFS following incoming edges with depth limit
}
```

**Important distinction:** Impact follows `BySource` (outgoing edges from root). Dependencies follows `ByTarget` (incoming edges to root, i.e., "what points to root" — but semantically we want "what root points to", which is actually `BySource`). Need to verify the edge direction semantics:
- Edge `A -> B` means "A references/depends-on B"
- Impact on A: "what depends on A" = follow `ByTarget[A]` (nodes that point TO A)
- Dependencies of A: "what does A depend on" = follow `BySource[A]` (nodes A points TO)

Wait — re-reading the existing code: `ExecuteImpact` uses `TraverseDFS` which follows `BySource` (outgoing edges). This means impact currently returns "what A affects downstream", not "what depends on A". Let me verify:

- `TraverseDFS(root)` follows `g.BySource[nodeID]` — outgoing edges from root
- For "if A fails, what breaks?" we need downstream dependents — things that depend ON A
- If edge direction is "A depends-on B" (A -> B), then to find "what breaks if B fails" we'd follow ByTarget[B] (incoming edges to B, i.e., things that depend on B)

**Actual semantics in code:** Looking at `GetImpact` and `TraverseDFS`, they follow `BySource` — outgoing edges. So an edge "A -> B" means "A depends on B", and `GetImpact("A")` returns things A reaches downstream. This is fine for impact IF edges mean "A references B" (A breaks if B fails, and B's dependents break transitively). The key point: **the existing traversal direction is correct for impact (outgoing), and dependencies should ALSO use outgoing (`BySource`) since "what does X depend on" means following X's outgoing dependency edges.** The difference between impact and dependencies is:
- **Impact:** "If X fails, what downstream systems break?" = follow `ByTarget[X]` — things that have X as a dependency
- **Dependencies:** "What does X need to work?" = follow `BySource[X]` — things X depends on

So: Impact needs a **new reverse traversal** following `ByTarget` (incoming edges). The existing `ExecuteImpact` actually follows `BySource`, which gives dependencies, not impact. This is a semantic alignment question the planner needs to handle. Looking at the test:

```go
// A -> B -> C -> D
// ExecuteImpact(root="A", depth=1) returns edge A->B
```

This test confirms: ExecuteImpact follows outgoing edges from root. If "A -> B" means "A depends on B", then this returns "what A depends on" (which is the dependencies query). For true impact ("if A fails, what breaks?"), we need things that depend on A — i.e., follow ByTarget[A].

**Resolution for the planner:** The existing `ExecuteImpact` function follows outgoing edges (BySource). The CONTEXT.md defines:
- Impact: "If component X fails, what breaks?" — needs reverse traversal (ByTarget)
- Dependencies: "What does component X depend on?" — needs forward traversal (BySource)

The current `ExecuteImpact` is actually more like a dependencies query. The planner should either:
1. Rename/repurpose `ExecuteImpact` for the dependencies query and write a new reverse traversal for impact
2. Or parameterize traversal direction

### Pattern 6: Fuzzy Component Name Matching

**What:** When a component is not found, suggest similar names.

```go
func suggestComponents(graph *Graph, query string, maxSuggestions int) []string {
    query = strings.ToLower(query)
    type scored struct {
        name  string
        score int
    }
    var candidates []scored
    for id := range graph.Nodes {
        lower := strings.ToLower(id)
        // Substring match
        if strings.Contains(lower, query) || strings.Contains(query, lower) {
            candidates = append(candidates, scored{id, 2})
            continue
        }
        // Prefix match
        if strings.HasPrefix(lower, query[:min(3, len(query))]) {
            candidates = append(candidates, scored{id, 1})
        }
    }
    // Sort by score desc, take top N
    // ...
}
```

Recommend substring + prefix matching over Levenshtein for simplicity. No external dependency needed.

### Pattern 7: Path Query with Per-Hop Confidence

**What:** Use existing `Graph.FindPaths(from, to, maxDepth)` and enrich paths with edge confidence.

```go
type PathResult struct {
    Paths []PathInfo `json:"paths"`
    Count int        `json:"count"`
}

type PathInfo struct {
    Nodes []string    `json:"nodes"`
    Hops  []HopInfo   `json:"hops"`
    TotalConfidence float64 `json:"total_confidence"` // product of hop confidences
}

type HopInfo struct {
    From             string  `json:"from"`
    To               string  `json:"to"`
    Confidence       float64 `json:"confidence"`
    ConfidenceTier   string  `json:"confidence_tier"`
    Type             string  `json:"type"`
    SourceFile       string  `json:"source_file"`
    ExtractionMethod string  `json:"extraction_method"`
}
```

`Graph.FindPaths` returns `[][]string` (node ID sequences). The CLI layer looks up the edge between consecutive nodes to populate `HopInfo`.

### Anti-Patterns to Avoid

- **Don't modify QueryResult struct for CLI output:** Wrap it in a `QueryEnvelope` instead. The internal struct is used by tests and the engine; CLI output is a different concern.
- **Don't use importtar.go for ZIP imports:** The legacy tar.gz import is retained for backward compatibility but is NOT the path for this phase. The new import handles ZIP format matching the export pipeline.
- **Don't load the graph for every query when using --graph:** Load once per command execution. Graph loading from SQLite is fast (<100ms for typical graphs) but there's no need for connection pooling or caching in a CLI tool.
- **Don't build a custom XDG library:** `os.Getenv("XDG_DATA_HOME")` + `os.UserHomeDir()` fallback is 5 lines of code. No dependency needed.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ZIP extraction | Custom decompressor | `archive/zip` stdlib | Already used in export; handles all edge cases |
| Graph traversal | New BFS/DFS for queries | Existing `TraverseDFS`, `FindPaths`, `computeDistances` | Thoroughly tested in Phase 2 |
| Confidence tier mapping | New tier logic | Existing `ScoreToTier()`, `TierAtLeast()` | 6-tier system fully implemented and tested |
| JSON serialization | Custom formatters | `json.MarshalIndent` | Standard approach used everywhere in codebase |
| Database schema validation | SQL introspection | `db.GetSchemaVersion()` comparison against `SchemaVersion` constant | Already implemented |
| Component type listing | New type registry | `AllComponentTypes()` | Returns all 12 types |

**Key insight:** This phase is primarily a CLI wiring exercise. The graph algorithms, confidence system, and persistence layer are all built. The work is: (1) import storage, (2) command routing, (3) response formatting.

## Common Pitfalls

### Pitfall 1: Edge Direction Confusion for Impact vs Dependencies

**What goes wrong:** Impact and dependencies queries return the same results because both follow the same edge direction.
**Why it happens:** Edge semantics ("A -> B" = "A references B") make it ambiguous whether impact should follow outgoing or incoming edges.
**How to avoid:** Clearly define: Impact follows `ByTarget` (reverse — what depends on X). Dependencies follows `BySource` (forward — what X depends on). Write explicit tests with directed graphs where the two queries must return different results.
**Warning signs:** Impact and dependencies return identical node sets for a non-symmetric graph.

### Pitfall 2: XDG Directory Permissions on First Run

**What goes wrong:** Import fails because `~/.local/share/graphmd/graphs/` doesn't exist.
**Why it happens:** First-time users have no XDG directory for graphmd.
**How to avoid:** Always `os.MkdirAll(dir, 0o755)` before writing. Import command should create the full directory tree.
**Warning signs:** "permission denied" or "no such file or directory" errors on fresh installs.

### Pitfall 3: Schema Version Comparison Direction

**What goes wrong:** Import accepts graphs from newer versions of graphmd that the current binary can't understand.
**Why it happens:** Comparing `meta.SchemaVersion >= SchemaVersion` when it should be `meta.SchemaVersion > SchemaVersion` (reject newer).
**How to avoid:** The rule is: reject if `meta.SchemaVersion > SchemaVersion`. Accept equal or older (older schemas are handled by migrations in `Migrate()`).
**Warning signs:** Queries return unexpected errors or missing columns after importing a graph from a newer version.

### Pitfall 4: Empty Results vs Errors

**What goes wrong:** "No path found" returns an error instead of success with empty results. Agents treat informational "no results" as failures.
**Why it happens:** Using the same error path for "component not found" (error) and "no path between components" (valid result).
**How to avoid:** Distinguish error conditions (component not found, no graph imported) from valid empty results (no path, no matching components). Error envelope for actual errors; success envelope with empty results + reason field for valid queries with no matches.
**Warning signs:** Exit code 1 for queries that successfully ran but found no results.

### Pitfall 5: `--depth all` Without Safety Bounds

**What goes wrong:** `--depth all` on a large graph causes excessive memory/time consumption.
**Why it happens:** Unbounded traversal on a graph with thousands of nodes.
**How to avoid:** Internally cap `--depth all` at a safety limit (e.g., 100, matching the existing `CrawlQuery` safety limit). Document this limit.
**Warning signs:** Query hangs or OOMs on large graphs.

### Pitfall 6: Named Graph Collision on Re-Import

**What goes wrong:** Importing a graph with a name that already exists silently overwrites without warning.
**Why it happens:** No check for existing graph with same name.
**How to avoid:** Overwrite is the correct behavior (import is idempotent — agents re-import updated graphs), but print a notice to stderr: "Replacing existing graph 'prod-infra'".
**Warning signs:** User confusion about which version of a graph is loaded.

## Code Examples

### ZIP Import with Validation

```go
func ImportZIP(zipPath string, graphName string) error {
    // Open ZIP
    r, err := zip.OpenReader(zipPath)
    if err != nil {
        return fmt.Errorf("import: open ZIP %q: %w", zipPath, err)
    }
    defer r.Close()

    // Extract to temp dir first
    tmpDir, err := os.MkdirTemp("", "graphmd-import-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmpDir)

    var hasDB, hasMeta bool
    for _, f := range r.File {
        if f.Name == "graph.db" { hasDB = true }
        if f.Name == "metadata.json" { hasMeta = true }
        // Extract each file to tmpDir...
    }

    if !hasDB || !hasMeta {
        return fmt.Errorf("import: invalid archive — missing graph.db or metadata.json")
    }

    // Parse and validate metadata
    metaBytes, _ := os.ReadFile(filepath.Join(tmpDir, "metadata.json"))
    var meta ExportMetadata
    json.Unmarshal(metaBytes, &meta)

    if meta.SchemaVersion > SchemaVersion {
        return fmt.Errorf("import: archive schema version %d is newer than supported version %d — upgrade graphmd",
            meta.SchemaVersion, SchemaVersion)
    }

    // Validate DB can be opened and loaded
    db, err := OpenDB(filepath.Join(tmpDir, "graph.db"))
    if err != nil {
        return fmt.Errorf("import: invalid graph database: %w", err)
    }
    testGraph := NewGraph()
    if err := db.LoadGraph(testGraph); err != nil {
        db.Close()
        return fmt.Errorf("import: graph database validation failed: %w", err)
    }
    db.Close()

    // Move to XDG storage
    destDir, _ := graphStoragePath(graphName)
    os.MkdirAll(destDir, 0o755)
    // Copy graph.db and metadata.json to destDir...

    // Update current marker
    setCurrentGraph(graphName)

    return nil
}
```

### Query Error Response

```go
func writeErrorJSON(w io.Writer, message, code string, suggestions []string) {
    resp := map[string]interface{}{
        "error": message,
        "code":  code,
    }
    if len(suggestions) > 0 {
        resp["suggestions"] = suggestions
    }
    if code == "NO_GRAPH" {
        resp["action"] = "run 'graphmd import <file.zip>' first"
    }
    json.NewEncoder(w).Encode(resp)
}
```

### Loading Named Graph for Queries

```go
func loadGraph(graphName string) (*Graph, *ExportMetadata, error) {
    storageDir, err := GraphStorageDir()
    if err != nil {
        return nil, nil, err
    }

    // Resolve graph name
    if graphName == "" {
        graphName, err = getCurrentGraph(storageDir)
        if err != nil {
            return nil, nil, fmt.Errorf("no graph imported")
        }
    }

    graphDir := filepath.Join(storageDir, graphName)
    dbPath := filepath.Join(graphDir, "graph.db")

    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        return nil, nil, fmt.Errorf("graph %q not found", graphName)
    }

    db, err := OpenDB(dbPath)
    if err != nil {
        return nil, nil, err
    }
    defer db.Close()

    graph := NewGraph()
    if err := db.LoadGraph(graph); err != nil {
        return nil, nil, err
    }

    // Load metadata
    metaBytes, _ := os.ReadFile(filepath.Join(graphDir, "metadata.json"))
    var meta ExportMetadata
    json.Unmarshal(metaBytes, &meta)

    return graph, &meta, nil
}
```

### Confidence Tier Enrichment

```go
// enrichConfidenceTier adds the tier name alongside the numeric score.
// Used in all query responses per CONTEXT.md contract.
func enrichConfidenceTier(confidence float64) (string, string) {
    if confidence < 0.4 {
        return fmt.Sprintf("%.2f", confidence), "below-threshold"
    }
    tier := ScoreToTier(confidence)
    return fmt.Sprintf("%.2f", confidence), string(tier)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tar.gz archives (`importtar.go`) | ZIP archives (`export.go`) | Phase 3 (2026-03-19) | Import must handle ZIP format, not tar.gz. Legacy `importtar.go` is retained but not used |
| In-memory graph only | SQLite-backed persistence | Phase 2 | `LoadGraph` reconstitutes full graph from SQLite — import stores the DB, queries load it |
| No structured query interface | `ExecuteImpact`, `ExecuteCrawl`, `QueryResult` | Phase 2 (02-05) | Core query engine exists; CLI wiring needed |

**Deprecated/outdated:**
- `ImportKnowledgeTar` / `LoadFromKnowledgeTar`: Legacy tar.gz import. Not relevant for Phase 4's ZIP-based workflow. Do not use or extend.
- `KnowledgeMetadata`: Legacy metadata struct for tar.gz. Phase 4 uses `ExportMetadata`.

## Open Questions

1. **Edge direction for impact vs dependencies**
   - What we know: `ExecuteImpact` follows `BySource` (outgoing edges). Edge "A -> B" means "A references B".
   - What's unclear: Is the current direction semantically correct for "if A fails, what breaks?" Impact should find things that DEPEND ON A (follow `ByTarget`), while the current implementation follows things A depends on (follow `BySource`).
   - Recommendation: The planner should verify edge semantics with the existing test suite and decide whether to: (a) keep `ExecuteImpact` as-is and use it for the dependencies query, creating a new reverse traversal for impact; or (b) swap the direction. The tests use a linear chain A->B->C->D and assert that `ExecuteImpact("A", 1)` returns edge A->B — which aligns with "dependencies of A" (what A reaches), not "impact of A failing" (what reaches A). **This is the highest-risk design question in the phase.**

2. **macOS XDG path behavior**
   - What we know: macOS doesn't natively use XDG. `$XDG_DATA_HOME` is typically unset.
   - What's unclear: Should we use `~/Library/Application Support/graphmd/` on macOS instead of `~/.local/share/graphmd/`?
   - Recommendation: Use `~/.local/share/graphmd/` on all platforms (consistent with Linux convention). `$XDG_DATA_HOME` override works if set. This is simpler and most graphmd users will be in Linux containers anyway.

3. **Table format scope**
   - What we know: `--format table` is requested for human debugging.
   - What's unclear: How much effort to invest in table formatting for all four query types.
   - Recommendation: Implement a minimal table format using `text/tabwriter` for list and impact queries. Path queries with complex hop-by-hop data can fall back to condensed text. Keep it simple — JSON is the primary output.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/knowledge/export.go` — ZIP packaging format, `ExportMetadata` struct
- Codebase analysis: `internal/knowledge/db.go` — `LoadGraph`, `SaveGraph`, `SchemaVersion`, `GetSchemaVersion`
- Codebase analysis: `internal/knowledge/query.go` — `ExecuteImpact`, `ExecuteCrawl`, `buildQueryResult`
- Codebase analysis: `internal/knowledge/graph.go` — `FindPaths`, `TraverseDFS`, `GetImpact`, `BySource`, `ByTarget`
- Codebase analysis: `internal/knowledge/types.go` — `QueryResult`, `AffectedNode`, `QueryEdge` structs
- Codebase analysis: `internal/knowledge/confidence.go` — `ScoreToTier`, `TierAtLeast`, 6-tier system
- Codebase analysis: `cmd/graphmd/main.go` — existing CLI command pattern (flag parsing, error handling)
- Codebase analysis: `internal/knowledge/importtar.go` — legacy import (for reference, not reuse)
- Codebase analysis: `internal/knowledge/command_helpers.go` — `findNodeForService`, `humanBytes`

### Secondary (MEDIUM confidence)
- Go stdlib documentation: `archive/zip`, `os.UserHomeDir()`, `text/tabwriter` — standard library usage patterns
- XDG Base Directory Specification — `$XDG_DATA_HOME` defaults to `$HOME/.local/share`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are already in use in the project; no new dependencies needed
- Architecture: HIGH — building blocks exist (traversal, persistence, confidence tiers); work is CLI wiring
- Pitfalls: HIGH — edge direction semantics identified as key risk; XDG and error handling patterns clear

**Research date:** 2026-03-23
**Valid until:** 2026-04-22 (stable domain — CLI patterns and Go stdlib don't change)
