# Architecture

**Analysis Date:** 2026-03-16

## Pattern Overview

**Overall:** Knowledge graph construction and querying system for markdown documentation

**Key Characteristics:**
- Command-driven CLI interface with stateless operations
- Document-centric graph model with typed, confidence-scored edges
- Multi-algorithm discovery pipeline (co-occurrence, structural, NER, semantic, LLM)
- SQLite persistence with in-memory graph manipulation
- Three-phase workflow: scan → discover → persist → query

## Layers

**Presentation Layer (CLI):**
- Purpose: Command parsing and user-facing output formatting
- Location: `cmd/graphmd/main.go`
- Contains: 9 command handlers (index, crawl, depends, components, context, relationships, graph, export, clean)
- Depends on: `internal/knowledge` package for all graph operations
- Used by: End users via `graphmd` binary

**Domain Layer (Knowledge Base):**
- Purpose: Core graph structures, discovery algorithms, and document processing
- Location: `internal/knowledge/`
- Contains: 61 Go files implementing graph traversal, document scanning, edge extraction, and discovery algorithms
- Depends on: Standard library, goldmark (markdown parsing), SQLite driver, Anthropic SDK
- Used by: CLI layer for all operations

**Persistence Layer (Database):**
- Purpose: SQLite-backed storage for graphs, indexes, and metadata
- Location: `internal/knowledge/db.go` (main interface)
- Contains: Schema initialization, CRUD operations for nodes/edges/indexes
- Depends on: SQLite via `modernc.org/sqlite`
- Used by: Knowledge package methods for Load/Save operations

**Indexing Layer (Search):**
- Purpose: Full-text search using BM25 ranking
- Location: `internal/knowledge/index.go`, `bm25.go`, `tokenizer.go`
- Contains: Tokenization, term frequency calculation, document ranking
- Depends on: Domain layer structures
- Used by: Search operations and semantic discovery

## Data Flow

**Indexing Pipeline (cmdIndex):**

1. `Knowledge.Scan(dir)` → walks directory tree, returns `[]Document` (markdown files only)
2. `LinkExtractor.Extract()` → parses markdown, extracts links → `[]*Edge` (confidence 1.0)
3. `MentionExtractor.Extract()` → pattern matching on prose → `[]*Edge` (confidence 0.7)
4. `CodeExtractor.Extract()` → code block parsing → `[]*Edge` (confidence 0.9)
5. `DiscoverRelationships()` → runs co-occurrence, structural, NER algorithms → `[]*DiscoveredEdge`
6. `FilterDiscoveredEdges()` → tiered confidence filtering → `[]*DiscoveredEdge`
7. `Graph.AddEdge()` → merges edges by confidence (deduplication)
8. `Database.SaveGraph()` → persists graph to SQLite

**Querying Pipeline (cmdCrawl/depends/components):**

1. `Database.LoadGraph()` → deserializes graph from SQLite → `*Graph`
2. `Graph.CrawlMulti()` (or `TransitiveDeps()`) → BFS/DFS traversal with options
3. Format output (JSON/DOT/tree/text)
4. Return to user

**Discovery Orchestration:**

- `DiscoverRelationships()` combines outputs from multiple algorithms
- `MergeDiscoveredEdges()` deduplicates edges with same source+target+type
- `FilterDiscoveredEdges()` applies tiered confidence thresholds based on signal count
- Edges with single strong signal pass immediately; weak signals pass only with corroboration

**State Management:**

- Documents: immutable after scanning; used for metadata and title extraction
- Graph: built in-memory, persisted to database; reloaded for each query operation
- Indexes: optional, built alongside graph for semantic search
- No cross-command state; each command is self-contained

## Key Abstractions

**Document:**
- Purpose: Represents a single markdown file with metadata
- Examples: `internal/knowledge/document.go` (Document struct)
- Pattern: Value type with computed fields (ID, ContentHash, PlainText)

