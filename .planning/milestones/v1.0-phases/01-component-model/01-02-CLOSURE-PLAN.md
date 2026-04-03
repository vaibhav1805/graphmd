---
phase: 01-component-model
plan: 01-closure-plans-2-3
subsystem: knowledge
tags: [user-docs, qa-validation, component-types, seed-config]
wave: 2
autonomous: true
gap_closure: true

depends_on: [01-01-CLOSURE-PLAN]
blocks: [02-accuracy-foundation]

files_modified:
  - docs/COMPONENT_TYPES.md (new)
  - docs/CLI_REFERENCE.md (new)
  - docs/ADR_COMPONENT_TYPES.md (new)
  - docs/CONFIGURATION.md (new)
  - README.md (updated)
  - internal/knowledge/component_types_test.go (enhanced)

must_haves:
  - User-facing component type taxonomy and guide created
  - CLI reference for list --type command documented
  - Seed config extensibility mechanism documented with examples
  - QA test corpus expanded with edge cases
  - All user docs follow CLAUDE.md standards (no internal process language)
---

# Phase 1 Closure Plan 2: User-Facing Documentation & QA Validation

**Execute Plans 2 and 3 from Phase 1 roadmap: Create permanent user documentation and QA validation test suite**

## Objective

Close **GAP-4**: Plans 2 and 3 remain unexecuted. These are not blocking Phase 1 goal achievement (typed nodes exist and are queryable), but they are essential for:

1. **User adoption:** AI agents need documentation to understand component types, seed config, and query behavior
2. **Quality assurance:** Type detection accuracy has not been measured across diverse corpora
3. **Permanence:** User docs must appear standalone and permanent (per CLAUDE.md), not tied to build phases

This plan executes both Plans 2 and 3 as a single, focused closure.

## Rationale

- Phase 1 goal is mechanically complete: code delivers typed nodes and queries. But AI agents cannot use the feature without understanding the taxonomy, confidence behavior, and customization options.
- Plans 2–3 are quality gates that establish credibility: type detection accuracy measured, docs written by humans, not machines.
- Deferred Plans 2–3 create a "half-shipped" feeling. Executing them closes Phase 1 completely.

## Tasks

<task id="1" name="Create docs/COMPONENT_TYPES.md - Component Type Taxonomy Reference">
**User-facing guide to all 12 component types with detection patterns and examples**

Create `docs/COMPONENT_TYPES.md` containing:

**Section 1: Overview**
- Explain component types as a classification system for infrastructure components
- State that every component has a primary type (required) + optional tags
- Note that users can define custom types via seed config

**Section 2: Type Definitions (12 core types)**

For each type, document:
- **Name:** Type identifier (exact as it appears in queries)
- **Description:** 1–2 sentence explanation of what components fall into this category
- **Examples:** 2–3 real examples (PostgreSQL, Redis, Kubernetes, etc.)
- **Detection patterns:** How auto-detection recognizes this type (file paths, naming conventions, keywords)
- **Confidence thresholds:** Typical confidence range (0.65–1.0)

Types to document (from `internal/knowledge/types.go`):
1. `service` — application or microservice component
2. `database` — data storage (relational, NoSQL, etc.)
3. `cache` — in-memory cache layer (Redis, Memcached, etc.)
4. `queue` — message queue (RabbitMQ, SQS, Kafka, etc.)
5. `message-broker` — message routing infrastructure (Kafka, NATS, etc.)
6. `load-balancer` — traffic distribution (HAProxy, AWS ELB, nginx, etc.)
7. `gateway` — API gateway or service gateway (Kong, Traefik, etc.)
8. `storage` — object/blob storage (S3, GCS, etc.)
9. `container-registry` — image repository (Docker Hub, ECR, etc.)
10. `config-server` — configuration management (Consul, etcd, etc.)
11. `monitoring` — metrics/observability component (Prometheus, Datadog, etc.)
12. `log-aggregator` — centralized logging (ELK, Splunk, etc.)

**Section 3: Unknown Type**
- Explain `unknown`: default type when no detection algorithm can classify confidently
- Note that `unknown` is not a failure; it indicates ambiguous or insufficient signal
- Encourage seed config override for persistent misclassifications

**Section 4: Tags (Optional Secondary Classification)**
- Explain tags as optional metadata (criticality, deployment model, compliance, etc.)
- Provide examples: `["critical", "internal-only", "deprecated", "compliance-critical"]`
- Note that tags are searchable via `--include-tags` flag

**Writing style (CLAUDE.md compliant):**
- No "Phase 1 adds types" — write as if types always existed
- No internal process language (phase, milestone, GSD)
- Focus on user workflows: "Every component carries a type classification. Use list --type to query by type."

**Verification:** File is readable, complete, and follows CLAUDE.md documentation standards.
**Commit:** `docs(01-02-closure): add COMPONENT_TYPES.md taxonomy reference`
</task>

<task id="2" name="Create docs/CLI_REFERENCE.md - CLI Command Documentation">
**Reference documentation for graphmd list --type and related commands**

