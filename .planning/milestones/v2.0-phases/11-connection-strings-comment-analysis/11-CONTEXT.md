# Phase 11: Connection Strings + Comment Analysis - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Two shared cross-language utilities that enhance all three parsers: (1) connection string detection parsing URLs/DSNs/env vars into component names, and (2) comment analysis extracting dependency hints from code comments and docstrings. Both produce CodeSignal output and work across Go, Python, and JS/TS.

</domain>

<decisions>
## Implementation Decisions

### Connection String Detection (CODE-04)

Claude's discretion, guided by research:
- Detect URL-format (postgres://host/db, redis://host:6379), DSN-format, and environment variable references ($DATABASE_URL, os.Getenv("REDIS_URL"))
- Shared utility callable from all three language parsers
- Extract target component name from URL hostname/path
- Confidence 0.85 for parsed connection strings (same as code-based detection)

### Comment Analysis (CODE-05)

**Comment pattern types to extract:**
- Explicit dependency mentions: "// Calls payment-api", "# Depends on Redis", "// Uses primary-db"
- TODO/FIXME with component references: "// TODO: migrate to new-db", "// FIXME: redis timeout"
- Docstrings / function docs: Python docstrings, JSDoc, Go doc comments describing service interactions
- URL references in comments: "// endpoint: http://user-service/api/v1", "# redis://cache:6379"

**Confidence tiered by discovery type:**
- Known component referenced in comment → 0.5 (corroborating evidence for existing graph node)
- New component from explicit pattern ("Depends on X", "Calls X") → 0.4 (discovery signal)
- New component from ambiguous pattern → 0.3 (speculative)

**Detection kind:** `comment_hint` for all comment-derived signals (consistent with Phase 9)

**Architecture:**
- Shared utility function called by all 3 language parsers (not language-specific parsing)
- One comment analyzer handles all comment syntaxes (// # /** */ """ ''')
- Can both enhance known components AND discover new ones

### Claude's Discretion

- Exact regex patterns for comment extraction
- How to identify "component-like" names in ambiguous comments
- Whether to scan .env files as part of connection string detection
- Connection string detection integration point (shared utility vs per-parser)
- How known component list is passed to the comment analyzer

</decisions>

<specifics>
## Specific Ideas

- Comments can discover undocumented dependencies ("legacy-auth-service" only mentioned in comments, not in code or markdown)
- The two-tier confidence (known: 0.5, discovered: 0.4/0.3) lets agents control noise by filtering on confidence
- Connection string detection is particularly valuable for database relationships where the code just calls `sql.Open(driver, connStr)` — the URL reveals the actual target

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 11-connection-strings-comment-analysis*
*Context gathered: 2026-03-31*
