# Feature Research

**Domain:** Code analysis dependency detection + MCP server for infrastructure graph tool
**Researched:** 2026-03-29
**Confidence:** MEDIUM-HIGH

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist once "code analysis" is advertised. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Import/package detection** (Go, Python, JS/TS) | Every code analysis tool does this; baseline signal | MEDIUM | Go: `go/parser` + `go/ast` stdlib. Python: `ast` stdlib (Import/ImportFrom nodes). JS/TS: regex or lightweight parser for `import`/`require`. Each language needs its own parser but the pattern is well-understood. |
| **HTTP client call detection** | Service-to-service calls are the primary dependency type in microservices | MEDIUM | Detect `http.Get/Post/Do`, `requests.get/post`, `fetch/axios` calls. Extract URLs/hostnames. Match against known component names. |
| **Database connection detection** | DB connections are the second most common dependency type | MEDIUM | Detect `sql.Open`, `psycopg2.connect`, `pg.Pool`, `mongoose.connect`, etc. Parse DSN/connection strings: `postgres://`, `mysql://`, `redis://`, `mongodb://`. Match hostname/dbname to components. |
| **Connection string/URL parsing** | Extracting the target from detected connections | LOW | Standard URL parsing + DSN format recognition. Patterns: `protocol://host:port/db`, env var references (`os.Getenv("DATABASE_URL")`). |
| **Signal merging (code + markdown)** | v1 has markdown signals; code signals must integrate, not replace | HIGH | Confidence-weighted merge of code-detected edges with existing markdown-detected edges. Same edge from both sources = higher confidence. New edges from code get code-specific confidence tier. |
| **MCP server with stdio transport** | Standard MCP transport for local LLM clients (Claude Code, Copilot CLI) | MEDIUM | Official Go SDK (`modelcontextprotocol/go-sdk`) or `mark3labs/mcp-go`. Wrap existing query interface as MCP tools. stdio is the expected default transport. |
| **MCP tool: query_impact** | Direct mapping of existing `graphmd query impact` | LOW | Wrap existing impact query. Input: component name, optional depth. Output: affected components with confidence. |
| **MCP tool: query_dependencies** | Direct mapping of existing `graphmd query dependencies` | LOW | Wrap existing dependencies query. Input: component name. Output: upstream dependencies. |
| **MCP tool: query_path** | Direct mapping of existing `graphmd query path` | LOW | Wrap existing path query. Input: source, target. Output: path between components. |
| **MCP tool: list_components** | Direct mapping of existing `graphmd query list` | LOW | Wrap existing list query. Input: optional type filter. Output: component list. |

### Differentiators (Competitive Advantage)