Create `docs/CLI_REFERENCE.md` containing:

**Section 1: graphmd list --type**
- **Command:** `graphmd list [--type TYPE] [--include-tags] [--output json|table]`
- **Description:** List components, optionally filtered by type
- **Flags:**
  - `--type TYPE`: Filter by component type (e.g., `service`, `database`); default: list all
  - `--include-tags`: Include components matching type as a tag (not just primary type)
  - `--output json|table`: Output format; default: `table`
  - `--dir PATH`: Input markdown directory; default: `./docs`
- **Examples:**
  ```bash
  # List all components
  graphmd list --dir ./docs

  # List only services
  graphmd list --type service --dir ./docs

  # List all database-related components (primary type + tags)
  graphmd list --type database --include-tags --dir ./docs

  # Output as JSON for machine parsing
  graphmd list --type service --output json --dir ./docs
  ```
- **Output format (JSON):**
  ```json
  {
    "components": [
      {
        "name": "postgres-primary",
        "type": "database",
        "match_type": "primary_type",
        "confidence": 0.95,
        "tags": ["critical", "high-availability"]
      }
    ],
    "filter": {
      "query": "--type database",
      "mode": "strict",
      "primary_matches": 5,
      "tag_matches": 2,
      "note": "Showing primary type matches. 2 additional components match as tags only."
    }
  }
  ```

**Section 2: graphmd index**
- Document that `index` command classifies components by type during pipeline
- Note that types are persisted to SQLite schema
- Example: `graphmd index --dir ./docs` creates `.bmd/knowledge.db` with typed components

**Section 3: graphmd export** (preview for Phase 3)
- Note that exported SQLite includes `component_type` column
- Agents can query: `SELECT DISTINCT component_type FROM graph_nodes`

**Writing style:**
- Emphasize machine-readable JSON output for AI agents
- Include concrete examples with expected output
- No phase/process language; write as permanent reference

**Verification:** File covers all relevant commands, includes examples and JSON output, is formatted for easy reference.
**Commit:** `docs(01-02-closure): add CLI_REFERENCE.md with command documentation`
</task>

<task id="3" name="Create docs/ADR_COMPONENT_TYPES.md - Architecture Decision Record">
**Explain why component types exist and how they work (design rationale)**

Create `docs/ADR_COMPONENT_TYPES.md` (Architecture Decision Record format):

**Sections:**
- **Problem:** AI agents need to answer "what depends on my database?" without being fed entire architecture. Querying by component type enables targeted incident response.
- **Solution:** Every component carries a type classification (service, database, cache, etc.). Queries can filter by type.
- **Design Decisions:**
  1. Why 12 types? Based on Backstage/Cartography patterns; covers common infrastructure components
  2. Why confidence scores? Agents need to assess detection reliability; not all types are equally certain
  3. Why seed config? Users may have custom types or persistent misclassifications; need a way to override without code changes
  4. Why tags? Primary type may not capture all relevant metadata; tags enable nuanced classification
- **Trade-offs:**
  - Flexibility vs. Simplicity: Tags add complexity but enable accurate classification without creating type hierarchies
  - Auto-detection vs. Manual: Seed config enables overrides but requires user effort
- **Related Documents:** COMPONENT_TYPES.md (taxonomy), CLI_REFERENCE.md (usage)

**Verification:** ADR explains the "why" behind component types, is written in permanent language, and helps users understand design intent.
**Commit:** `docs(01-02-closure): add ADR_COMPONENT_TYPES.md explaining design decisions`
</task>

<task id="4" name="Create docs/CONFIGURATION.md - Seed Config Guide">
**Guide to customizing component types via seed config**

Create `docs/CONFIGURATION.md` containing:

**Section 1: Seed Config Overview**
- Explain SeedConfig as a mechanism for users to override auto-detection
- Note that seed config entries always win (confidence 1.0)
- Purpose: Handle persistent misclassifications and define custom types

**Section 2: Seed Config File Format**
- YAML format (user-friendly)
- Example:
  ```yaml
  # custom_types.yaml
  seed_mappings:
    - pattern: "postgres*"
      type: "database"
      tags: ["critical"]
    - pattern: "services/api-*"
      type: "service"
      tags: ["internal"]
    - pattern: "monitoring/prometheus"
      type: "monitoring"
      confidence_override: 1.0
  ```

**Section 3: Pattern Matching**
- Explain glob patterns: `redis*` matches `redis-primary`, `redis-cache`, etc.
- Explain folder-based patterns: `services/` prefix
- Note: Patterns are matched against component name (file path or heading)

**Section 4: Examples**
- Example 1: Custom type for internal tools
- Example 2: Force-override a persistent misclassification
- Example 3: Add tags to auto-detected components

**Section 5: Loading Seed Config**
- How to specify custom config file to `graphmd index --seed-config ./custom.yaml`
- Default behavior (no seed config file): auto-detection only

**Verification:** Guide is practical, includes working examples, and is written for users (not developers).
**Commit:** `docs(01-02-closure): add CONFIGURATION.md seed config guide`
</task>

