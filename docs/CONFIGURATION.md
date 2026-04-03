# Configuration Guide: Customizing Component Types

This guide explains how to extend and customize component type classification for your infrastructure using seed configuration.

---

## Seed Config Overview

**Seed Config** is a mechanism for overriding auto-detected component types without modifying code. It allows you to:

- **Define custom types** for components that don't fit the 12 core categories
- **Override misclassifications** when auto-detection gets it wrong
- **Add confidence overrides** to force explicit classification
- **Apply tags** to auto-detected components

### Key Principles

- **Always wins:** Seed config mappings have precedence over auto-detection (confidence 1.0)
- **User intent:** Seed-configured types are treated as explicit user decisions
- **No code changes:** Customize your taxonomy entirely through YAML files
- **Glob patterns:** Match components by name patterns, not exact matches

---

## Seed Config File Format

Seed config files use YAML format for human readability.

### Basic Structure

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
| `type` | string | Yes | Component type to assign. May be a core type (`service`, `database`, etc.) or custom type. |
| `tags` | array | No | Optional tags to apply to matched components. Examples: `[critical, internal]` |
| `confidence_override` | float | No | Confidence score to assign (default: 1.0). Seed-configured types always override auto-detection. |

---

## Pattern Matching

Glob patterns determine which components are matched by a seed config entry.

### Syntax

Patterns use standard glob syntax:
- `*` matches any sequence of characters (except `/`)
- `?` matches a single character
- `[abc]` matches one character in the set
- `/` is the path separator for folder-based matching

### Examples

**Match by name prefix:**
```yaml
- pattern: "redis*"          # Matches: redis-primary, redis-cache, redis-secondary
  type: "cache"
```

**Match by suffix:**
```yaml
- pattern: "*-db"            # Matches: postgres-db, mysql-db, mongodb-db
  type: "database"
```

**Match by folder:**
```yaml
- pattern: "services/*"      # Matches: services/api, services/auth, services/payment
  type: "service"
```

**Match by exact name:**
```yaml
- pattern: "prometheus"      # Matches: prometheus (exactly)
  type: "monitoring"
```

**Match with wildcards:**
```yaml
- pattern: "k8s-*/cluster-*" # Matches: k8s-prod/cluster-main, k8s-staging/cluster-backup
  type: "container-registry"
```

---

## Complete Configuration Examples

### Example 1: Override Persistent Misclassifications

Problem: Your company has a component named "helper-service" that's always detected as `service`, but it's actually a utility library (cache layer).

**Solution:**

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "helper-service"
    type: "cache"
    tags: ["internal", "utility"]
```

Result: `helper-service` is now classified as `cache` (confidence 1.0), overriding auto-detection.

---

### Example 2: Custom Type for Internal Tools

Problem: Your infrastructure has internal tools (monitoring dashboards, config editors) that don't fit existing types.

**Solution:**

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "internal-tools/*"
    type: "internal-tool"      # Custom type
    tags: ["internal-only"]
    confidence_override: 1.0
```

Result: All components under `internal-tools/` are classified as `internal-tool` type.

---

### Example 3: Tag Auto-Detected Components

Problem: You want to mark all components matching a pattern as "critical" without changing their type.

**Solution:**

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "postgres*"
    type: "database"           # Keep auto-detected type
    tags: ["critical", "ha"]   # Add tags
    confidence_override: 0.99   # Override confidence slightly to prefer seed config
```

Result: All PostgreSQL components are tagged as `critical` and `ha`, while keeping the `database` type.

---

### Example 4: Handling Legacy Components

Problem: You have legacy infrastructure that doesn't follow modern naming conventions. You need explicit mappings.

**Solution:**

```yaml
# custom_types.yaml
seed_mappings:
  # Legacy naming scheme: "svc-X" for services
  - pattern: "svc-*"
    type: "service"
    tags: ["legacy"]

  # Legacy naming: "db-X" for databases
  - pattern: "db-*"
    type: "database"
    tags: ["legacy"]

  # One-off mappings for weird names
  - pattern: "the-old-beast"
    type: "service"
    tags: ["legacy", "high-priority-migration"]

  - pattern: "infra-cache-layer-v1"
    type: "cache"
    tags: ["deprecated"]
```

Result: Legacy components are properly classified and tagged for tracking.

---

### Example 5: Comprehensive Organization Configuration

A real-world example combining multiple patterns:

```yaml
# organization_types.yaml
seed_mappings:
  # Services (organized by team)
  - pattern: "platform/services/*"
    type: "service"
    tags: ["platform-team", "critical"]

  - pattern: "data-platform/services/*"
    type: "service"
    tags: ["data-team"]

  # Databases (explicitly managed)
  - pattern: "databases/prod-*"
    type: "database"
    tags: ["production", "critical"]

  - pattern: "databases/staging-*"
    type: "database"
    tags: ["staging"]

  - pattern: "databases/cache-*"
    type: "cache"
    tags: ["caching", "non-critical"]

  # Infrastructure components
  - pattern: "infra/kafka-*"
    type: "message-broker"
    tags: ["infrastructure"]

  - pattern: "infra/prometheus-*"
    type: "monitoring"
    tags: ["infrastructure", "observability"]

  - pattern: "infra/elk-*"
    type: "log-aggregator"
    tags: ["infrastructure", "observability"]

  # Custom types for organization-specific needs
  - pattern: "ml-platform/*"
    type: "ml-service"          # Custom type
    tags: ["ml-team", "experimental"]

  - pattern: "data-warehouse"
    type: "data-warehouse"      # Custom type
    tags: ["critical", "analytics"]
