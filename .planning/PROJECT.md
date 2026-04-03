# graphmd: Dependency Graph for Infrastructure Troubleshooting

## What This Is

graphmd analyzes markdown documentation and codebases to build a queryable dependency graph of infrastructure components (services, databases, caches, message queues, etc.). Engineers document their architecture in markdown; graphmd extracts entities and infers relationships. The graph can be exported as a self-contained SQLite database that AI agents can query in production to troubleshoot incidents without context window bloat.

## Core Value

**AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph instead of being fed entire architecture via prompts.**

This enables real-time incident response: when infrastructure provisioning fails, an agent can trace impact through the graph (UI layer → service → AWS → database) instead of hallucinating or requiring context-heavy prompts.

## Current State (Brownfield)

Existing codebase handles:
- Markdown scanning and entity extraction
- Relationship discovery and inference
- Graph construction and traversal
- Full-text and semantic search
- Graph export

Execution modes to build:
- `crawl` — Local graph exploration
- `export` — Build graph + create SQLite package
- `import` — Load package in production, expose query interface

## Requirements

### Validated

- ✓ Parse markdown files recursively — existing
- ✓ Extract component names/entities from text — existing
- ✓ Build graph structure (nodes/edges) — existing
- ✓ Traverse relationships — existing (crawl algorithms)
- ✓ Export graph data — existing (export command)

### Active

- [ ] Refine component model to track types (service, database, cache, queue, etc.)
- [ ] Implement `crawl` command for local graph exploration
- [ ] Implement `export` command to build graph + create SQLite package + ZIP
- [ ] Implement `import` command to load ZIP + initialize SQLite in production
- [ ] Build CLI query interface for graph traversal (impact analysis, path finding)
- [ ] Build MCP server wrapper around query interface (for LLM agents)
- [ ] Handle circular dependencies gracefully (no infinite loops in traversal)
- [ ] Infer relationships from markdown references (service names, not just explicit links)
- [ ] Track component type (service vs database vs cache, etc.)

### Out of Scope (v1)

- Code analysis integration (Axon-style) — defer to v2 (add layers of inference)
- Feature flag extraction — defer to v2
- Environment config extraction — defer to v2
- Incremental indexing (full re-index on export okay for v1)
- Human-facing visualization (CLI output is plain text, optimized for agents)
- Concurrent writes to SQLite (production is read-only after import)

## Context

**Use case:** On-demand infrastructure failures. When RDS provisioning or instance launching fails, oncall engineers or automated AI agents need to understand:
- Which services depend on this resource?
- What user journeys are affected?
- What downstream systems could detect this failure?

**Current pain point:** Explaining architecture to LLMs via prompts fills context windows. A queryable graph is more efficient and reliable.

**Deployment model:**
- Dev/Local: Engineers run `crawl` to explore
- CI/Export: Pipeline runs `export` to build and package graph
- Production: Container imports ZIP once, AI agents query via CLI/MCP

**Component examples:**
- Services: payment-api, user-service, infrastructure-provisioner
- Databases: primary-db, analytics-db, cache-redis
- Infrastructure: aws-rds, kubernetes-cluster, message-queue
- Anything named and referenced in markdown is a component

## Constraints

- **Performance:** SQLite single-writer, but production is read-only after import (no constraint)
- **Scope:** v1 is markdown-only; code analysis deferred
- **Graph size:** In-memory during build is okay (pre-computation happens locally); production container only holds SQLite (small disk footprint)
- **Freshness:** Export/import cycle is manual; no real-time updates in v1 (acceptable for incident response)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Everything is a "Component" with types | Simpler model, flexible for AI agents to understand context | — Pending |
| Markdown-only for v1 | Existing codebase is strong here; code analysis is harder problem | — Pending |
| Export creates ZIP package | Portable, self-contained, easy to import into containers | — Pending |
| AI agents as primary users (not humans) | Shaped by incident response use case; CLI/MCP optimized for agent queries | — Pending |

---

*Last updated: 2026-03-16 after project initialization*
