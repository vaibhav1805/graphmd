# CLI Reference

Complete reference documentation for graphmd command-line interface, with emphasis on component type queries and machine-readable output for AI agents.

## graphmd list — Query Components by Type

List components from your infrastructure documentation, optionally filtered by type, tags, or output format.

### Syntax

```bash
graphmd list [FLAGS] [--dir PATH]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type TYPE` | string | (all) | Filter by component type (e.g., `service`, `database`). Returns only components with matching primary type. |
| `--include-tags` | boolean | false | Include components matching type as a tag (not just primary type). Useful for finding components tagged with a type they don't primarily identify as. |
| `--output json\|table` | string | `table` | Output format. `json` for machine parsing, `table` for human-readable display. |
| `--dir PATH` | string | `./docs` | Input markdown directory to scan for components. Must be an absolute or relative path to a valid directory. |

### Examples

#### List all components (table output)

```bash
graphmd list --dir ./docs
```

**Output:**
```
Name                 Type         Confidence  Tags
─────────────────────────────────────────────────────────────
api-gateway          gateway      0.95        [critical, internal]
postgres-primary     database     0.98        [critical, ha]
redis-cache          cache        0.92        [high-performance]
order-service        service      0.88        []
...
```

#### List only services

```bash
graphmd list --type service --dir ./docs
```

**Output (table):**
```
Name             Type     Confidence  Tags
──────────────────────────────────────────
order-service    service  0.88        []
user-service     service  0.91        [internal]
payment-service  service  0.85        []
```

#### Query services as JSON (for AI agents)

```bash
graphmd list --type service --output json --dir ./docs
```

**Output:**
```json
{
  "components": [
    {
      "name": "api-gateway",
      "type": "gateway",
      "match_type": "primary_type",
      "confidence": 0.95,
      "tags": ["critical", "internal"]
    },
    {
      "name": "order-service",
      "type": "service",
      "match_type": "primary_type",
      "confidence": 0.88,
      "tags": []
    },
    {
      "name": "payment-service",
      "type": "service",
      "match_type": "primary_type",
      "confidence": 0.85,
      "tags": []
    }
  ],
  "filter": {
    "query": "--type service",
    "mode": "strict",
    "primary_matches": 3,
    "tag_matches": 0,
    "note": "Showing primary type matches only. Use --include-tags to include tag matches."
  }
}
```

#### Find all databases and components tagged as database

```bash
graphmd list --type database --include-tags --output json --dir ./docs
```

**Output:**
```json
{
  "components": [
    {
      "name": "postgres-primary",
      "type": "database",
      "match_type": "primary_type",
      "confidence": 0.98,
      "tags": ["critical", "ha"]
    },
    {
      "name": "postgres-replica",
      "type": "database",
      "match_type": "primary_type",
      "confidence": 0.96,
      "tags": ["ha"]
    },
    {
      "name": "backup-service",
      "type": "service",
      "match_type": "tag",
      "confidence": 0.72,
      "tags": ["database-backup", "critical"]
    }
  ],
  "filter": {
    "query": "--type database --include-tags",
    "mode": "loose",
    "primary_matches": 2,
    "tag_matches": 1,
    "note": "Showing primary type and tag matches. 1 additional component matches as a tag only."
  }
}
```

#### Find critical components

**Note:** Currently, filtering by tag requires post-processing the JSON output with tools like `jq`:

```bash
graphmd list --output json --dir ./docs | jq '.components[] | select(.tags[]? | contains("critical"))'
```

**Output:**
```json
{
  "name": "postgres-primary",
  "type": "database",
  "match_type": "primary_type",
  "confidence": 0.98,
  "tags": ["critical", "ha"]
}
{
  "name": "api-gateway",
  "type": "gateway",
  "match_type": "primary_type",
  "confidence": 0.95,
  "tags": ["critical", "internal"]
}
```

### JSON Output Format

All JSON responses follow this structure:

```json
{
  "components": [
    {
      "name": "string",
      "type": "string",
      "match_type": "primary_type" | "tag",
      "confidence": 0.0–1.0,
      "tags": ["string", ...]
    }
  ],
  "filter": {
    "query": "string (flags used)",
    "mode": "strict" | "loose" (depends on --include-tags),
    "primary_matches": number,
    "tag_matches": number,
    "note": "string (explanation)"
  }
}
```

