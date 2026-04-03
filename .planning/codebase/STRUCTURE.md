# Directory Structure

**Analysis Date:** 2026-03-16

## Project Layout

```
graphmd/
├── cmd/
│   └── graphmd/
│       └── main.go              # CLI entry point
├── internal/
│   └── knowledge/               # Core package (all functionality)
│       ├── *.go                 # 40+ core modules
│       └── *_test.go            # 13+ test files
├── test-data/                   # Test fixtures and markdown samples
├── .gitignore
├── go.mod                       # Go module definition
├── go.sum                        # Dependency lock file
└── README.md                     # Project documentation
```

## Key Locations

### Entry Point
- `cmd/graphmd/main.go` - Single CLI command entry point for all operations

### Core Modules (`internal/knowledge/`)
The entire codebase lives in a single package with clear functional separation:

**Document & Graph Foundation**
- `document.go` - Document metadata and content representation
- `node.go` - Knowledge graph nodes
- `edge.go` - Knowledge graph edges
- `graph.go` - Graph structure and operations
- `registry.go` - Node/edge registry management

**Indexing & Querying**
- `index.go` - Main indexing interface
- `pageindex.go` - Per-document page indexing
- `bm25.go` - BM25 full-text search implementation
- `semantic.go` - Semantic search using embeddings
- `db.go` - SQLite persistence layer

**Document Processing**
- `scanner.go` - Markdown file scanning and traversal
- `extractor.go` - Markdown content extraction
- `chunk.go` - Content chunking for indexing
- `discovery.go` - Entity/relationship discovery
- `discovery_orchestration.go` - Discovery pipeline orchestration

**Search & Analysis**
- `crawl.go` - Graph traversal and crawling
- `component_search.go` - Component discovery
- `component_graph.go` - Component relationship graphs
- `crosssearch.go` - Cross-document search
- `algo_aggregator.go` - Search result aggregation
- `structural.go` - Structural analysis
- `tree.go` - Document tree representation
- `tree_builder.go` - Tree construction
- `svo.go` - Subject-verb-object triple extraction
- `hybrid_builder.go` - Hybrid search result building

**Discovery & Context**
- `manifest.go` - Project manifest handling
- `context_test.go` - Context-aware operations

**Advanced Features**
- `export.go` - Graph export functionality
- `debug_context_test.go` - Debug context utilities

### Test Files
13+ test files follow naming convention: `{module}_test.go`
- Comprehensive test coverage with 500+ test functions
- Test fixtures in `test-data/` directory
- Pure stdlib testing (no external frameworks)

## Naming Conventions

### File Organization
- Lowercase, snake_case filenames (e.g., `component_graph.go`)
- Test files co-located with source: `{name}_test.go`
- Single package: `internal/knowledge` (no subpackages)

### Code Organization
- **Exported symbols:** PascalCase (e.g., `Document`, `Graph`, `Node`)
- **Unexported symbols:** camelCase (e.g., `indexEntry`, `searchResult`)
- **Test helpers:** `make*`, `build*` prefix (e.g., `makeTestDoc`, `buildGraph`)

## Build Artifacts
- No checked-in binaries
- `go build ./cmd/graphmd` produces `graphmd` executable
- Single executable handles all CLI commands

## Configuration & Data
- No config files in repository
- Database created at runtime (SQLite)
- Cache directories created as needed
- Test data in `test-data/` for reproducible testing
