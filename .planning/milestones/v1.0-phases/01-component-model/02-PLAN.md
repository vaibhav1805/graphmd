---
wave: 2
depends_on: [01]
files_modified:
  - docs/COMPONENT_TYPES.md
  - docs/CLI_REFERENCE.md
  - docs/ADR_COMPONENT_TYPES.md
  - docs/SEED_CONFIG_GUIDE.md
  - README.md
autonomous: false
---

# Plan 2: User-Facing Documentation & Guide

**Objective:** Create comprehensive, agent-friendly documentation explaining the component model, types, and queries. NO mention of "phases" in any user-facing docs.

**Requirements covered:** User-facing deliverables for COMP-01, COMP-02, COMP-03

**Dependency:** Plan 01 (component type system must exist)

## Task 2.1: Create Component Types Reference Documentation

Comprehensive guide to the 12-type taxonomy with examples, use cases, and decision trees.

```xml
<task>
  <description>Create docs/COMPONENT_TYPES.md explaining the 12-type taxonomy. For each type, include: definition, examples (service: payment-api, user-service; database: primary-db, analytics-db), how detection identifies each type, confidence interpretation. Include decision tree for ambiguous cases (e.g., "is this a cache or storage?"). Write for AI agents, not humans — focus on precise classification rules.</description>
  <acceptance_criteria>
    - [ ] All 12 types documented with clear definitions
    - [ ] At least 2 realistic examples per type
    - [ ] Detection heuristics explained (patterns, NER, semantic signals)
    - [ ] Confidence tier interpretation (0.9+ = LLM-confirmed, 0.8-0.89 = ensemble high confidence, etc.)
    - [ ] Decision tree for ambiguous classifications
    - [ ] No mention of project "phases" — write as if documentation is permanent, not phase-specific
  </acceptance_criteria>
  <verification>Docs are clear, examples are realistic, confidence tiers match code implementation, no "phase" references</verification>
</task>
```

## Task 2.2: Write CLI Reference for list --type Command

Quick-start guide and examples for using component type queries.

```xml
<task>
  <description>Create docs/CLI_REFERENCE.md with a dedicated section for the list --type command. Include: syntax, flags (--include-tags), output format, example queries, and interpretation guide. Examples: "list --type service" (get all services), "list --type database" (databases only), "list --type cache --include-tags" (caches + tag-matched components). Show JSON output structure with field explanations.</description>
  <acceptance_criteria>
    - [ ] Syntax clearly documented: graphmd list --type TYPE [--include-tags]
    - [ ] At least 3 real-world example queries with expected output
    - [ ] JSON output structure fully explained (fields, types, ranges)
    - [ ] Confidence score interpretation for query consumers (agents)
    - [ ] Error cases documented (unknown type, empty results, etc.)
    - [ ] No "phase" terminology — written as permanent CLI reference
  </acceptance_criteria>
  <verification>Examples are executable; output format matches code implementation; confidence ranges match code</verification>
</task>
```

## Task 2.3: Create Architecture Decision Record (ADR)

Document why component types were added, design decisions, and how they enable AI agent reasoning.

```xml
<task>
  <description>Create docs/ADR_COMPONENT_TYPES.md explaining: problem statement (AI agents need typed components to reason about infrastructure impact), solution (12-type taxonomy), key decisions (why these 12, why ensemble detection, why confidence scores), trade-offs (cost of multi-algo classification vs accuracy gain). Write for both humans (operators) and agents (query consumers).</description>
  <acceptance_criteria>
    - [ ] Problem statement: why types matter for incident response
    - [ ] Solution overview: 12-type taxonomy with examples
    - [ ] Key decisions documented with rationale
    - [ ] Trade-offs discussed (accuracy vs performance, customization complexity vs simplicity)
    - [ ] Extensibility explained (how users can add custom types via seed config)
    - [ ] ADR format: Status (Accepted/Proposed/etc.), Context, Decision, Consequences
  </acceptance_criteria>
  <verification>ADR is complete, rationale is clear, decision is defensible, trade-offs are explicit</verification>
</task>
```

## Task 2.4: Update README with Component Model Overview

High-level explanation of component types for users landing on the project.

```xml
<task>
  <description>Update README.md with a new "Component Types" section. Explain: what component types are, why they matter (enable agents to answer "if this fails, what breaks?"), the 12-type taxonomy, and how to query by type. Link to COMPONENT_TYPES.md for detailed reference. Keep this section brief (2-3 paragraphs) — detailed docs are elsewhere.</description>
  <acceptance_criteria>
    - [ ] README section explains component types in <100 words
    - [ ] Value proposition clear: types enable AI agent reasoning
    - [ ] 12-type taxonomy listed with brief examples
    - [ ] Link to full docs (COMPONENT_TYPES.md) provided
    - [ ] Example query shown: graphmd list --type service
    - [ ] No "phase" language — this is permanent project documentation
  </acceptance_criteria>
  <verification>README section is concise, links work, example is realistic, tone matches rest of README</verification>
</task>
```

## Task 2.5: Create Configuration Guide for Custom Types

Document how users can extend the taxonomy with custom component types.

```xml
<task>
  <description>Create docs/SEED_CONFIG_GUIDE.md explaining how to create a seed config YAML file for custom component type mappings. Show example: patterns like "redis*" → cache, "postgres*" → database, custom types like "vendor-service" or "deprecated-api". Explain precedence (seed config > detection algos) and use cases (company-specific type extensions, type corrections for ambiguous components).</description>
  <acceptance_criteria>
    - [ ] YAML schema documented with required/optional fields
    - [ ] At least 3 realistic examples of custom mappings
    - [ ] Precedence rules explained (seed config wins)
    - [ ] File location and loading mechanism explained
    - [ ] Examples show both pattern-based and name-based mappings
    - [ ] Use case for custom types explained (company-specific taxonomy, type corrections)
  </acceptance_criteria>
  <verification>Example YAML files are valid; schema matches code; precedence is correct</verification>
</task>
```

---

## Success Criteria (Documentation Completeness)

**Deliverables:**
- 5 comprehensive documentation files
- All written for permanent reference, not phase-specific
- Zero "phase" or "iteration" language
- Examples match code implementation
- Links between docs are consistent

---

## Delivery Artifacts

- `docs/COMPONENT_TYPES.md` — Type taxonomy guide with examples
- `docs/CLI_REFERENCE.md` — Command syntax and usage
- `docs/ADR_COMPONENT_TYPES.md` — Architecture decisions
- `docs/SEED_CONFIG_GUIDE.md` — Custom type configuration
- Updated `README.md` — High-level overview

---

*Plan created: 2026-03-16*
*Requirement mapping: User-facing documentation for COMP-01, COMP-02, COMP-03*