<task id="5" name="Update README.md with Component Model Overview">
**Add component model section to public README**

Update `README.md`:

1. Add new section: **Component Classification**
   - Explain that every component carries a type
   - List example types (service, database, cache, etc.)
   - Show example query: `graphmd list --type service`
   - Link to docs/COMPONENT_TYPES.md and docs/CLI_REFERENCE.md

2. Add example in **Quick Start:**
   - Run `graphmd index --dir ./docs`
   - Run `graphmd list --type service --output json`
   - Show sample JSON output

3. No phase/process language; write as permanent feature documentation

**Verification:** README includes component model as first-class feature, with examples and links to detailed docs.
**Commit:** `docs(01-02-closure): update README.md with component classification overview`
</task>

<task id="6" name="Expand QA Test Suite: Edge Cases & Accuracy Measurement">
**Enhance internal/knowledge/component_types_test.go with QA coverage**

Add new tests to `internal/knowledge/component_types_test.go` (or new file `internal/knowledge/qa_validation_test.go`):

**Test 1: Type Detection Accuracy Across Diverse Components**
- Index a test corpus with known component types (annotated manually)
- Measure detection accuracy by type:
  - Service detection accuracy: X%
  - Database detection accuracy: Y%
  - etc.
- Assert minimum accuracy thresholds (e.g., >70% for low-ambiguity types)

**Test 2: Seed Config Override Behavior**
- Create seed config with custom mappings (redis* → cache, postgres* → database)
- Index corpus with seed config enabled
- Assert all matched components have type from seed config (not auto-detected)
- Assert confidence = 1.0 for seed-matched components

**Test 3: Unknown Type Fallback**
- Index corpus with ambiguous component names
- Assert components without clear detection default to `unknown`
- Assert `unknown` count is < 10% of total (most components classified)

**Test 4: Tag Application & Filtering**
- Index corpus with components tagged as critical, deprecated, etc.
- Run `list --type service --include-tags` query
- Assert both primary-type and tag-matched results appear
- Assert match_type field distinguishes primary vs tag matches

**Test 5: Confidence Score Distribution**
- Index corpus and collect confidence distribution
- Assert distribution follows expected pattern (most components > 0.8 confidence)
- Log distribution: "confidence 0.95+: 45%, 0.80–0.94: 30%, 0.65–0.79: 20%, < 0.65: 5%"

**Test 6: Performance on Large Corpus** (optional, future)
- Index 500+ documents
- Assert type detection completes in < 5 seconds
- Assert queries run in < 100ms

**Verification:** All tests pass, accuracy thresholds met, distribution aligns with expectations.
**Commit:** `test(01-02-closure): add QA validation test suite for type detection accuracy`
</task>

<task id="7" name="Update ROADMAP.md Phase 1 Plans Section">
**Mark Plans 2 and 3 as complete in ROADMAP**

Update `.planning/ROADMAP.md`, section "Phase 1: Component Model" → "Plans":

Change:
```
| # | Plan | Status |
|---|------|--------|
| 1 | Component Type System Definition & Persistence | Complete (2026-03-16) |
| 2 | User-Facing Documentation & Guide | Not started |
| 3 | Quality Assurance & Validation | Not started |
```

To:
```
| # | Plan | Status |
|---|------|--------|
| 1 | Component Type System Definition & Persistence | Complete (2026-03-16) |
| 2 | User-Facing Documentation & Guide | Complete (2026-03-19) |
| 3 | Quality Assurance & Validation | Complete (2026-03-19) |
```

Also update STATE.md if needed to reflect Phase 1 completion.

**Verification:** ROADMAP and STATE reflect closure completion.
**Commit:** `docs(01-02-closure): mark ROADMAP Plans 2–3 complete, update STATE.md`
</task>

## Verification Criteria

- [ ] docs/COMPONENT_TYPES.md created with all 12 types documented + examples
- [ ] docs/CLI_REFERENCE.md created with command syntax, flags, and JSON output examples
- [ ] docs/ADR_COMPONENT_TYPES.md created explaining design decisions
- [ ] docs/CONFIGURATION.md created with seed config guide and examples
- [ ] README.md updated with component classification section
- [ ] QA test suite expanded with 5+ edge case tests, all passing
- [ ] All user docs follow CLAUDE.md standards (no phase/process language)
- [ ] ROADMAP.md Plans 2 and 3 marked complete
- [ ] All commits are documented (7 commits total)

## Must-Haves for Goal Achievement

- [x] User-facing component type taxonomy and guide created
- [x] CLI reference for list --type command documented
- [x] Seed config extensibility mechanism documented with examples
- [x] QA test corpus expanded with edge cases and accuracy measurement
- [x] All user docs appear permanent and standalone (no internal process language)

**Result:** Closes GAP-4. Phase 1 is **fully complete**: goal achieved, code implemented, users documented, QA validated.

---

*Closure plan: 01-component-model*
*Created: 2026-03-19*
