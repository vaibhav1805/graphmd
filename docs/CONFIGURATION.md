# Configuration Guide

This guide covers all configuration mechanisms for graphmd: `.graphmdignore` for excluding files, `graphmd-aliases.yaml` for component name resolution, seed configuration for type customization, and named graph management.

---

## .graphmdignore

The `.graphmdignore` file controls which files and directories are excluded from scanning during `export` and `crawl`. Place it in the root of the directory being scanned.

### File Format

- One pattern per line
- Lines starting with `#` are comments
- Blank lines are skipped
- Patterns ending with `/` match directories only (trailing slash is stripped during matching)
- All other patterns match files
- Supports glob wildcards: `*`, `?`, `[abc]`

### Example

```gitignore
# .graphmdignore

# Directories
vendor/
node_modules/
.git/
dist/
build/
.bmd/

# Files
*.lock
temp_*
```

### Default Patterns

When no `.graphmdignore` file exists, graphmd applies these default directory exclusions:

```
vendor
node_modules
.git
__pycache__
.venv
dist
build
target
.gradle
.next
out
.cache
bin
obj
.bmd
.planning
```

When a `.graphmdignore` file is present, only its patterns are used (defaults are not merged).

---

## graphmd-aliases.yaml

The alias configuration file maps multiple names for the same component to a single canonical identity. This prevents duplicate nodes when documentation and code refer to the same component by different names.

Place `graphmd-aliases.yaml` in the root of the directory being scanned.

### File Format

```yaml
# graphmd-aliases.yaml
aliases:
  canonical-name:
    - alias-1
    - alias-2
    - alias-3
```

Each key is the canonical component name. Its values are alternate names that should resolve to the canonical name during graph building.

### Example

```yaml
# graphmd-aliases.yaml
aliases:
  postgres-primary:
    - pg-main
    - primary-db
    - pgdb
    - main-database

  redis-cache:
    - redis
    - cache-layer
    - session-store

  api-gateway:
    - gateway
    - api-gw
    - ingress
```

### How Aliases Are Applied

During `export` and `crawl`, graphmd loads the alias file and resolves node IDs and edge endpoints:

1. If a node ID matches an alias, the node is renamed to the canonical name
2. If an edge source or target matches an alias, the endpoint is updated to the canonical name
3. Matching is case-sensitive
4. If the canonical name already exists as a node, the alias node is not merged (to avoid data loss)

The number of aliases applied is reported in the export summary and stored in `metadata.json`.

### Use Cases

- **Documentation vs. code names:** Your docs call it "primary-db" but your code connects to "postgres-primary"
- **Abbreviations:** Teams use shorthand ("pg", "redis") while documentation uses full names
- **Legacy names:** Old documentation references outdated component names

---

## Seed Configuration

Seed config overrides auto-detected component types without modifying code. It allows you to define custom types, fix misclassifications, and apply tags.

### Key Principles

- **Always wins:** Seed config mappings have precedence over auto-detection (confidence 1.0)
- **Glob patterns:** Match components by name patterns, not exact matches
- **No code changes:** Customize your taxonomy entirely through YAML files

### File Format

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "COMPONENT_PATTERN"
    type: "TYPE"
    tags:
      - "tag1"
      - "tag2"
    confidence_override: 1.0  # optional; always 1.0 if omitted
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pattern` | string | Yes | Glob pattern matching component names. Examples: `postgres*`, `services/api-*`, `*-db` |
| `type` | string | Yes | Component type to assign. May be a core type or custom type. |
| `tags` | array | No | Tags to apply to matched components. |
| `confidence_override` | float | No | Confidence score (default: 1.0). |

### Pattern Matching

Patterns use standard glob syntax:

- `*` matches any sequence of characters (except `/`)
- `?` matches a single character
- `[abc]` matches one character in the set
- `/` is the path separator for folder-based matching

```yaml
# Match by prefix
- pattern: "redis*"
  type: "cache"

# Match by suffix
- pattern: "*-db"
  type: "database"

# Match by folder
- pattern: "services/*"
  type: "service"

# Exact match
- pattern: "prometheus"
  type: "monitoring"
