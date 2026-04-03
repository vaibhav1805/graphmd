# Coding Conventions

**Analysis Date:** 2026-03-16

## Language & Runtime

**Language:** Go 1.24.0

**Module:** `github.com/graphmd/graphmd`

## Naming Patterns

**Files:**
- Package-level source files: lowercase, underscore-separated words (e.g., `db.go`, `edge.go`, `discovery_orchestration.go`)
- Test files: suffix with `_test.go` (e.g., `components_test.go`, `discovery_orchestration_test.go`)
- Constants in files: match the module purpose (e.g., `db.go` for database constants)

**Functions & Methods:**
- Exported (public): PascalCase starting with capital letter (e.g., `NewEdge`, `OpenDB`, `DetectComponents`)
- Unexported (private): camelCase starting with lowercase letter (e.g., `edgeID`, `parseManifest`, `buildTFIDFMatrix`)
- Constructor functions: pattern is `New[Type]` (e.g., `NewEdge`, `NewDatabase`, `NewGraph`)
- Receiver methods: short receiver variable names (usually 1-2 letters), e.g., `(e *Edge)` for Edge methods, `(g *Graph)` for Graph methods
- Helper functions in tests: underscore-prefixed or prefixed with verb (e.g., `makeDiscoveredEdge`, `buildOrchTestDocs`, `makeServiceGraph`)

**Variables & Constants:**
- Constants: SCREAMING_SNAKE_CASE (e.g., `SchemaVersion`, `ConfidenceComponentFilename`, `inDegreeThreshold`)
- Struct field names: PascalCase for exported fields (e.g., `Edge.ID`, `Component.Name`, `Database.path`)
- Local variables: camelCase (e.g., `conf`, `nodeID`, `tmpDir`)
- Loop/iterator variables: single letter (e.g., `i`, `j`, `n`, or `src`, `dep` for semantic clarity in range loops)
- Receiver variables: single/double letter convention (e.g., `e`, `g`, `db`, `sd`)

**Types:**
- Struct names: PascalCase (e.g., `Component`, `Edge`, `Database`, `ComponentDetector`)
- Interface names: typically NOT used; this codebase uses struct types directly
- Type aliases for custom types: PascalCase (e.g., `EdgeType`, `SignalType`)
- Constant types: often string-based for human readability (e.g., `EdgeType string` not `iota`)

## Package Organization

**Package:** All core logic lives in `github.com/graphmd/graphmd/internal/knowledge/` package.
- Single package design: no sub-packages within `internal/knowledge/`
- Related functionality grouped by file (e.g., all component detection in `components.go` and `components_test.go`)
- Entry point in `cmd/graphmd/main.go` dispatches commands to knowledge package

**Imports:**
- Order (standard library first, then third-party, then internal):
  1. Standard library (fmt, os, strings, etc.)
  2. Third-party packages (gopkg.in/yaml.v3, github.com/anthropics/anthropic-sdk-go, etc.)
  3. No internal imports needed within single package

**Comments:**
- File-level doc comments: describe package and key abstractions (e.g., "Package knowledge — SQLite persistence layer...")
- Function doc comments: brief explanation of purpose, parameters, and error conditions
- Inline comments: explain complex algorithm decisions or non-obvious logic
- All exported symbols must have doc comments (enforced implicitly by convention)
- Comments use complete sentences with periods

## Code Style

**Formatting:**
- No explicit linter/formatter configuration found (likely uses `gofmt` defaults)
- Line length: appears to be standard ~100-120 characters based on observation
- Indentation: tabs (Go standard)
- Spacing: single blank line between functions/methods, no extra padding

**Error Handling:**
- Pattern: explicit error return as last return value
- Error checking: `if err != nil { return nil, err }` or `if err != nil { ... cleanup ... }`
- Error wrapping: use `fmt.Errorf` with `%w` for wrapping (e.g., `fmt.Errorf("knowledge.OpenDB: initialize: %w", err)`)
- Error messages: lowercase, begin with function/package name when context helpful (e.g., `"knowledge.NewEdge: source must not be empty"`)
- No custom error types; standard `error` interface used throughout

**Deferred cleanup:**
- Pattern: `defer db.Close()` immediately after successful creation
- No `defer` chains (one defer per resource)

## Function Design

**Size Guidelines:**
- Functions range 20-100 lines typically; longer orchestration functions (100-200 lines) exist for discovery logic
- Test functions: typically 10-40 lines per test case

**Parameters:**
- Limit to 3-4 parameters; use struct for options (e.g., `CrawlOptions`)
- Boolean parameters used rarely; prefer explicit option structs when multiple boolean flags

**Return Values:**
- Single return (value) for simple queries
- Multiple returns for operations with errors: `(value, error)`
- Sometimes `(slice, error)` for collection-returning functions
- Error always last return value

**Receiver Design:**
- Methods use pointer receivers for types that modify state (e.g., `(db *Database)`)
- Value receivers rare; only for small, immutable types

## Interface Patterns

**Type Assertions:**
- Struct-based composition over interfaces; code defines concrete types (Component, Edge, Graph)
- No formal interfaces defined in this codebase (keeps it simple)
- Methods operate directly on concrete types

## Testing Conventions

See TESTING.md for test-specific patterns.

## Comments & Documentation

**Doc Comments (Godoc style):**
- Start with function/type name (e.g., "// NewEdge creates and validates an Edge.")
- Detailed comments explain complex logic (e.g., confidence thresholds, algorithm rationale)
- Parameter constraints documented inline (e.g., "confidence must be in [0.0, 1.0]")

**Inline Comments:**
- Explain "why", not "what" — the code should be clear about what it does
- Used for heuristics and non-obvious choices (e.g., confidence thresholds, edge filtering rules)

**Type/Constant Comments:**
- Each exported type/constant has a comment (e.g., `// EdgeType categorises semantic relationships...`)
- Inline comments explain the semantic meaning of enum values

## Special Patterns

**Test Helpers:**
- Pattern: `func make[TypeName](...)` or `func build[TypeName](t *testing.T, ...) [Type]`
- Always accept `*testing.T` as first param for error reporting
- Use `t.Helper()` as first line to improve error messages
- Examples: `makeDiscoveredEdge`, `buildOrchTestDocs`, `makeServiceGraph`

**Database Layer:**
- Transaction wrapper pattern: operations that modify multiple tables wrapped in `BEGIN...COMMIT`
- Null-byte separator for composite keys: `source + "\x00" + target + "\x00" + type`

**Validation:**
- Constructor functions validate input (e.g., `NewEdge` checks source/target non-empty and confidence in [0, 1])
- Return explicit error with context on validation failure

---

*Convention analysis: 2026-03-16*
