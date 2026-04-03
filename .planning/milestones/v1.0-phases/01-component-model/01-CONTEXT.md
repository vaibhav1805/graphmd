# Phase 1: Component Model - Context

**Gathered:** 2026-03-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Refine the component model so every graph node carries a typed classification (service, database, cache, queue, etc.) that AI agents can filter and query. Wire component detection output to persistence layer, enabling type-based queries on exported graphs.

Scope: Component type definition, detection, persistence, and type-filtered queries. Relationship confidence and provenance are Phase 2 concerns. Component extraction via LLM is deferred to a future iteration.

</domain>

<decisions>
## Implementation Decisions

### Component Detection Pipeline (Tree-First Strategy)

**Overall approach:** PageIndex builds document tree (folder hierarchy, heading structure) → Detection algos run in that context → Fallback to LLM when ambiguous.

#### Stage 1: Document Tree Foundation
- Use PageIndex to build tree structure: root → folders → files → headings → content blocks
- Each tree node carries: location, heading context, parent/sibling/child relationships
- Tree context informs all subsequent detection steps
- Component locations tracked throughout detection for full provenance

#### Stage 2: Low-Cost Detection (Patterns + NER)
- Pattern matching on file/folder paths (base confidence: 0.7)
  - Example: "postgres*" filename → database, "services/" folder → service components
- Named Entity Recognition on component names (base confidence: 0.75)
  - Identifies tech names with understanding of tree context
  - Example: "Kubernetes" under "Infrastructure/Container" → higher confidence for container-registry
- Semantic/co-occurrence signals from nearby text (base confidence: 0.65)

#### Stage 3: Confidence Threshold Check
- If ensemble confidence > 0.8 → Use low-cost algos result, skip LLM
- If ensemble confidence ≤ 0.8 → Proceed to LLM fallback

#### Stage 4: LLM Fallback (High Confidence)
- Triggered only for ambiguous cases
- Prompt includes: tree context (folder path, headings), component name, nearby text, all algo votes
- LLM base confidence: 0.9
- Returns type + LLM's own confidence assessment
- Saves cost by avoiding LLM for obvious cases

#### Stage 5: Seed Config Overrides
- User-provided YAML config can define:
  - Pattern aliases: "any file matching 'redis*' → cache"
  - Custom types beyond the 12-type taxonomy
- Seed config **always wins** over algo results
- Enables user-driven customization and type extensions

#### Stage 6: Ensemble Resolution
- Weighted majority voting using base confidence of each algo
- Final type = argmax(algo_confidence × vote)
- Final confidence = aggregate of all contributing signals
- Full provenance maintained: which algos voted, their individual confidences

### Component Type System

#### Taxonomy
- Base 12 types from Backstage/Cartography patterns: service, database, cache, queue, message-broker, load-balancer, gateway, storage, container-registry, config-server, monitoring, log-aggregator
- User-extensible via seed config (additional custom types allowed)
- All components without clear classification can be assigned custom types by user

#### Type Flexibility
- Each component has: **primary_type** (required) + **tags** (optional array)
  - Example: Component "prometheus" → primary_type: "monitoring", tags: ["critical", "internal-only"]
- Primary type defines the core classification
- Tags capture secondary concerns (criticality, deployment model, compliance tags, etc.)
- Enables nuanced classification without multi-type complexity

### Persistence & Schema

#### Data Structure
- Store classification in JSON blob format for flexibility
- `classification` column contains: `{type, confidence, primary_type, tags, detection_methods, algo_signals, tree_location}`
- Separate `component_mentions` table tracks all detected locations (file + heading path for each mention)
- Enables "where was this component found?" queries and full provenance tracking

#### Backward Compatibility
- **Old exports (pre-Phase 1) are invalid** — require re-export with Phase 1 logic
- Forces all components through the detection pipeline
- No migration complexity, ensures data quality

#### Export Versioning
- Each export is versioned (tracked in metadata)
- Component types can change across versions
- Enables audit trail: "what was the type in v1.2?"
- Agents can query "as of which export?"

### Query Filtering & Response Format

#### Strictness Control (User/Agent Specified)
- `list --type database` → Primary type matches only (strict, default)
- `list --type database --include-tags` → Primary type + tag matches (inclusive)
- Command flag explicitly controls match behavior
- User/agent explicitly chooses confidence trade-off

#### Response Format (Clear to Agents)
Each result includes match metadata so agents understand confidence:
```json
{
  "components": [
    {
      "name": "postgres-primary",
      "type": "database",
      "match_type": "primary_type",
      "confidence": 0.95,
      "tags": ["critical", "high-availability"]
    },
    {
      "name": "app-service",
      "type": "service",
      "tags": ["database-adjacent"],
      "match_type": "tag_match",
      "confidence": 0.65
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
- `match_type` field indicates how each component matched (primary vs tag)
- `confidence` reflects detection ensemble confidence
- Summary in `filter` shows strict vs tag match counts
- Agents can assess result trustworthiness

### Claude's Discretion

- Specific heuristics for pattern matching (file extensions, naming conventions for each type)
- Exact confidence thresholds (currently 0.8 for LLM trigger threshold)
- Semantic signal weighting in co-occurrence algo
- Format of seed config YAML schema
- Tree location storage format (file path + heading hierarchy representation)

</decisions>

<specifics>
## Specific Ideas

- **Seed config inspiration:** Similar to Backstage catalog-info.yaml — user-friendly YAML for component definitions and type mappings
- **PageIndex integration:** Treat document tree as a first-class entity throughout detection, not an afterthought
- **Confidence interpretation:** Make confidence scores meaningful to AI agents — not just 0-1, but tied to which detection method succeeded
- **Tag examples:** Consider standard tags like ["critical", "deprecated", "internal-only", "compliance-critical"] to help agents reason about risk

</specifics>

<deferred>
## Deferred Ideas

- **LLM component extraction:** Using LLM to *discover* new components (not just classify existing ones) — deferred to next iteration after Phase 1 core is solid
- **Relationship confidence:** Phase 2 adds confidence tiers for relationships themselves (distinct from component type confidence)
- **Type hierarchies:** Could define service > web-service > api-service hierarchies, but deferred — start flat taxonomy
- **Multi-export diffing:** Tracking type changes across versions and highlighting what changed — valuable but not Phase 1 scope
- **Interactive detection refinement:** CLI mode where user confirms/corrects detected types during crawl — nice-to-have for future

</deferred>

---

*Phase: 01-component-model*
*Context gathered: 2026-03-16*