**Node:**
- Purpose: Vertex in the knowledge graph
- Examples: `internal/knowledge/graph.go` (Node struct)
- Pattern: Lightweight label with ID and Title; stored in Graph.Nodes map

**Edge:**
- Purpose: Directed relationship between documents with confidence scoring
- Examples: `internal/knowledge/edge.go` (Edge struct)
- Pattern: Value type with ID, Source, Target, Type, Confidence, Evidence fields
- 6 typed relationship categories: references, depends-on, calls, implements, mentions, related

**Graph:**
- Purpose: In-memory knowledge graph with O(1) adjacency lookup
- Examples: `internal/knowledge/graph.go` (Graph struct)
- Pattern: Multi-map storage (Nodes, Edges, BySource, ByTarget) for efficient traversal
- Methods: AddNode, AddEdge, RemoveEdge, TraverseBFS, TransitiveDeps, FindPaths, DetectCycles, GetSubgraph

**Extractor:**
- Purpose: Multi-strategy relationship extraction from markdown
- Examples: `internal/knowledge/extractor.go` (Extractor struct)
- Pattern: Three extraction strategies applied sequentially (Link, Mention, Code)

**Database:**
- Purpose: SQLite persistence and schema management
- Examples: `internal/knowledge/db.go` (Database struct)
- Pattern: Wraps sql.DB; methods for SaveGraph/LoadGraph with transactional guarantees

**Discovery Algorithms:**
- Purpose: Find implicit relationships beyond explicit links
- Examples: `cooccurrence.go`, `structural.go`, `ner.go`, `semantic.go`, `llm_discovery.go`
- Pattern: Each returns `[]*DiscoveredEdge` with Signals and Confidence; merged and filtered

## Entry Points

**CLI Entry Point:**
- Location: `cmd/graphmd/main.go`
- Triggers: `graphmd <command> [options]`
- Responsibilities: Command dispatch, flag parsing, error handling, output formatting

**Indexing Command:**
- Location: `cmdIndex()` in `cmd/graphmd/main.go`
- Triggers: `graphmd index --dir DIR [--skip-discovery] [--llm-discovery]`
- Responsibilities: Orchestrate scan → extract → discover → save pipeline

**Query Commands:**
- Location: `cmdCrawl()`, `cmdDepends()`, `cmdComponents()`, `cmdContext()`, `cmdRelationships()`, `cmdGraph()`
- Triggers: `graphmd crawl|depends|components|context|relationships|graph [options]`
- Responsibilities: Load graph from DB, execute query (BFS/analysis), format output

## Error Handling

**Strategy:** Explicit error returns with contextual wrapping; no panics in library code

**Patterns:**
- Document scanning: soft-skip unreadable files; return error for directory access failures
- Link resolution: silent skip of unresolvable links; confidence downgrade to 0.5
- Database operations: transactional; rollback on error via `tx.Rollback()`
- Command execution: exit with code 1 on error; print to stderr

## Cross-Cutting Concerns

**Logging:** Uses `fmt.Fprintf(os.Stderr, ...)` for informational messages during long operations

**Validation:** Type constructors (e.g., `NewGraph()`, `NewDocument()`) validate non-nil and non-empty fields

**Deduplication:** Graph.AddEdge() is idempotent; duplicate edge IDs (same source+target+type) are silently dropped
Document merging: Higher-confidence edges always replace lower-confidence ones (confidence-based dedup)

**Determinism:**
- Document scanning: results sorted by RelPath
- Edge discovery: keys sorted before returning to ensure stable output across runs
- Database migrations: schema version tracked; old databases auto-migrated on open

**Performance Optimization:**
- BFS depth limiting in CrawlMulti to prevent full-graph traversal
- Link resolution caching via filepath operations
- SQLite WAL mode for improved concurrency
- Avoid repeated edge insertion via Graph.AddEdge() idempotency

---

*Architecture analysis: 2026-03-16*