```

---

## Loading Seed Config

### Using CLI Flag

To use a seed config file when indexing:

```bash
graphmd index --dir ./docs --seed-config ./custom_types.yaml
```

The `index` command will:
1. Load the seed config file
2. Parse all seed_mappings
3. Apply them during component detection
4. Persist types to database (with confidence 1.0 for seed-matched components)

### Default Behavior

If no `--seed-config` flag is provided:

```bash
graphmd index --dir ./docs
```

Only auto-detection is used. Seed config is optional and entirely user-driven.

### File Location

You can store seed config anywhere:

```bash
# In project root
graphmd index --dir ./docs --seed-config ./seed_config.yaml

# In configuration directory
graphmd index --dir ./docs --seed-config ~/.graphmd/custom_types.yaml

# Relative or absolute paths both work
graphmd index --dir ./docs --seed-config /etc/graphmd/types.yaml
```

---

## Validation & Debugging

### Verify Seed Config Was Applied

After indexing with seed config, query the database to confirm:

```bash
# Check components matched by seed config
sqlite3 .bmd/knowledge.db "SELECT name, component_type, confidence FROM graph_nodes WHERE component_type = 'cache' ORDER BY confidence DESC;"
```

Components with `confidence = 1.0` were matched by seed config.

### Count Components by Source

```bash
# Show how many components came from seed config vs auto-detection
sqlite3 .bmd/knowledge.db "
SELECT
  CASE
    WHEN confidence = 1.0 THEN 'seed_config'
    ELSE 'auto_detection'
  END as source,
  COUNT(*) as count
FROM graph_nodes
GROUP BY source;
"
```

### List All Custom Types

```bash
# Find custom types (not in core 12)
sqlite3 .bmd/knowledge.db "
SELECT DISTINCT component_type
FROM graph_nodes
WHERE component_type NOT IN (
  'service', 'database', 'cache', 'queue', 'message-broker',
  'load-balancer', 'gateway', 'storage', 'container-registry',
  'config-server', 'monitoring', 'log-aggregator', 'unknown'
)
ORDER BY component_type;
"
```

---

## Best Practices

### 1. Start with Auto-Detection

Run graphmd without seed config first:

```bash
graphmd index --dir ./docs
graphmd list --output json | jq '.components | group_by(.type) | map({type: .[0].type, count: length})'
```

This shows you what auto-detection achieves. Then create seed config for the gaps.

### 2. Use Comments in Your YAML

```yaml
# custom_types.yaml

# ========== CORE SERVICES ==========
- pattern: "core/*"
  type: "service"
  tags: ["core", "critical"]

# ========== LEGACY COMPONENTS ==========
# These don't follow modern naming; map explicitly
- pattern: "old-*"
  type: "service"
  tags: ["legacy"]
```

### 3. Keep Patterns Specific

Good:
```yaml
- pattern: "services/api-*"      # Specific; matches services/api-users, services/api-orders
  type: "service"
```

Avoid:
```yaml
- pattern: "*"                   # Too broad; matches everything
  type: "service"
```

### 4. Use Tags Consistently

Define a standard set of tags for your organization:

```yaml
# Good: consistent tag naming
- pattern: "prod-*"
  tags: ["production", "critical"]

- pattern: "staging-*"
  tags: ["staging", "non-critical"]
```

Avoid:
```yaml
# Bad: inconsistent naming
- pattern: "prod-*"
  tags: ["prod", "important"]

- pattern: "staging-*"
  tags: ["stage", "not-important"]
```

### 5. Document Your Configuration

Create a README for your seed config:

```markdown
# Seed Config Documentation

This file customizes component types for ACME Corp's infrastructure.

## Custom Types

- `ml-service`: Machine learning pipeline components
- `data-warehouse`: Analytical data storage (distinct from operational databases)

## Tag Categories

- **Environment:** production, staging, development
- **Criticality:** critical, non-critical
- **Team:** platform-team, data-team, ml-team
- **Lifecycle:** legacy, experimental, deprecated
```

---

## Troubleshooting

### Pattern Not Matching Expected Components

Check:
1. **Case sensitivity:** Patterns are case-sensitive. `Service-*` won't match `service-foo`.
2. **Path vs. name:** If components are in folders, include folder prefix: `services/api-*` not `api-*`.
3. **Wildcard scope:** `*` doesn't cross folder boundaries. Use `**` for nested folders (if supported).

### Confidence Not 1.0

Seed config entries always get confidence 1.0. If you see lower confidence:
- The pattern didn't match (check spelling)
- Auto-detection found it instead (verify pattern in seed config)

### Too Many Unknown Types

Reasons:
1. Many components don't match any pattern (add more patterns)
2. Auto-detection has low confidence (lower than 0.65 threshold)

Solution: Add seed config for the gaps.

---

## Migration: From Auto-Detection to Seed Config

### Step 1: Export All Auto-Detected Components

```bash
graphmd index --dir ./docs
graphmd list --output json > auto_detected.json
```

### Step 2: Analyze Distribution

```bash
jq '.components | group_by(.type) | map({type: .[0].type, count: length, avg_confidence: (map(.confidence) | add / length)}) | sort_by(.count) | reverse' auto_detected.json
```

### Step 3: Identify Misclassifications

Manually review components with:
- Confidence < 0.8 (uncertain)
- Type = `unknown` (unclassified)

Create seed config to fix them.

### Step 4: Apply Seed Config

```bash
graphmd index --dir ./docs --seed-config ./custom_types.yaml
graphmd list --output json > seed_detected.json
```

### Step 5: Verify Changes

```bash
# Compare before and after
diff <(jq '.components | sort_by(.name) | .[].name' auto_detected.json) \
     <(jq '.components | sort_by(.name) | .[].name' seed_detected.json)
```

---

**Last updated:** 2026-03-19
