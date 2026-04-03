# Technical Concerns & Areas of Focus

**Analysis Date:** 2026-03-16

## Technical Debt

### 1. JSON Error Handling
- **Issue:** Error handling in JSON operations could be more robust
- **Impact:** JSON parsing failures may not surface clearly
- **Location:** Multiple modules handling JSON unmarshaling
- **Mitigation:** Add explicit error type checks and validation

### 2. Unimplemented CLI Commands
- **Issue:** Some CLI flags and subcommands referenced but not fully implemented
- **Impact:** Users may invoke commands that don't function properly
- **Location:** `cmd/graphmd/main.go` and command handlers
- **Fix Priority:** High - affects user experience

### 3. File Permissions Inconsistencies
- **Issue:** Generated files may have varying permission modes
- **Impact:** Issues with file access in shared environments
- **Location:** Export and file generation functions
- **Mitigation:** Standardize permission handling across file operations

## Known Bugs

### 1. Nil Pointer Dereference Risks
- **Issue:** Some code paths lack nil checks before dereferencing
- **Impact:** Potential panic in production on edge case inputs
- **Locations:**
  - Graph operations with missing edges
  - Document metadata access
  - Component relationship traversal
- **Fix:** Add defensive nil checks in critical paths

### 2. Circular Dependency Handling
- **Issue:** Circular references in document/component graphs not fully handled
- **Impact:** Infinite loops in traversal or cycle detection failures
- **Location:** `component_graph.go`, `tree_builder.go`, graph traversal functions
- **Reproduction:** Complex markdown with circular links
- **Test Coverage:** Missing determinism tests for cycles

### 3. Schema Migration Issues
- **Issue:** SQLite schema changes not fully backward compatible
- **Impact:** Database upgrade failures, migration errors
- **Location:** `db.go` initialization and migration logic
- **Mitigation:** Add migration versioning and rollback support

## Security Concerns

### 1. Subprocess Command Validation
- **Issue:** External process invocation needs validation
- **Impact:** Potential command injection if user input reaches subprocess calls
- **Severity:** Medium (depends on use case)
- **Review:** `scanner.go` and any system command execution

### 2. World-Readable Cache Files
- **Issue:** Generated cache may be stored with overly permissive access
- **Impact:** Information disclosure in multi-user systems
- **Location:** Cache directory creation and file output
- **Fix:** Set restrictive file permissions (0600/0700)

### 3. File Path Traversal
- **Issue:** User-provided paths may not be fully validated
- **Impact:** Access to files outside intended directory
- **Location:** `scanner.go`, manifest handling, file traversal
- **Mitigation:** Canonicalize and validate all file paths

### 4. SQL Safety
- **Issue:** Dynamic query construction could be vulnerable
- **Impact:** Potential SQL injection if input not properly sanitized
- **Location:** `db.go`, query building functions
- **Review:** Parameterized queries are used, verify all paths

### 5. MD5 Usage for Security
- **Issue:** MD5 used for fingerprinting/deduplication
- **Impact:** Not suitable for cryptographic purposes or integrity
- **Location:** Likely in content hashing or deduplication
- **Note:** Acceptable for non-security uses; document intent clearly

## Performance Concerns

### 1. BFS Bottlenecks
- **Issue:** Breadth-first search in large graphs may be slow
- **Impact:** Query latency for complex relationship traversal
- **Location:** `crawl.go`, graph traversal algorithms
- **Optimization Ideas:**
  - Bidirectional search
  - Pruning heuristics
  - Caching intermediate results

### 2. Batch Insert Limitations
- **Issue:** Database batch operations may have size constraints
- **Impact:** Slow indexing for large documents
- **Location:** `index.go`, `db.go` bulk insert operations
- **Optimization:**
  - Profile batch sizes
  - Consider transaction grouping
  - Parallel indexing within document

### 3. Full-Text Search Optimization
- **Issue:** BM25 implementation may not scale to very large corpora
- **Impact:** Search becomes slower with document growth
- **Location:** `bm25.go` scoring and indexing
- **Optimization Ideas:**
  - Incremental index updates
  - Range-based statistics refresh
  - Parallel query execution

### 4. File Scanning Performance
- **Issue:** Recursive markdown scanning may be slow on large directories
- **Impact:** Long initial indexing time
- **Location:** `scanner.go` directory traversal
- **Optimization:**
  - Parallel directory scanning
  - Skip patterns for common excluded dirs
  - Incremental scanning support

## Fragile Areas

### 1. Discovery Orchestration
- **Issue:** Complex pipeline with many interdependent stages
- **Impact:** Failures in one stage affect downstream processing
- **Location:** `discovery_orchestration.go`
- **Fragility:** Tightly coupled stages, error propagation unclear
- **Mitigation:** Add pipeline checkpointing and retry logic

