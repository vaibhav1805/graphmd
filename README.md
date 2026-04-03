# graphmd

A dependency graph analyzer for infrastructure documentation. Query component relationships and answer critical questions like "if this fails, what breaks?" without feeding entire architecture to AI agents.

## What Is graphmd?

graphmd scans your infrastructure documentation (markdown files), automatically detects components, classifies them by type, and builds a queryable dependency graph. This enables AI agents and operators to:

- **Query by component type:** "Find all databases" or "List all critical services"
- **Analyze dependencies:** "What services depend on postgres-primary?"
- **Assess impact:** "If this cache fails, which services are affected?"
- **Audit infrastructure:** "Count components by type, show confidence scores"

Instead of feeding AI agents your entire architecture, they query the pre-computed graph — faster, cheaper, and more reliable.

## Features

### Component Classification

Every component carries a type classification, enabling targeted queries:

```bash
# List all services
graphmd list --type service

# List all databases
graphmd list --type database --output json

# Find all critical components
graphmd list --output json | jq '.components[] | select(.tags[]? | contains("critical"))'
```

Components are classified into 12 core types: `service`, `database`, `cache`, `queue`, `message-broker`, `load-balancer`, `gateway`, `storage`, `container-registry`, `config-server`, `monitoring`, `log-aggregator`.

See [docs/COMPONENT_TYPES.md](docs/COMPONENT_TYPES.md) for complete type reference.

### Confidence-Aware Detection

Each component carries a confidence score (0.4–1.0) reflecting detection reliability. Higher confidence indicates stronger evidence:

```json
{
  "name": "postgres-primary",
  "type": "database",
  "confidence": 0.98,
  "tags": ["critical", "ha"]
}
```

AI agents can filter by confidence threshold for risk-aware decisions.

### Extensible Type System

Define custom types without code changes via seed configuration:

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "internal-tools/*"
    type: "internal-tool"
    tags: ["internal-only"]
```

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for customization guide.

### Dependency Graph Queries

Export the indexed graph to SQLite for direct queries:

```bash
# Index your documentation
graphmd index --dir ./docs

# Query components by type
sqlite3 .bmd/knowledge.db "SELECT * FROM graph_nodes WHERE component_type = 'database';"

# Find relationships
sqlite3 .bmd/knowledge.db "SELECT * FROM graph_edges WHERE source_type = 'service' AND target_type = 'database';"
```

## Quick Start

### Prerequisites

- Go 1.21+
- Your infrastructure documented in markdown (in any directory)

### Installation

```bash
# Clone and build
git clone https://github.com/your-org/graphmd
cd graphmd
go build -o graphmd ./cmd/graphmd
```

### Index Your Documentation

```bash
# Scan documentation directory and detect components
./graphmd index --dir ./docs

# Or with seed config for custom types
./graphmd index --dir ./docs --seed-config ./custom_types.yaml
```

The index command creates `.bmd/knowledge.db` containing the indexed graph.

### Query Components by Type

```bash
# List all services (table output)
./graphmd list --type service --dir ./docs

# List services as JSON (for AI agents)
./graphmd list --type service --output json --dir ./docs

# Output:
# {
#   "components": [
#     {
#       "name": "api-gateway",
#       "type": "gateway",
#       "confidence": 0.95,
#       "tags": ["critical", "internal"]
#     },
#     {
#       "name": "order-service",
#       "type": "service",
#       "confidence": 0.88,
#       "tags": []
#     }
#   ],
#   "filter": {
#     "query": "--type service",
#     "primary_matches": 1,
#     "tag_matches": 0
#   }
# }
```

See [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) for complete command reference.

## Common Workflows

### Find All Databases and Their Dependents

```bash
# Step 1: Index documentation
graphmd index --dir ./docs

# Step 2: List all databases
sqlite3 .bmd/knowledge.db "
  SELECT name, confidence FROM graph_nodes
  WHERE component_type = 'database'
  ORDER BY confidence DESC;
"

# Step 3: Find services that depend on each database
sqlite3 .bmd/knowledge.db "
  SELECT source.name as 'service', target.name as 'database'
  FROM graph_edges
  JOIN graph_nodes source ON graph_edges.source = source.id
  JOIN graph_nodes target ON graph_edges.target = target.id
  WHERE target.component_type = 'database'
  AND source.component_type = 'service';
