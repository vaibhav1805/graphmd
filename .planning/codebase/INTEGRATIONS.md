# External Integrations

**Analysis Date:** 2026-03-16

## APIs & External Services

**Anthropic Claude API:**
- Claude AI for semantic relationship discovery (LLM-based)
  - SDK: `github.com/anthropics/anthropic-sdk-go v1.26.0`
  - Auth: Environment variables `ANTHROPIC_API_KEY` or `ANTHROPIC_KEY`
  - Implementation: `internal/knowledge/llm_discovery.go`
  - Optional flag: `--llm-discovery` when running `index` command
  - Models supported: claude-haiku-4-5-20251001, claude-opus-4-1-20250805 (or any Anthropic model)
  - Timeout: 30 seconds per API call

**URL-based Components:**
- HTTP/HTTPS URL detection in markdown
  - Implementation: `internal/knowledge/mention_patterns.go`
  - Pattern: `(?i)\bhttps?://[^\s/]*(?P<name>[\w][\w-]*)[^\s]*`
  - Confidence: 0.5 (low confidence URL pattern matching)
  - No actual HTTP calls; URLs are detected as text patterns and parsed for component names

## Data Storage

**Databases:**
- SQLite (embedded)
  - Location: `.bmd/knowledge.db` in indexed directory
  - Client: `modernc.org/sqlite` (pure Go driver)
  - Connection: `database/sql` standard package
  - Schema version: 2 (auto-migrated)
  - Features: WAL mode, foreign key constraints
  - Tables:
    - `documents` - Markdown file metadata (path, hash, mtime)
    - `index_entries` - Inverted index for full-text search (term → doc frequencies)
    - `bm25_stats` - BM25 corpus statistics (doc count, average doc length, term data)
    - `graph_nodes` - Knowledge graph vertices (id, type, file, title, metadata)
    - `graph_edges` - Knowledge graph edges (source, target, type, confidence, evidence)
    - `metadata` - Key-value store (schema versioning)

**File Storage:**
- Local filesystem only
  - Source: Markdown files in any directory structure
  - Index output: `.bmd/knowledge.db` (SQLite database)
  - Export output: tar.gz archives (gzip + tar)
  - No cloud file storage integration in code (except S3 via AWS CLI)

**Caching:**
- In-memory: BM25 index cache during knowledge construction
- File-based: LLM discovery cache (JSON file, location configurable via `LLMDiscoveryConfig.CacheFile`)
- No external cache service (Redis, Memcached, etc.)

## Authentication & Identity

**Auth Provider:**
- None for markdown sources (local files)
- Optional Anthropic API key for Claude LLM features
  - Env var check: `ANTHROPIC_API_KEY` first, fallback to `ANTHROPIC_KEY`
  - Error handling: Graceful degradation if key not set (LLM discovery skipped)

## Monitoring & Observability

**Error Tracking:**
- None detected (no Sentry, Datadog, etc.)

**Logs:**
- Standard output: JSON and formatted text output to stdout/stderr
  - Progress messages to stderr
  - Results to stdout
- No structured logging framework; uses standard Go fmt package
- Exit codes: 0 (success), 1 (error)

## CI/CD & Deployment

**Hosting:**
- Standalone binary (single statically-linked executable)
- No cloud platform integration required
- Can be deployed to any filesystem-accessible location

**CI Pipeline:**
- None detected in codebase
  - Uses Go's built-in testing (`*_test.go` files)
  - No GitHub Actions, GitLab CI, Travis CI configuration found
  - Relies on developer to run tests locally: `go test ./...`

## Environment Configuration

**Required env vars:**
- `ANTHROPIC_API_KEY` or `ANTHROPIC_KEY` - Optional for LLM discovery feature
  - Only required if using `--llm-discovery` flag or enabling semantic discovery
  - Not required for basic indexing, crawl, dependency analysis

**Optional env vars:**
- None other than Anthropic API key

**Secrets location:**
- None committed to repository
- API keys passed via environment variables only
- Configuration files (components.yaml) stored in indexed directory (not secrets)

## Webhooks & Callbacks

**Incoming:**
- None (graphmd is a CLI tool, not a server)

**Outgoing:**
- None (graphmd performs local analysis, no callback endpoints)

## AWS Integration

**S3 (Optional):**
- S3 bucket publishing for knowledge archives
  - Implementation: `internal/knowledge/export.go:PublishToS3()`
  - Method: AWS CLI via `exec.Command("aws", "s3", "cp", ...)`
  - Requires: AWS CLI installed locally (not a Go dependency)
  - Usage: `graphmd export --publish s3://bucket/path` flag
  - Returns error if AWS CLI not found

- S3 bucket downloading for knowledge imports
  - Implementation: `internal/knowledge/export.go:DownloadFromS3()`
  - Method: AWS CLI via `exec.Command("aws", "s3", "cp", ...)`
  - Requires: AWS CLI installed locally
  - Returns error if AWS CLI not found

**Authentication:**
- Delegated to AWS CLI (uses system AWS credentials)
- No Go-native S3 client library (aws-sdk-go not in go.mod)

## Archive/Distribution

**Export Format:**
- tar.gz with metadata
  - Contains: knowledge.json (metadata), markdown files, .bmd/knowledge.db
  - Includes: SHA256 checksum, semantic version, git provenance (optional)
  - Size: Variable (markdown files + SQLite database)

**Import Format:**
- tar.gz extraction to destination directory
  - Expected structure: knowledge.json, *.md files, .bmd/knowledge.db
  - Sanitization: Path traversal protection (rejects `..` paths)
  - Temporary cleanup: Downloaded S3 files removed after import

## Component Discovery

**Configuration Source:**
- components.yaml (optional YAML file in indexed directory)
  - Parsed by `NewComponentDiscovery()` in `internal/knowledge/components_discovery.go`
  - Defines custom component boundaries and types
  - Falls back to heuristic detection if not present

**Detection Methods:**
1. YAML config file (if present, highest priority)
2. Package markers in code (e.g., go:generate comments)
3. Conventional directory patterns (src/, lib/, services/)
4. Depth-based fallback (top-level directories)
5. Heuristic detection (high in-degree nodes in graph)

---

*Integration audit: 2026-03-16*