```

### Loading Seed Config

Pass the seed config file when indexing:

```bash
graphmd index --dir ./docs --seed-config ./custom_types.yaml
```

### Complete Example

```yaml
# organization_types.yaml
seed_mappings:
  # Override misclassification
  - pattern: "helper-service"
    type: "cache"
    tags: ["internal", "utility"]

  # Custom type for ML infrastructure
  - pattern: "ml-platform/*"
    type: "ml-service"
    tags: ["ml-team", "experimental"]

  # Tag production databases as critical
  - pattern: "databases/prod-*"
    type: "database"
    tags: ["production", "critical"]

  # Legacy components
  - pattern: "svc-*"
    type: "service"
    tags: ["legacy"]
```

### Verifying Seed Config

After indexing with seed config, components with `confidence = 1.0` were matched by seed config:

```bash
sqlite3 .bmd/knowledge.db "
  SELECT name, component_type, confidence
  FROM graph_nodes
  WHERE confidence = 1.0
  ORDER BY name;
"
```

---

## Named Graph Management

graphmd supports multiple named graphs in persistent storage. This allows you to import graphs from different environments or versions and query them independently.

### Storage Location

Graphs are stored under XDG-compliant paths:

- Linux/macOS: `$XDG_DATA_HOME/graphmd/` (default: `~/.local/share/graphmd/`)
- Each named graph gets its own subdirectory containing `graph.db` and `metadata.json`

### Importing Named Graphs

```bash
# Name derived from filename
graphmd import prod-graph.zip
# Stored as: ~/.local/share/graphmd/prod-graph/

# Explicit name
graphmd import graph.zip --name prod-infra
# Stored as: ~/.local/share/graphmd/prod-infra/

# Import a second graph
graphmd import graph.zip --name staging-infra
```

The most recently imported graph becomes the default for queries.

### Querying Named Graphs

Use `--graph` on any query command to select a specific graph:

```bash
# Query the default (most recent) graph
graphmd query impact --component primary-db

# Query a specific named graph
graphmd query impact --component primary-db --graph prod-infra
graphmd query list --type service --graph staging-infra
```

The `--graph` flag works on all query subcommands: `impact`, `dependencies`, `path`, and `list`.

### Replacing a Graph

Importing with the same name as an existing graph replaces it:

```bash
graphmd import updated-graph.zip --name prod-infra
# "Replacing existing graph "prod-infra""
```

---

## Best Practices

### 1. Start with Auto-Detection

Run graphmd without seed config first to see what auto-detection achieves:

```bash
graphmd crawl --input ./docs --format json
```

Then create seed config for the gaps.

### 2. Use Aliases for Name Consistency

If your docs and code use different names for the same component, define aliases rather than renaming things in your documentation:

```yaml
aliases:
  postgres-primary:
    - pg-main
    - primary-db
```

### 3. Keep .graphmdignore Updated

Exclude directories that generate noise (build output, dependencies, caches) to keep the graph focused on real infrastructure documentation.

### 4. Version Your Exports

Use `--git-version` or `--version` to tag exports with meaningful versions:

```bash
graphmd export --input ./docs --output graph.zip --git-version
```

### 5. Use Named Graphs for Environments

Import different environments as separate named graphs:

```bash
graphmd import prod-graph.zip --name prod
graphmd import staging-graph.zip --name staging
graphmd query list --graph prod --type database
```

---

## Troubleshooting

### Pattern Not Matching in .graphmdignore

- Patterns are case-sensitive
- Directory patterns must end with `/`
- File glob patterns (e.g., `*.lock`) do not cross directory boundaries

### Aliases Not Being Applied

- Check that `graphmd-aliases.yaml` is in the root of the scanned directory
- Alias matching is case-sensitive
- If the canonical name already exists as a node, the alias node is not renamed

### Seed Config Pattern Not Matching

- Patterns are case-sensitive
- `*` does not cross `/` boundaries — use folder prefixes for nested components
- Check the confidence column: `1.0` confirms seed config matched

### No Graph Found When Querying

If queries return `NO_GRAPH`, import a graph first:

```bash
graphmd export --input ./docs --output graph.zip
graphmd import graph.zip --name my-graph
graphmd query list
```

---

**Last updated:** 2026-04-03