"
```

### Audit Critical Infrastructure

```bash
# Find all components tagged as critical
./graphmd list --output json --dir ./docs | jq '.components[] | select(.tags[]? | contains("critical"))'

# Count critical components by type
./graphmd list --output json --dir ./docs | \
  jq '.components[] | select(.tags[]? | contains("critical")) | .type' | \
  sort | uniq -c
```

### Validate Seed Config Application

```bash
# Index with seed config
graphmd index --dir ./docs --seed-config ./custom_types.yaml

# Check that seed config was applied (confidence 1.0 = seed-configured)
sqlite3 .bmd/knowledge.db "
  SELECT name, component_type, confidence
  FROM graph_nodes
  WHERE confidence = 1.0
  ORDER BY name;
"
```

## Documentation

- **[docs/COMPONENT_TYPES.md](docs/COMPONENT_TYPES.md)** — Complete reference for all 12 component types, detection patterns, and confidence scoring
- **[docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md)** — Command reference for `graphmd list`, `graphmd index`, `graphmd export`, with examples and JSON output formats
- **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)** — Guide to customizing types via seed configuration; examples and best practices
- **[docs/ADR_COMPONENT_TYPES.md](docs/ADR_COMPONENT_TYPES.md)** — Architecture decision record explaining design choices (why 12 types, confidence scores, extensibility)

## Project Structure

```
graphmd/
├── cmd/
│   └── graphmd/              # CLI entry point
├── internal/
│   └── knowledge/            # Core detection and graph pipeline
├── docs/                     # User-facing documentation
└── test-data/                # Test corpus for validation
```

## How It Works

### 1. Scanning

`graphmd index` scans your markdown documentation and extracts:
- File paths and headings (component names)
- Keywords and context (type detection signals)
- Relationships between components

### 2. Component Type Detection

For each detected component, graphmd applies multiple detection algorithms:
- **Naming patterns:** Does the name contain "postgres", "redis", "service"?
- **File paths:** Is it in `services/`, `databases/`, `monitoring/`?
- **Keywords:** Does the documentation mention type-specific keywords?

Result: `(name, type, confidence, detection_methods)`

### 3. Confidence Scoring

Each detection contributes evidence. Confidence ranges [0.4, 1.0]:
- **0.95–1.0:** Very high (explicit naming)
- **0.80–0.94:** High (clear but not explicit)
- **0.65–0.79:** Moderate (some ambiguity)
- **0.40–0.64:** Low (weak signal)

Components with confidence below 0.65 default to `unknown`.

### 4. Persistence

The indexed graph is persisted to SQLite (`.bmd/knowledge.db`) with tables:
- `graph_nodes`: Components with type, confidence, tags
- `graph_edges`: Relationships between components
- `component_mentions`: Provenance (where was each component detected?)

### 5. Querying

Query the indexed graph via:
- **CLI:** `graphmd list --type TYPE`
- **SQL:** Direct SQLite queries for complex analysis
- **JSON export:** Machine-readable output for AI agents

## Design Philosophy

**AI agents should query pre-computed graphs, not entire architectures.**

Why?

1. **Efficiency:** Agents get answers in milliseconds, not seconds
2. **Cost:** Fewer tokens in prompts means cheaper inference
3. **Reliability:** Pre-computed graphs are more reliable than heuristic parsing
4. **Compliance:** Structured queries avoid exposing sensitive data

graphmd enables this by:
- Automatically detecting components from your existing documentation
- Computing a queryable dependency graph upfront
- Exposing the graph via simple, machine-friendly APIs

## Testing

Test corpus in `test-data/` contains diverse component examples:

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestComponentDetection ./...
```

Coverage target: >85% for core packages.

## Configuration

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for full customization options.

### Example: Custom Types for Your Organization

```yaml
# custom_types.yaml
seed_mappings:
  # Define custom types
  - pattern: "ml-platform/*"
    type: "ml-service"
    tags: ["ai", "experimental"]

  # Override misclassifications
  - pattern: "helper-service"
    type: "cache"
    tags: ["utility"]

  # Add tags to auto-detected components
  - pattern: "prod-*"
    type: "service"
    tags: ["production", "critical"]
```

Load via: `graphmd index --dir ./docs --seed-config ./custom_types.yaml`

## Contributing

graphmd welcomes contributions. Please see `CLAUDE.md` for development guidance.

## License

[Your license here]

---

**Last Updated:** 2026-03-19
