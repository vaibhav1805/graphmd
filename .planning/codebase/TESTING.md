# Testing Patterns

**Analysis Date:** 2026-03-16

## Testing Framework
- **Framework:** Go standard library `testing` package
- **No external dependencies:** Pure stdlib, no test helper frameworks
- **Coverage:** 500+ test functions across 13+ test files

## Test Structure

### File Organization
- Test files co-located with source: `{name}_test.go`
- Same package scope: `package knowledge` (no separate test packages)
- Direct access to unexported functions and types

### Test File Locations
- `discovery_test.go` - Entity/relationship discovery tests
- `discovery_orchestration_test.go` - Discovery pipeline orchestration tests
- `components_discovery_test.go` - Component discovery tests
- `context_test.go` - Context and debugging tests
- `debug_context_test.go` - Debug context operations
- Plus 8+ additional test files covering core modules

## Test Patterns

### Table-Driven Tests
Primary testing pattern used extensively:

```go
tests := []struct {
    name     string
    input    InputType
    expected ExpectedType
}{
    {name: "simple case", input: ..., expected: ...},
    {name: "complex case", input: ..., expected: ...},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := functionUnderTest(tt.input)
        if !reflect.DeepEqual(result, tt.expected) {
            t.Errorf("got %v, want %v", result, tt.expected)
        }
    })
}
```

### Test Helpers
Consistent naming conventions for test fixtures:
- `make*` prefix for creating test objects (e.g., `makeTestDoc`, `makeGraph`)
- `build*` prefix for constructing complex structures
- Helpers defined in same test file or utility functions
- Test data located in `test-data/` directory

### Mocking Strategies
- **No external mocking framework** - uses interfaces and manual implementation
- Mock types implement required interfaces (e.g., mock database, mock extractors)
- Dependency injection through function parameters
- In-memory implementations for testing (e.g., in-memory graphs)

## Test Coverage Areas

### Discovery Tests
- Entity extraction from markdown (entities, topics, relationships)
- Component detection and classification
- Relationship inference and validation
- Pipeline orchestration and error handling

### Indexing Tests
- Document indexing and updates
- Full-text search (BM25) functionality
- Semantic search integration
- Query result ranking and relevance

### Graph Tests
- Graph construction and manipulation
- Node and edge management
- Graph traversal and algorithms
- Component graph operations

### Integration Tests
- End-to-end workflows (crawl → index → search)
- Database persistence and recovery
- Multi-document processing
- Error handling and edge cases

## Test Data Strategy

### Test Fixtures Location
- `test-data/` directory contains markdown samples and fixtures
- Organized by use case (complex graphs, simple documents, edge cases)
- Deterministic content for consistent test results
- Real markdown structure for testing extraction logic

### Determinism Testing
- Tests verify consistent behavior across multiple runs
- Relationship discovery verified for deterministic ordering
- Random seed control for algorithm testing
- Sorted results for reproducible comparisons

## Testing Best Practices

### Error Handling
- Tests verify error cases explicitly
- Nil pointer checks and panic prevention
- Database constraint violations caught
- Invalid input handling verified

### Test Isolation
- Each test is independent
- Test data cleaned up after execution
- No shared state between tests
- Database transactions or in-memory dbs for isolation

### Assertions
- Custom error messages for failures
- Comparing complex structures with `reflect.DeepEqual`
- Slice/map length and content verification
- String content validation with substring matching

## Current Testing Gaps

### Missing Coverage
- CLI flag parsing and validation
- Concurrent modification scenarios
- Large-scale data processing (performance benchmarks)
- Database corruption recovery
- Cache coherency under concurrent access
- LLM integration testing (determinism)
- Graph validation edge cases