**Field descriptions:**
- `name`: Component identifier (e.g., service name, database hostname)
- `type`: Primary type classification (service, database, cache, etc.)
- `match_type`: How the component matched the filter (`primary_type` or `tag`)
- `confidence`: Detection confidence (0.4–1.0). Higher = more reliable.
- `tags`: Array of optional secondary classifications
- `filter.mode`: `strict` (primary type only) or `loose` (includes tag matches)

---

## graphmd index — Build Component Graph

Index markdown documents and detect component types, persisting results to SQLite.

### Syntax

```bash
graphmd index [FLAGS] [--dir PATH] [--seed-config FILE]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir PATH` | string | `./docs` | Input markdown directory to scan for components. |
| `--seed-config FILE` | string | (none) | Optional YAML file for seed configuration (custom types, overrides). See docs/CONFIGURATION.md. |

### Examples

#### Standard indexing

```bash
graphmd index --dir ./docs
```

**Output:**
```
Scanning ./docs
Found 62 markdown files
Detecting components...
Detected 42 components across 12 types
Persisting to .bmd/knowledge.db
Done.
```

#### Indexing with seed config

```bash
graphmd index --dir ./docs --seed-config ./custom_types.yaml
```

**Output:**
```
Scanning ./docs
Found 62 markdown files
Loading seed config from ./custom_types.yaml (5 custom mappings)
Detecting components...
Detected 42 components across 12 types (5 overridden by seed config)
Persisting to .bmd/knowledge.db
Done.
```

### Database Schema

The `index` command creates `.bmd/knowledge.db` with the following relevant schema:

```sql
-- Graph nodes with component type
CREATE TABLE graph_nodes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  component_type TEXT NOT NULL DEFAULT 'unknown',
  confidence REAL NOT NULL DEFAULT 0.5,
  tags TEXT,  -- JSON array: ["tag1", "tag2"]
  detection_method TEXT
);

-- Component mentions (provenance)
CREATE TABLE component_mentions (
  id TEXT PRIMARY KEY,
  component_id TEXT NOT NULL,
  file_path TEXT NOT NULL,
  detection_method TEXT NOT NULL,
  confidence REAL NOT NULL,
  FOREIGN KEY (component_id) REFERENCES graph_nodes(id)
);
```

---

## graphmd export — Export Dependency Graph

Export the indexed graph to SQLite or other formats (preview for Phase 3).

### Syntax

```bash
graphmd export [FLAGS] [--format FORMAT] [--output FILE]
```

### Flags (Preview)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format sqlite\|json` | string | `sqlite` | Output format (not yet implemented). |
| `--output FILE` | string | `./graph.db` | Output file path. |

### Note

The `export` command is under development. The indexed SQLite database (`.bmd/knowledge.db`) already contains the full component graph with types. Future versions will support additional formats.

---

## Querying the Database Directly

Advanced users can query the underlying SQLite database directly:

### Count components by type

```bash
sqlite3 .bmd/knowledge.db "SELECT component_type, COUNT(*) FROM graph_nodes GROUP BY component_type ORDER BY COUNT(*) DESC;"
```

### Find all critical components

```bash
sqlite3 .bmd/knowledge.db "SELECT name, component_type, confidence FROM graph_nodes WHERE tags LIKE '%critical%' ORDER BY confidence DESC;"
```

### Export all databases

```bash
sqlite3 .bmd/knowledge.db "SELECT name, confidence, tags FROM graph_nodes WHERE component_type = 'database' ORDER BY confidence DESC;"
```

---

## Common Workflows

### I want to understand my service dependencies

```bash
# List all services
graphmd list --type service --output json --dir ./docs | jq '.components[].name'

# Then examine what each service depends on by looking at relationship data
sqlite3 .bmd/knowledge.db "SELECT * FROM graph_edges WHERE source IN (SELECT id FROM graph_nodes WHERE component_type = 'service');"
```

### I want to find all high-confidence components

```bash
graphmd list --output json --dir ./docs | jq '.components[] | select(.confidence >= 0.9)'
```

### I want to validate that my seed config is being applied

```bash
graphmd index --dir ./docs --seed-config ./custom.yaml

# Check results
graphmd list --output json --dir ./docs | jq '.components[] | select(.name == "my-component")'
```

### I want to export components for analysis in another tool

```bash
graphmd list --output json --dir ./docs > components.json

# Agents can then parse and analyze components.json
```

---

**Last updated:** 2026-03-19