Features that set graphmd apart. Not required, but valuable given the "AI agents query infrastructure" positioning.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Message queue producer/consumer detection** | Most tools stop at imports/HTTP; detecting Kafka/RabbitMQ/SQS producer-consumer relationships maps async dependencies that are invisible in call graphs | HIGH | Detect patterns: `kafka.NewProducer`, `channel.Publish` (RabbitMQ), `sqs.SendMessage`. Must identify topic/queue names to match producers to consumers across services. Pattern-based, not type-system-based. |
| **Cache connection detection** | Redis/Memcached connections create implicit dependencies rarely tracked | LOW-MEDIUM | Detect `redis.NewClient`, `memcache.New`, etc. Lower complexity than queues because cache is typically point-to-point. |
| **Multi-source confidence boosting** | Edge found in both markdown AND code gets higher confidence than either alone | MEDIUM | Leverages existing 6-tier confidence system. "Markdown says A->B, code confirms A calls B" = high confidence. Unique to graphmd's hybrid approach. |
| **Env var reference tracking** | Many connections use env vars (`os.Getenv("REDIS_URL")`); tracking these as "unresolved references" signals dependencies even without runtime values | LOW | Don't resolve the env var (can't at static analysis time). Record it as a dependency signal: "service X references DATABASE_URL" = likely DB dependency. |
| **MCP structured output (outputSchema)** | MCP spec supports `outputSchema` for typed results; most servers skip it | LOW | Define JSON schemas for each tool's output. Helps LLM agents parse results reliably. graphmd already has well-defined JSON envelopes. |
| **MCP graph metadata tool** | Expose graph metadata (component count, edge count, confidence distribution, last export time) as a tool | LOW | Agents can assess graph freshness and coverage before querying. Maps to existing crawl stats. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems for graphmd's scope and architecture.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Full AST type resolution** | "We need to know the exact type of every variable to detect connections" | Requires full type-checking (Go: `go/types`, Python: mypy-level). 10x complexity for marginal detection improvement. Most connections use well-known constructor patterns detectable by name. | Pattern-based detection on function/package names. `sql.Open(...)` doesn't need type resolution to identify a DB connection. |
| **Runtime/dynamic analysis** | "Static analysis misses dynamically constructed URLs and reflection" | Requires running the code, managing environments, dependencies. Completely different architecture. | Accept that static analysis catches 70-80% of dependencies. Flag unresolved env vars as "potential dependency" signals. |
| **Code flow tracing (call chains within a service)** | "Show me the call path from HTTP handler to DB query" | Intra-service analysis, not inter-service dependency detection. Different problem domain. Already explicitly deferred to v2.1. | Keep scope to inter-service/inter-component dependencies. Code flows are a separate feature. |
| **IaC/Docker Compose parsing** | "Parse Terraform/K8s manifests for infrastructure dependencies" | Different parsers, different dependency semantics (provisioning vs runtime). Already out of scope per PROJECT.md. | Markdown + code analysis covers the runtime dependency graph. IaC is a separate graph. |
| **Natural language query via MCP** | "Let agents ask questions in English instead of structured queries" | The LLM agent IS the NLP layer. Adding NLP inside graphmd duplicates the agent's job and adds fragility. | Expose structured tools with clear descriptions. The agent translates user intent to tool calls. |
| **SSE/HTTP MCP transport** | "Support remote MCP connections over HTTP" | Adds networking, auth, CORS, deployment complexity. graphmd's deployment model is local container. | stdio transport only for v2.0. Container runs MCP server locally. HTTP transport is a future consideration if remote access is needed. |
| **Real-time file watching** | "Detect code changes and update the graph automatically" | graphmd uses batch export/import model. Real-time watching adds complexity (debouncing, partial updates, consistency) with minimal value for the incident-response use case. | Re-run `export` in CI on code changes. Batch model is sufficient. |

## Feature Dependencies

```
[Language Parsers (Go/Python/JS)]
    └──requires──> [Import Detection]
    └──requires──> [HTTP Client Detection]
    └──requires──> [DB Connection Detection]
    └──requires──> [Queue Producer/Consumer Detection]
    └──requires──> [Cache Connection Detection]

[HTTP Client Detection]
    └──requires──> [Connection String/URL Parsing]

[DB Connection Detection]
    └──requires──> [Connection String/URL Parsing]

[Signal Merging]
    └──requires──> [Language Parsers]
    └──requires──> [Existing Markdown Detection (v1)]

[MCP Server]
    └──requires──> [Existing Query Interface (v1)]
    └──enhances──> [Signal Merging] (richer graph = better query results)

[MCP Tools (impact/deps/path/list)]
    └──requires──> [MCP Server]

[MCP Graph Metadata Tool]
    └──requires──> [MCP Server]

[Env Var Reference Tracking]
    └──enhances──> [DB Connection Detection]
    └──enhances──> [HTTP Client Detection]
```

### Dependency Notes

- **Language Parsers are the foundation:** All code detection features require per-language parsing. This is the gating work.
- **Connection String Parsing is shared infrastructure:** Both HTTP and DB detection need URL/DSN parsing. Build once, use everywhere.
- **Signal Merging depends on both old and new:** Must integrate code signals with existing markdown signals. This is the integration point.
- **MCP Server is independent of code analysis:** Can be built in parallel since it wraps the existing query interface. Does not require code analysis to be complete.
- **MCP gets better with code analysis:** More edges in the graph = more useful query results. But MCP is functional with markdown-only graphs.

