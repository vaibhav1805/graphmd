# Phase 10: Python + JS/TS Parsers - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement Python and JavaScript/TypeScript parsers conforming to the LanguageParser interface from Phase 9. Both use regex-based detection. Detect the same dependency categories as the Go parser: HTTP calls, database connections, message queue clients, and cache clients. Validate that the LanguageParser abstraction generalizes without interface changes.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

User delegated all implementation decisions to Claude. The following guidelines from Phase 9 patterns and milestone research should be followed:

**Python parser (CODE-02):**
- Detect: imports, requests/httpx HTTP calls, SQLAlchemy/psycopg/asyncpg DB connections, kafka-python/pika/boto3 SQS queue clients, redis/pymemcache cache clients
- Regex-based pattern matching on Python source files (.py)
- Same confidence tiers as Go: API call 0.9, connection string 0.85, comment hint 0.4
- Exclude test files (*_test.py, test_*.py, tests/ directories)
- No import-only detections — require usage evidence
- Handle Python-specific patterns: decorators (@app.route), context managers (with), f-strings with URLs

**JavaScript/TypeScript parser (CODE-03):**
- Detect: imports/require, fetch/axios HTTP calls, pg/mysql2/mongoose/prisma DB connections, kafkajs/amqplib queue clients, ioredis/redis cache clients
- Regex-based pattern matching on JS/TS files (.js, .ts, .jsx, .tsx)
- Same confidence tiers as Go
- Exclude test files (*.test.js, *.spec.ts, __tests__/ directories)
- No import-only detections — require usage evidence
- Handle JS-specific patterns: async/await, template literals with URLs, destructured imports

**Cross-cutting:**
- Both parsers must implement LanguageParser interface WITHOUT changes to the interface
- Use the same CodeSignal struct from Phase 9
- Same `--analyze-code` flag triggers both parsers automatically (registered alongside Go parser)
- False positive control: confidence below 0.5 for ambiguous patterns, skip string literals in comments
- Source component inference: Python from setup.py/pyproject.toml package name, JS from package.json name field

</decisions>

<specifics>
## Specific Ideas

- Research recommends regex-first for Python/JS — tree-sitter deferred to v2.1 if accuracy proves insufficient
- The LanguageParser interface was designed in Phase 9 to be language-agnostic — this phase validates that design
- Each parser gets its own subpackage: internal/code/pyparser/, internal/code/jsparser/

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 10-python-js-ts-parsers*
*Context gathered: 2026-03-31*
