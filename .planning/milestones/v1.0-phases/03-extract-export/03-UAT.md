---
status: complete
phase: 03-extract-export
source: [03-01-SUMMARY.md, 03-02-SUMMARY.md]
started: 2026-03-19T17:00:00Z
updated: 2026-03-23T00:00:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Export command produces ZIP
expected: Running `go build ./cmd/graphmd/ && ./graphmd export --input ./test-data --output /tmp/test-graph.zip` produces a ZIP file at /tmp/test-graph.zip without errors.
result: pass
note: "User observed graph dependencies and crawl output are empty — will check in subsequent tests"

### 2. ZIP contains exactly graph.db and metadata.json
expected: Running `unzip -l /tmp/test-graph.zip` shows exactly 2 entries: `graph.db` and `metadata.json`. No markdown files, no other artifacts.
result: pass

### 3. Metadata has required fields
expected: Running `unzip -p /tmp/test-graph.zip metadata.json | python3 -m json.tool` shows valid JSON with fields: version, schema_version (5), created_at (ISO 8601), component_count (> 0), relationship_count (>= 0), input_path.
result: pass

### 4. Components extracted from markdown
expected: Extract graph.db and run `sqlite3 /tmp/graph.db "SELECT title, component_type FROM graph_nodes LIMIT 10"`. Should show components with types (not all 'unknown'), extracted from test-data markdown files.
result: pass

### 5. Schema v5 indexes present
expected: Running `sqlite3 /tmp/graph.db "SELECT name FROM sqlite_master WHERE type='index'"` shows idx_nodes_title and idx_edges_confidence among the indexes.
result: pass

### 6. .graphmdignore defaults work
expected: Export does not scan hidden directories or common build dirs. The component list should not include files from .git, vendor, node_modules, etc. (verify by absence of such paths in graph_nodes).
result: pass

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