## MVP Definition

### Launch With (v2.0)

Minimum to claim "code analysis + MCP" capability.

- [ ] **Go parser with import + HTTP + DB detection** — Go is graphmd's own language; strongest ecosystem knowledge. Validates the detection pipeline.
- [ ] **Python parser with import + HTTP + DB detection** — Largest user base for infrastructure tooling.
- [ ] **JS/TS parser with import + HTTP + DB detection** — Covers the three most common backend languages.
- [ ] **Connection string/URL parsing** — Shared infrastructure for extracting targets from detected connections.
- [ ] **Signal merging (code + markdown)** — Without this, code and markdown are separate worlds. The hybrid approach IS the differentiator.
- [ ] **MCP server with stdio transport** — Enables LLM agent integration, the core value proposition.
- [ ] **MCP tools: impact, dependencies, path, list** — Direct mapping of existing query interface.

### Add After Validation (v2.x)

Features to add once core code analysis + MCP is working.

- [ ] **Message queue producer/consumer detection** — High value but high complexity. Needs topic/queue name matching across services.
- [ ] **Cache connection detection** — Lower complexity addition once DB detection pipeline exists.
- [ ] **Env var reference tracking** — Signals dependencies even without runtime values. Low complexity addition.
- [ ] **MCP graph metadata tool** — Useful for agent self-assessment. Low complexity.
- [ ] **MCP structured output (outputSchema)** — Improves agent reliability. Low complexity, high polish.

### Future Consideration (v2.1+)

Features to defer until code analysis is proven.

