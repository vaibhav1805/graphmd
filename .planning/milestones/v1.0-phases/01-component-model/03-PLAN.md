---
wave: 3
depends_on: [01, 02]
files_modified:
  - test-data/component_types_corpus/
autonomous: false
---

# Plan 3: Quality Assurance & Validation

**Objective:** Ensure component types are correctly detected, persisted, and queryable across all code paths.

**Dependency:** Plan 01 (component type system must exist), Plan 02 (optional - for reference in validation)

## Task 3.1: Create QA Test Corpus & Validation Suite

Build a small markdown corpus with diverse components for testing type detection.

```xml
<task>
  <description>Create test-data/component_types_corpus/ with markdown files containing: services (payment-api.md, user-service.md), databases (postgres.md, mongodb.md), caches (redis.md), queues (rabbitmq.md, kafka.md), infrastructure (kubernetes.md, nginx.md). Each file describes the component in natural language. This corpus is used for: detecting types, measuring detection accuracy, validating persistence, testing list --type queries.</description>
  <acceptance_criteria>
    - [ ] Test corpus covers all 12 component types (at least one file per type)
    - [ ] Each file describes the component naturally (as a user might write)
    - [ ] Files are diverse: naming patterns, description styles, ambiguity levels
    - [ ] Component count: at least 20 distinct components across corpus
    - [ ] Corpus is reusable for regression testing and benchmarking
  </acceptance_criteria>
  <verification>Corpus is diverse, realistic, covers all types, can be exported and queried</verification>
</task>
```

## Task 3.2: Validate Type Detection Accuracy

Run detection pipeline on test corpus, measure accuracy, log confidence distribution.

```xml
<task>
  <description>Export test corpus using the Phase 1 detection pipeline. Measure detection accuracy: compare detected types to ground truth (manually annotated types in corpus). Log confidence distribution: how many components detected at 0.9+, 0.8-0.89, etc. Create a report showing accuracy by type and detection method. Success criteria: >80% accuracy on low-ambiguity types (service, database, cache), >70% on moderate-ambiguity types (message-broker, gateway).</description>
  <acceptance_criteria>
    - [ ] Accuracy measured for all 12 types
    - [ ] Confidence distribution logged
    - [ ] Report generated: accuracy by type, accuracy by detection method
    - [ ] Low-ambiguity types (service, database, cache) >80% accuracy
    - [ ] Moderate-ambiguity types >70% accuracy
    - [ ] Edge cases analyzed: unknown vs uncertain
  </acceptance_criteria>
  <verification>Report shows accuracy percentages, confidence distributions are logged, accuracy meets baselines</verification>
</task>
```

## Task 3.3: Test list --type Queries Against Test Corpus

Run comprehensive query tests: strict mode, inclusive mode, confidence filtering.

```xml
<task>
  <description>After export of test corpus: run graphmd list --type for each of the 12 types in strict mode. Verify result accuracy (all returned components are correct type). Run in inclusive mode (--include-tags) and verify tag matches are included. Verify confidence scores are present and in valid range. Test error cases: unknown type (should return empty), invalid flag combination, etc.</description>
  <acceptance_criteria>
    - [ ] Strict mode queries return only primary type matches
    - [ ] Inclusive mode (--include-tags) returns primary + tag matches
    - [ ] All returned components have valid types and non-null confidence
    - [ ] Confidence scores in [0.4, 1.0] range
    - [ ] JSON output is parseable and consistent
    - [ ] Unknown type query returns empty results (no error)
    - [ ] Edge cases handled (type with zero matches, very low confidence components)
  </acceptance_criteria>
  <verification>All queries return correct results; confidence scores are valid; JSON is parseable; no errors on edge cases</verification>
</task>
```

---

## Success Criteria (QA Completeness)

**Deliverables:**
- Comprehensive test corpus covering all 12 types
- Accuracy report with confidence distribution
- Query validation report showing all test cases passing
- Measured baseline: >80% accuracy on low-ambiguity types, >70% on moderate-ambiguity

**Impact:** Validates that Phase 1 implementation meets detection and query reliability expectations

---

## Delivery Artifacts

- `test-data/component_types_corpus/` — Diverse test corpus with ground truth annotations
- `ACCURACY_REPORT.md` — Type detection accuracy by type and method
- `QUERY_VALIDATION_REPORT.md` — list --type query test results

---

*Plan created: 2026-03-16*
*Requirement mapping: QA validation for COMP-01, COMP-02, COMP-03*
