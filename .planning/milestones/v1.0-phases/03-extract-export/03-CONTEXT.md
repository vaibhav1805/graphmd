# Phase 3: Extract & Export Pipeline - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Complete the end-to-end pipeline from markdown scanning through SQLite packaging as a portable ZIP archive. Wire component extraction and relationship discovery algorithms into a unified export flow that produces a validated graph artifact for import and querying in Phase 4.

This phase focuses on **building and packaging** — not querying. Export artifact will contain a complete, normalized knowledge graph with component and relationship metadata, confidence scores, and provenance information.

</domain>

<decisions>
## Implementation Decisions

### Crawl Scope & File Discovery

- **File exclusions:** Auto-generated `.graphmdignore` file with common patterns (vendor, node_modules, .git, etc.)
  - Operators can customize exclusions; crawl respects the ignore file
- **File types:** `.md` files only (no `.markdown` or other extensions)
- **Hidden files:** Always excluded (files starting with `.`)
- **Component provenance:** Include source file path in database schema so agents can trace components back to source markdown

### Component Extraction & Normalization

- **Deduplication:** When multiple extraction methods find the same component, produce a single entry with aggregated confidence score
- **Component name aliasing:** Support user-defined aliases in config (e.g., `payment-api` ↔ `PaymentAPI` ↔ `Payment-Service` map to canonical name)
  - Aliases applied during export pipeline, before graph building
- **Type resolution:** When extraction methods suggest different types for the same component, use the highest confidence score to determine type
- **Confidence threshold:** Include all extracted components regardless of confidence level; agents filter based on confidence
- **Unknown types:** Components with undetermined types default to `unknown` type
- **Method attribution:** Include `extraction_method` metadata for each component (which detection algorithm found it)

### Relationship Discovery & Signal Aggregation

- **Algorithm set:** Run all available discovery algorithms by default
  - Algorithms: co-occurrence, structural, NER, semantic, LLM
  - Operators can disable expensive algorithms in config if needed
- **Algorithm disagreement:** Include all relationship findings (not just agreements)
  - Confidence score reflects aggregation: higher confidence = more algorithms found the relationship
- **LLM algorithm handling:**
  - No time/cost limits; LLM runs to completion
  - Log all API calls so operators can monitor and cancel if needed
- **Confidence in results:** Single aggregated confidence score per relationship (no per-algorithm attribution)
  - Agents assess trustworthiness via confidence tier

### Export Format & Packaging

- **ZIP contents:** `graph.db` (SQLite) + `metadata.json` only
  - Minimal, portable artifact
  - Source file references preserved in database via provenance fields
- **Metadata fields (metadata.json):**
  - Required: `version`, `created_at` (ISO 8601), `component_count`, `relationship_count`
  - Extended: `input_path`, `ignore_patterns`, `aliases_applied`, `algorithm_versions`, `schema_version`
- **SQLite indexing:** Create indexes on:
  - Component name (fast lookups)
  - Component type (fast filtering by service/database/etc.)
  - Relationship source/target (fast traversal for impact/dependency queries)
  - Relationship confidence (fast filtering by reliability)
- **Schema versioning:**
  - Include schema version in metadata.json
  - Phase 4 import validates schema compatibility before loading

### Claude's Discretion

- Exact index column combinations and cardinality tuning
- `.graphmdignore` default patterns (implement sensible defaults)
- Alias config file format and parsing
- Metadata.json formatting and nesting structure
- Error handling and recovery during export (partial failures, timeouts)

</decisions>

<specifics>
## Specific Ideas

- The export should feel like a deployable artifact: operators run `graphmd export` once, ship the ZIP to production, and agents query it reliably
- Component aliasing should be intuitive for teams that have naming inconsistencies (common in infra docs)
- Metadata should answer the question "how was this graph built?" — helps operators understand confidence in the exported data

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-extract-export*
*Context gathered: 2026-03-19*