### 2. LLM Cache Coherency
- **Issue:** Cached LLM results may become stale
- **Impact:** Outdated embeddings or analysis in downstream uses
- **Location:** Semantic search, embedding caching
- **Risk:** Silent data inconsistency, not detected until query anomalies
- **Solution:** Add cache invalidation and versioning

### 3. Component Graph Relationships
- **Issue:** Component relationships derived from heuristics, may be incomplete
- **Impact:** Missing or incorrect dependency analysis
- **Location:** `component_graph.go`, component discovery
- **Fragility:** Changes to detection heuristics may break assumptions
- **Testing:** Insufficient coverage for complex component scenarios

### 4. Chunk Indexing
- **Issue:** Text chunking strategy may fragment important concepts
- **Impact:** Search accuracy affected by chunk boundaries
- **Location:** `chunk.go`, `index.go`
- **Problem:** Fixed-size chunks may split semantic units
- **Enhancement:** Semantic chunk boundaries

### 5. Manifest Parsing
- **Issue:** Project manifest parsing may be brittle to format changes
- **Impact:** Configuration failures on manifest format evolution
- **Location:** `manifest.go`
- **Mitigation:** Schema versioning and graceful degradation

## Scaling Limits

### 1. SQLite Concurrency
- **Issue:** SQLite has limited concurrent write support
- **Impact:** Bottleneck for concurrent indexing/updates
- **Scaling Limit:** Single writer at a time
- **Solution:** Consider PostgreSQL for high-concurrency scenarios

### 2. In-Memory Graph Size
- **Issue:** Entire graph stored in memory for analysis
- **Impact:** Out-of-memory failures on very large codebases
- **Scaling Limit:** Depends on available RAM
- **Mitigation:** Streaming graph analysis, chunked processing

### 3. BM25 Statistics
- **Issue:** Full text search statistics may not scale
- **Impact:** Search quality degrades with corpus size
- **Solution:** Incremental statistics updates, sampling

### 4. Batch Operation Sizes
- **Issue:** Large batch operations may consume excessive memory
- **Impact:** Crashes on very large documents or updates
- **Location:** Bulk indexing, graph operations
- **Fix:** Implement adaptive batch sizing

### 5. LLM Rate Limiting
- **Issue:** Anthropic API rate limits may throttle operations
- **Impact:** Slow semantic indexing for large codebases
- **Mitigation:** Request batching, exponential backoff, rate limit tracking

## Dependencies at Risk

### 1. `modernc.org/sqlite`
- **Status:** Active maintenance, pure Go implementation
- **Risk:** Low - widely used, good compatibility
- **Monitor:** Breaking changes in SQLite schema support

### 2. `Anthropic SDK`
- **Status:** Official, actively maintained
- **Risk:** Medium - API changes could affect integration
- **Monitor:** API deprecations, rate limit changes

### 3. `goldmark` (Markdown Parser)
- **Status:** Active maintenance, good Go ecosystem coverage
- **Risk:** Low - stable markdown spec
- **Monitor:** CommonMark compliance updates

## Missing Features

### 1. Incremental Indexing
- **Current:** Full re-indexing required on document changes
- **Enhancement:** Track changed documents, update only affected indices
- **Impact:** Faster updates, better performance for large codebases

### 2. Export Wiring
- **Current:** Export functionality partially implemented
- **Enhancement:** Complete graph export with multiple formats (GraphQL, JSON-LD, RDF)
- **Impact:** Enables integration with external tools

### 3. Progress Indicators
- **Current:** Long-running operations provide no feedback
- **Enhancement:** Add progress bars and status reporting
- **Impact:** Better user experience for large operations

### 4. Graph Validation
- **Current:** Limited validation of graph consistency
- **Enhancement:** Automatic cycle detection, orphan node cleanup, integrity checks
- **Impact:** Detect and prevent corrupted graphs

## Test Coverage Gaps

### 1. CLI Argument Parsing
- **Status:** Limited testing of command-line flag combinations
- **Impact:** Unknown interactions between flags

### 2. Concurrent Modification
- **Status:** No tests for concurrent indexing/updates
- **Impact:** Race conditions in production

### 3. Database Corruption
- **Status:** No recovery testing from corrupted databases
- **Impact:** Unknown behavior on DB failures

### 4. Cache Corruption
- **Status:** Missing tests for corrupted cache recovery
- **Impact:** Stale data on cache failures

### 5. Large-Scale Operations
- **Status:** No benchmark tests for scaling limits
- **Impact:** Unknown performance characteristics at scale
