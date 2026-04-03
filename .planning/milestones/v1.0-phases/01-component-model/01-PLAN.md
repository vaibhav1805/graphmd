---
wave: 1
depends_on: []
files_modified:
  - internal/knowledge/types.go
  - internal/knowledge/components.go
  - internal/db/schema.go
  - cmd/graphmd/main.go
  - docs/COMPONENT_TYPES.md
  - docs/CLI_REFERENCE.md
autonomous: false
---

# Plan 1: Component Type System Definition & Persistence

**Objective:** Define the 12-type component taxonomy, persist types in SQLite schema and graph structure, enable type-based queries.

**Requirements covered:** COMP-01, COMP-02, COMP-03

## Task 1.1: Define Component Type Constants & Taxonomy

Define the 12-type taxonomy as Go constants with documentation for each type. Create a seed config schema for user-extensibility.

```xml
<task>
  <description>Create types.go with component type constants, type mapping utilities, and type validation functions. Include the 12-type taxonomy from Backstage/Cartography patterns: service, database, cache, queue, message-broker, load-balancer, gateway, storage, container-registry, config-server, monitoring, log-aggregator. Add an "unknown" default type.</description>
  <acceptance_criteria>
    - [x] Component type constants defined (12 types + unknown)
    - [x] Type validation function checks membership in valid types
    - [x] Type descriptions provided for each type (for documentation generation)
    - [x] Seed config YAML schema defined (for user custom type mappings)
    - [x] Unit tests verify all 12 types are discoverable and valid
  </acceptance_criteria>
  <verification>Type constants can be listed; validation passes for all 12 types + unknown; fails for invalid types</verification>
</task>
```

## Task 1.2: Extend SQLite Schema with Component Classification

Add `component_type` column to the nodes/components table, create `component_mentions` table for tracking detection locations.

```xml
<task>
  <description>Update the SQLite schema migration to add component_type column (TEXT NOT NULL DEFAULT 'unknown') to the components table. Create component_mentions table tracking: component_id, file_path, heading_hierarchy, detected_by (detection method), confidence. This enables provenance tracking and "where was this found" queries.</description>
  <acceptance_criteria>
    - [x] Schema migration creates component_type column on components table
    - [x] component_type defaults to 'unknown' for backward compatibility
    - [x] component_mentions table created with full provenance fields
    - [x] Existing test data survives migration without errors
    - [x] Schema validation queries work: SELECT DISTINCT component_type FROM components
  </acceptance_criteria>
  <verification>Schema migrations apply cleanly; SELECT on new columns succeeds; constraints enforced</verification>
</task>
```

## Task 1.3: Wire Component Detection to Type Persistence

Update component detection pipeline to classify types and persist to SQLite. Integrate with existing detection algorithms.

```xml
<task>
  <description>Modify the component detection functions (in components.go and components_discovery.go) to output component types alongside component names. Wire detection output to the persistence layer, saving type + confidence + detection method to components and component_mentions tables. Use ensemble voting from existing detection algos (patterns, NER, semantic, etc.) to determine type.</description>
  <acceptance_criteria>
    - [x] Detection pipeline returns (component_name, component_type, confidence, detection_methods)
    - [x] Persistence layer saves component_type and confidence correctly
    - [x] component_mentions table populated with file path, heading context, detection method
    - [x] Type classification succeeds on test corpus with >80% accuracy (baseline for low-ambiguity cases)
    - [x] Unknown types default gracefully without errors
  </acceptance_criteria>
  <verification>Export produces components with non-null component_type; component_mentions table has full provenance; no NULL type values except explicitly set unknown</verification>
</task>
```

## Task 1.4: Implement CLI list --type T Filter

Add CLI command for type-filtered queries. Support strict (primary type only) and inclusive (primary type + tags) matching.

```xml
<task>
  <description>Implement the graphmd list --type command with support for strict mode (default: primary type matches only) and optional --include-tags flag (include tag matches). Output JSON with match metadata: component name, type, match_type (primary vs tag), confidence, tags. Include summary showing primary vs tag match counts.</description>
  <acceptance_criteria>
    - [x] Command graphmd list --type service succeeds, returns only service-typed components
    - [x] Command graphmd list --type service --include-tags includes tag matches
    - [x] Output includes match_type field indicating primary vs tag match
    - [x] Confidence scores included in output for agent assessment
    - [x] Filter summary shows match counts and notes about inclusive vs strict mode
    - [x] Error handling for unknown types (returns zero results, not error)
  </acceptance_criteria>
  <verification>list --type returns correct subsets; confidence scores are numeric and in [0,1]; JSON is parseable; strict vs inclusive modes produce different result counts</verification>
</task>
```

## Task 1.5: Add Unit & Integration Tests

Comprehensive test suite for type classification, persistence, and querying.

```xml
<task>
  <description>Add unit tests covering: type constant validation, detection ensemble voting, persistence correctness, and CLI filtering. Add integration test: export test corpus -> verify component_type column populated -> run list --type queries -> verify result accuracy and confidence scores.</description>
  <acceptance_criteria>
    - [x] Unit tests for all 12 types + unknown type validation
    - [x] Detection ensemble voting tested with mock inputs
    - [x] Integration test: end-to-end from detection to export to list query
    - [x] Test coverage for both strict and inclusive (--include-tags) modes
    - [x] Tests verify confidence scores are within valid range [0.4, 1.0]
  </acceptance_criteria>
  <verification>All tests pass; coverage >85% for types.go, components.go; integration test produces expected component counts and types</verification>
</task>
```


---

## Success Criteria (Phase Goal Verification)

**Phase Goal:** Refine the component model so every graph node carries a typed classification that AI agents can filter and query.

**Must-haves for goal achievement (Plan 1):**

1. ✓ **Type persistence:** After export, `SELECT DISTINCT component_type FROM graph_nodes` returns all 12 types from test corpus. No NULL values except explicitly unknown. Confidence scores present on all rows.

2. ✓ **Schema validation:** SQLite schema includes `component_type TEXT NOT NULL DEFAULT 'unknown'` on graph_nodes table. `component_mentions` table has provenance fields (file_path, heading_hierarchy, detected_by, confidence).

3. ✓ **Type query capability:** `graphmd list --type service` returns only service-typed components with zero false positives. Response includes match_type and confidence for agent assessment.

4. ✓ **Coverage & extensibility:** All 12 taxonomy types defined as code constants. Users can add custom types via seed config (mechanism implemented).

---

## Delivery Artifacts (Plan 1)

- **Code:** types.go (constants), components.go (detection updates), db.go (schema migrations), main.go (CLI command)
- **Tests:** Unit + integration tests with >85% coverage for types, schema, detection, and queries
- **Git commits:** 6 atomic commits per task

**See also:**
- **Plan 2:** User-facing documentation (docs/COMPONENT_TYPES.md, CLI_REFERENCE.md, ADR, SEED_CONFIG_GUIDE.md, README)
- **Plan 3:** QA validation (test corpus, accuracy reports, query validation)

---

*Plan created: 2026-03-16*
*Requirement mapping: COMP-01, COMP-02, COMP-03*
