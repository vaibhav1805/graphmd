---
status: complete
phase: 05-crawl-exploration
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md]
started: 2026-03-28T00:00:00Z
updated: 2026-03-28T00:00:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Crawl displays component stats
expected: Running `graphmd crawl --input ./test-data` displays component count, relationship count, and components grouped by type (e.g., services, databases).
result: pass

### 2. Confidence distribution with ASCII bars
expected: Output includes a confidence tier breakdown with counts, percentages, and ASCII bar charts (e.g., `strong (0.8-0.9): 23 edges (46%) ████████████`). Only non-zero tiers shown.
result: pass

### 3. Quality score displayed
expected: Output shows an overall quality score as a percentage (e.g., "Quality: 62.4%") reflecting the weighted average of confidence scores.
result: pass

### 4. Quality warnings shown
expected: Output includes a quality warnings section listing issues like orphan nodes (no relationships), dangling edge references, or weak-only components.
result: pass

### 5. JSON format output
expected: Running `graphmd crawl --input ./test-data --format json` outputs valid JSON. Check with `graphmd crawl --input ./test-data --format json | python3 -m json.tool`. Should contain summary, components (by_type), confidence tiers (with range arrays), and quality_warnings.
result: pass

### 6. Legacy crawl still works
expected: Running `graphmd crawl --from-multiple ./test-data` (the old targeted traversal mode) still works without errors. Should not conflict with the new stats mode.
result: pass
note: "Returns empty data without prior `graphmd index` — expected behavior, not a regression"

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