- [ ] **Code flow tracing** — Already deferred per PROJECT.md. Intra-service call chains.
- [ ] **Additional language parsers** (Rust, Java, C#) — Extend coverage based on user demand.
- [ ] **HTTP/SSE MCP transport** — Remote access for non-local deployments.
- [ ] **gRPC/protobuf detection** — Service definitions in .proto files map to dependencies.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Go parser + detection | HIGH | MEDIUM | P1 |
| Python parser + detection | HIGH | MEDIUM | P1 |
| JS/TS parser + detection | HIGH | MEDIUM | P1 |
| Connection string parsing | HIGH | LOW | P1 |
| Signal merging | HIGH | HIGH | P1 |
| MCP server (stdio) | HIGH | MEDIUM | P1 |
| MCP query tools (4) | HIGH | LOW | P1 |
| Queue detection | MEDIUM | HIGH | P2 |
| Cache detection | MEDIUM | LOW-MEDIUM | P2 |
| Env var tracking | MEDIUM | LOW | P2 |
| MCP metadata tool | LOW-MEDIUM | LOW | P2 |
| MCP outputSchema | LOW-MEDIUM | LOW | P2 |
| Code flows | MEDIUM | HIGH | P3 |
| gRPC detection | MEDIUM | MEDIUM | P3 |
| Additional languages | LOW-MEDIUM | MEDIUM each | P3 |
| HTTP MCP transport | LOW | MEDIUM | P3 |

**Priority key:**
- P1: Must have for v2.0 launch
- P2: Should have, add in v2.x iterations
- P3: Nice to have, future consideration

## Code Analysis: Detection Patterns by Dependency Type

### Import/Package Dependencies

What to detect and how, per language:

| Language | Pattern | AST Node / Regex | Example |
|----------|---------|------------------|---------|
| Go | `import "pkg"` | `go/ast.ImportSpec` | `import "github.com/org/service-b/client"` |
| Go | `import (...)` block | `go/ast.GenDecl` with `IMPORT` token | Multi-import blocks |
| Python | `import module` | `ast.Import` | `import redis` |
| Python | `from module import name` | `ast.ImportFrom` | `from kafka import KafkaProducer` |
| JS/TS | `import ... from "pkg"` | Regex or lightweight parser | `import axios from 'axios'` |
| JS/TS | `require("pkg")` | Regex | `const pg = require('pg')` |

### HTTP Client Calls (Service-to-Service)

| Language | Pattern | What to Extract |
|----------|---------|-----------------|
| Go | `http.Get(url)`, `http.Post(url, ...)`, `http.NewRequest(method, url, ...)`, `client.Do(req)` | URL string literal or variable |
| Go | `grpc.Dial(target, ...)` | Target address |
| Python | `requests.get(url)`, `requests.post(url, ...)`, `httpx.get(url)` | URL string literal |
| Python | `aiohttp.ClientSession().get(url)` | URL string literal |
| JS/TS | `fetch(url)`, `axios.get(url)`, `axios.post(url, ...)` | URL string literal |
| JS/TS | `http.request(options)` | Options object with hostname |

### Database Connections

| Language | Pattern | Connection String Format |
|----------|---------|------------------------|
| Go | `sql.Open("postgres", dsn)` | `postgres://user:pass@host:5432/db` |
| Go | `sql.Open("mysql", dsn)` | `user:pass@tcp(host:3306)/db` |
| Go | `mongo.Connect(ctx, options.Client().ApplyURI(uri))` | `mongodb://host:27017/db` |
| Python | `psycopg2.connect(dsn)` | `host=... dbname=...` or URL |
| Python | `pymysql.connect(host=..., db=...)` | Keyword args |
| Python | `pymongo.MongoClient(uri)` | `mongodb://host/db` |
| JS/TS | `new Pool({connectionString})` (pg) | `postgres://...` |
| JS/TS | `mongoose.connect(uri)` | `mongodb://...` |
| JS/TS | `mysql.createConnection(config)` | Config object or URL |

### Message Queue Producers/Consumers

| Language | Pattern | What to Extract |
|----------|---------|-----------------|
| Go | `sarama.NewSyncProducer`, `sarama.NewConsumer` | Broker addresses, topic names |
| Go | `amqp.Dial(url)`, `ch.Publish(exchange, key, ...)` | AMQP URL, exchange/routing key |
| Python | `KafkaProducer(bootstrap_servers=...)`, `KafkaConsumer(topic, ...)` | Servers, topics |
| Python | `pika.BlockingConnection(params)`, `channel.basic_publish(...)` | AMQP params, exchange/routing |
| JS/TS | `new Kafka({brokers})`, `producer.send({topic})` | Brokers, topics |

### Cache Connections

| Language | Pattern | What to Extract |
|----------|---------|-----------------|
| Go | `redis.NewClient(&redis.Options{Addr: ...})` | Address |
| Go | `memcache.New(server)` | Server address |
| Python | `redis.Redis(host=..., port=...)` | Host, port |
| Python | `Redis.from_url(url)` | Redis URL |
| JS/TS | `new Redis(url)` (ioredis) | Redis URL |
| JS/TS | `createClient({url})` (node-redis) | URL |

## MCP Server: Tool Design Patterns

### Tool Design Principles (from MCP best practices)

1. **Outcome-oriented, not operation-oriented.** Each tool should accomplish a specific goal, not expose raw API operations. graphmd's existing query patterns already map well to this (each query answers a specific question).

2. **Flat arguments, not nested objects.** Use `component: string, depth: int` not `options: {component: string, depth: int}`. Agents handle flat args better.

3. **Snake_case tool names** with service prefix: `graphmd_query_impact`, `graphmd_list_components`, `graphmd_graph_info`.

4. **5-15 tools max** per server. graphmd's 4 query types + metadata = 5 tools. Well within the sweet spot.

5. **Descriptions are instructions for the agent.** Lead with the most important info. Include when to use each tool and what the output means.

6. **Structured output with outputSchema** for reliable parsing. graphmd already has well-defined JSON envelopes that map directly to output schemas.

### Recommended Tool Definitions

| Tool Name | Input | Output | When Agent Uses It |
|-----------|-------|--------|-------------------|
| `graphmd_query_impact` | `component: string, depth?: int` | Affected components with confidence, dependency chain | "If X fails, what breaks?" |
| `graphmd_query_dependencies` | `component: string, depth?: int` | Upstream dependencies with confidence | "What does X depend on?" |
| `graphmd_query_path` | `source: string, target: string` | Path between components, if one exists | "Is X connected to Y? How?" |
| `graphmd_list_components` | `type?: string` | Component list with types and confidence | "What components exist? What services are there?" |
| `graphmd_graph_info` | (none) | Component count, edge count, confidence distribution, graph name, export time | "How fresh is this graph? How complete is it?" |

### MCP Go SDK Selection

**Recommendation: Official Go SDK** (`github.com/modelcontextprotocol/go-sdk`)

| Criterion | Official SDK | mcp-go (community) |
|-----------|-------------|---------------------|
| Maintained by | MCP project + Google | Community (mark3labs) |
| Go struct → JSON Schema | Automatic (generics) | Manual or reflection |
| Transport | stdio + cmd | stdio + HTTP + SSE + in-process |
| Maturity | v1.2.0 (Jan 2026) | Longer history, more battle-tested |
| Spec compliance | Canonical | Tracks spec closely |

The official SDK is sufficient for graphmd's needs (stdio transport only). The automatic schema generation from Go structs reduces boilerplate for tool definitions.

## Competitor Feature Analysis

| Feature | Emerge (code viz) | deps.dev (Google) | Snyk (SCA) | graphmd v2 |
|---------|-------------------|-------------------|------------|------------|
| Import analysis | Yes (file-level) | Yes (package-level) | Yes (package-level) | Yes (file + component mapping) |
| HTTP call detection | No | No | No | Yes — differentiator |
| DB connection detection | No | No | No | Yes — differentiator |
| Queue detection | No | No | No | Yes (P2) |
| Markdown signal fusion | No | No | No | Yes — unique |
| Vulnerability tracking | No | No | Yes (core) | No (not our domain) |
| Dependency graph query | Limited | API | Limited | Full query interface (impact, deps, path, list) |
| MCP integration | No | No | No | Yes — differentiator |
| AI agent optimization | No | No | No | Yes (core value) |

**Key insight:** Existing tools focus on package-level dependencies (what libraries does this use?) or vulnerability scanning. None combine code-level service dependency detection with documentation signals and AI-agent-queryable output. graphmd's niche is clear.

## Sources

- [Go AST packages (go/parser, go/ast)](https://pkg.go.dev/go/ast) — HIGH confidence
- [Python ast module](https://realpython.com/ref/stdlib/ast/) — HIGH confidence
- [MCP Tools Specification (2025-06-18)](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) — HIGH confidence
- [MCP Best Practices (Philipp Schmid)](https://www.philschmid.de/mcp-best-practices) — HIGH confidence
- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) — HIGH confidence
- [mcp-go community SDK](https://github.com/mark3labs/mcp-go) — HIGH confidence
- [MCP Architecture Patterns (IBM)](https://developer.ibm.com/articles/mcp-architecture-patterns-ai-systems/) — MEDIUM confidence
- [COD Model: Codebase Dependency Mapping (Augment)](https://www.augmentcode.com/learn/cod-model-5-phase-guide-to-codebase-dependency-mapping) — MEDIUM confidence
- [Database connection patterns across languages](https://hostman.com/tutorials/database-connection-in-python-go-and-javascript/) — HIGH confidence
- [Google importlab (Python dependency inference)](https://github.com/google/importlab) — MEDIUM confidence
- [Pyan (Python call graph analysis)](https://github.com/davidfraser/pyan) — MEDIUM confidence

---
*Feature research for: graphmd v2.0 code analysis + MCP*
*Researched: 2026-03-29*
