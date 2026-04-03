# Technology Stack

**Analysis Date:** 2026-03-16

## Languages

**Primary:**
- Go 1.24.0 - All production code, CLI application, knowledge management service

**Secondary:**
- YAML - Configuration files (components.yaml for component discovery)
- JSON - Data serialization (archive metadata, edge information, graph exports)
- Markdown - Documentation source format being indexed

## Runtime

**Environment:**
- Go 1.24.0 runtime (macOS arm64 tested, cross-platform compatible)

**Package Manager:**
- Go Modules (go.mod, go.sum)
- Lockfile: Present (`go.sum` at `/Users/flurryhead/Developer/Opensource/graphmd/go.sum`)

## Frameworks

**Core:**
- Standard library networking (http filtering in URL detection)
- Standard library crypto (md5 for change detection, sha256 for checksums)
- Archive/tar and compress/gzip for knowledge export/import

**Parsing & Markup:**
- `github.com/yuin/goldmark v1.7.16` - Markdown parsing with AST walking for link extraction
  - Used in `internal/knowledge/extractor.go` for code block detection and language-specific imports
  - Enables link detection and relationship discovery from markdown files

**Database:**
- `modernc.org/sqlite v1.46.1` - SQLite database driver (pure Go implementation)
  - Used in `internal/knowledge/db.go` for knowledge graph persistence
  - Schema includes documents, index_entries, bm25_stats, graph_nodes, graph_edges, metadata tables
  - Implements WAL (Write-Ahead Logging) and foreign key constraints
  - Stores markdown file index, full-text search index, and relationship graph

**Testing:**
- `github.com/stretchr/testify v1.9.0` - Test assertions and mocking (indirect dependency)

**JSON/Data Processing:**
- `github.com/tidwall/gjson v1.18.0` - JSON parsing (indirect, used for metadata)
- `github.com/tidwall/sjson v1.2.5` - JSON mutation (indirect)
- `gopkg.in/yaml.v3 v3.0.1` - YAML parsing for components configuration

**Build/Dev Tools:**
- Go's built-in testing framework (no external test framework required)
- Compiler: `github.com/google/pprof` for profiling support (indirect)

## Key Dependencies

**Critical:**
- `github.com/anthropics/anthropic-sdk-go v1.26.0` - Claude API integration for LLM-powered discovery
  - Uses option package for API key configuration
  - Enables semantic relationship discovery via Claude prompts
  - Optional but enabled when `--llm-discovery` flag is used or configured
  - API key sourced from ANTHROPIC_API_KEY or ANTHROPIC_KEY environment variables

- `modernc.org/sqlite v1.46.1` - Embedded relational database for indexes
  - Pure Go SQLite driver (no C dependencies)
  - Stores relationship graph, markdown index, BM25 statistics
  - Schema version 2 with migrations support
  - Accessed via standard database/sql package

- `github.com/yuin/goldmark v1.7.16` - Markdown parsing for link extraction
  - AST-based parsing enables accurate link detection
  - Supports code fence language detection for import extraction

**Infrastructure:**
- `golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546` - Experimental stdlib features (maps, slices)
- `golang.org/x/sys v0.37.0` - Platform-specific system calls
- `golang.org/x/sync v0.17.0` - Concurrency utilities
- `google/uuid v1.6.0` - UUID generation for internal IDs
- `dustin/go-humanize v1.0.1` - Human-readable formatting

## Configuration

**Environment:**
- ANTHROPIC_API_KEY or ANTHROPIC_KEY - Claude API authentication (for LLM discovery)
  - Checked in `internal/knowledge/llm_discovery.go:newClaudeClient()`
  - Optional; gracefully degraded if not set

- Optional: components.yaml in indexed directory for custom component detection
  - Located at `{dir}/components.yaml`
  - Allows users to specify component boundaries and types
  - Loaded by `ComponentDiscovery` in `internal/knowledge/components_discovery.go`

**Build:**
- `go.mod` - Module definition and version constraints
- `go.sum` - Checksums for dependency verification
- No build configuration files (Makefile, gradle, webpack, etc.)
- Binary built directly: `go build -o graphmd ./cmd/graphmd/`

## Platform Requirements

**Development:**
- Go 1.24.0 or later
- Unix-like filesystem (symlink support recommended for test-data)
- AWS CLI (optional, for S3 export/import via `aws s3 cp`)

**Production:**
- Minimal: just Go runtime (no external services required)
- Local filesystem for markdown source and .bmd database directory
- Optional: AWS CLI for S3 integration (PublishToS3, DownloadFromS3)
- Optional: Claude API access for LLM-powered discovery

**Deployment Target:**
- Single binary executable (statically linked, no runtime dependencies)
- Cross-compiles to Linux, macOS, Windows
- Can embed into larger Go applications or run standalone CLI

---

*Stack analysis: 2026-03-16*
