# Phase 10: Python + JS/TS Parsers - Research

**Researched:** 2026-03-31
**Domain:** Regex-based dependency detection for Python and JavaScript/TypeScript source files
**Confidence:** HIGH

## Summary

Phase 10 implements two new parsers (Python and JavaScript/TypeScript) that conform to the existing `LanguageParser` interface established in Phase 9. The Go parser provides a clear template: each parser implements `Name()`, `Extensions()`, and `ParseFile()`, emitting `CodeSignal` structs. The key difference is that Go uses AST-based detection via `go/ast`, while Python and JS/TS use regex-based pattern matching per the project's CGo-free constraint and the decision to defer tree-sitter to v2.1.

The research confirms that regex-based detection is viable for the target dependency patterns (HTTP calls, DB connections, cache clients, queue producers/consumers) because these patterns are syntactically regular — they involve well-known function/method calls with identifiable names. The primary challenges are: (1) Python's multiple import styles (`import X`, `from X import Y`, `from X import Y as Z`) require careful regex design, (2) JS/TS has two module systems (CommonJS `require()` and ESM `import`) that must both be tracked, and (3) both languages need context-aware filtering to avoid matching patterns inside comments and already-excluded test files.

The LanguageParser interface requires NO changes — both parsers fit cleanly into the existing `RegisterParser` → `AnalyzeFile` → `AnalyzeDir` pipeline. Source component inference needs extension: `InferSourceComponent` currently only checks `go.mod`, and must also check `pyproject.toml`/`setup.py` (Python) and `package.json` (JS/TS).

**Primary recommendation:** Implement each parser as a self-contained subpackage (`internal/code/pyparser/`, `internal/code/jsparser/`) following the GoParser template exactly: a patterns table mapping import+function to detection metadata, a `ParseFile` method that scans line-by-line with regex, and test files using inline source fixtures.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
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

### Claude's Discretion
All implementation decisions delegated to Claude.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CODE-02 | Python language parser detecting imports, HTTP calls (requests/httpx), DB connections (SQLAlchemy/psycopg), queue clients (kafka/pika/boto3 SQS), and cache clients (redis/memcache) | Regex patterns for each library's function call signatures; import tracking to resolve package aliases; test file exclusion patterns |
| CODE-03 | JavaScript/TypeScript language parser detecting imports, HTTP calls (fetch/axios), DB connections (pg/mysql2/mongoose), queue clients, and cache clients | Dual module system support (require/import); regex patterns for each library; file extension mapping (.js/.ts/.jsx/.tsx); test file exclusion patterns |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `regexp` (stdlib) | Go 1.24 | Pattern matching for Python/JS source lines | Only dependency needed; regex is sufficient for function call detection patterns |
| `net/url` (stdlib) | Go 1.24 | URL parsing from detected connection strings | Reuse same approach as GoParser for extracting hostnames |
| `encoding/json` (stdlib) | Go 1.24 | Parse package.json for JS component name inference | Standard JSON parsing, no external deps needed |
| `strings` (stdlib) | Go 1.24 | Line-by-line scanning, comment stripping | Core text processing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `path/filepath` (stdlib) | Go 1.24 | File extension matching, test file detection | Every ParseFile call |
| `bufio` (stdlib) | Go 1.24 | Line-by-line file scanning | Optional alternative to strings.Split for large files |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Regex line scanning | `gotreesitter` (pure-Go tree-sitter) | Tree-sitter provides AST-level accuracy but adds dependency complexity; deferred to v2.1 per project decision |
| Manual JSON parsing | `go/ast` for JS | Not applicable — Go's AST packages are Go-specific; regex is the right choice for JS/TS |
| Line-by-line regex | Multi-line regex | Multi-line catches more patterns (e.g., multi-line function calls) but significantly increases false positive risk and regex complexity |

**Installation:**
No new dependencies required. All stdlib.

## Architecture Patterns

### Recommended Project Structure
```
internal/code/
├── signal.go           # CodeSignal struct (existing)
├── analyzer.go         # LanguageParser interface, CodeAnalyzer (existing)
├── integration.go      # RunCodeAnalysis entry point (existing, needs parser registration update)
├── goparser/
│   ├── parser.go       # GoParser (existing)
│   ├── patterns.go     # Go detection patterns (existing)
│   └── parser_test.go  # (existing)
├── pyparser/
│   ├── parser.go       # PythonParser implementing LanguageParser
│   ├── patterns.go     # Python detection patterns + import regexes
│   └── parser_test.go  # Inline Python source fixtures
└── jsparser/
    ├── parser.go       # JSParser implementing LanguageParser
    ├── patterns.go     # JS/TS detection patterns + import regexes
    └── parser_test.go  # Inline JS/TS source fixtures
```

### Pattern 1: Two-Phase Line Scanning (Import Tracking + Call Detection)

**What:** Each parser scans the file in two passes (or accumulates state in one pass): first, build an import map (which packages are imported under which names), then scan for function calls matching those imports against the pattern table.

**When to use:** Every ParseFile implementation.

**Why:** The Go parser uses AST nodes to resolve imports to call sites. Regex parsers must do this manually: track `import requests` or `const axios = require('axios')`, then match `requests.get(...)` or `axios.post(...)` against known patterns. Without import tracking, every `get()` call would match — catastrophic false positives.

**Example (Python):**
```go
// Phase 1: Build import map
// "import requests" -> importMap["requests"] = "requests"
// "from kafka import KafkaProducer" -> importMap["KafkaProducer"] = "kafka"
// "import redis as r" -> importMap["r"] = "redis"

// Phase 2: Match calls against imports
// Line: `requests.get("http://payment-api:8080/pay")`
// -> "requests" is in importMap, mapped to package "requests"
// -> "requests.get" matches pattern {Package: "requests", Function: "get", Kind: "http_call"}
// -> Extract URL, emit CodeSignal
```

### Pattern 2: DetectionPattern Table (Declarative, Not Imperative)

**What:** Define patterns as data (struct slices), not imperative matching logic. Each pattern maps a package+function to a detection kind, target type, confidence, and argument position for target extraction.

**When to use:** Pattern definitions in `patterns.go` files.

**Why:** The GoParser established this pattern with `DefaultPatterns []DetectionPattern`. Python and JS parsers should follow the same declarative approach. This makes patterns readable, testable independently, and extensible without modifying parser logic.

**Example (Python patterns):**
```go
type PyDetectionPattern struct {
    Package    string  // e.g., "requests"
    Function   string  // e.g., "get"
    Kind       string  // e.g., "http_call"
    TargetType string  // e.g., "service"
    Confidence float64 // e.g., 0.9
    ArgIndex   int     // Position of URL/connection string arg (0-based), -1 for none
}

var DefaultPythonPatterns = []PyDetectionPattern{
    // HTTP calls
    {Package: "requests", Function: "get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
    {Package: "requests", Function: "post", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
    {Package: "httpx", Function: "get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
    // DB connections
    {Package: "psycopg2", Function: "connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
    {Package: "asyncpg", Function: "connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
    // ... etc
}
```

### Pattern 3: Comment Stripping Before Pattern Matching

**What:** Before matching function call patterns, strip or skip comment lines to prevent false positives from commented-out code or documentation examples.

**When to use:** Every line scan iteration.

**Why:** A line like `# requests.get("http://old-api/deprecated")` in Python or `// fetch("http://test-endpoint")` in JS would produce false signals without comment filtering. The Go parser doesn't have this problem because it uses AST (comments are separate nodes), but regex parsers must handle it explicitly.

**Implementation:**
```go
// Python: skip lines where the first non-whitespace is #
// Also handle inline comments: code # comment (only match the code part)
trimmed := strings.TrimSpace(line)
if strings.HasPrefix(trimmed, "#") {
    continue // skip full-line comments
}
// Strip inline comment (naive but sufficient for function call detection)
if idx := strings.Index(line, " #"); idx >= 0 {
    line = line[:idx]
}

// JS/TS: skip lines starting with //
// Multi-line /* */ comments: track state with inBlockComment bool
if strings.HasPrefix(trimmed, "//") {
    continue
}
```

### Anti-Patterns to Avoid

- **Matching bare function names without import context:** Never match `get()`, `connect()`, `open()` etc. without confirming the calling package is a known dependency library. These are absurdly common function names.
- **Multi-line regex for function calls:** Avoid trying to match function calls spanning multiple lines (`requests.get(\n    "http://...")`). The complexity-to-value ratio is terrible. Single-line matching catches 80%+ of real patterns; multi-line adds 30%+ false positive risk.
- **Sharing regex patterns between Python and JS:** Despite superficial similarities, the syntax differences (string quoting, import styles, method chaining) mean patterns should be separate per parser. Shared patterns create coupling that makes each parser harder to debug.
- **Import-only signals:** Per CONTEXT.md, do NOT emit signals for imports alone. `import redis` without a `redis.Redis()` or `redis.from_url()` call should produce zero signals. This is consistent with the GoParser's behavior (TestImportOnlyNoSignals).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL hostname extraction | Custom URL parser | `net/url.Parse` + `Hostname()` | Same as GoParser; handles edge cases (ports, auth, paths) |
| JSON parsing (package.json) | String matching for "name" field | `encoding/json.Unmarshal` into struct | Handles escaping, nesting, encoding correctly |
| String literal extraction from regex | Custom unquoting | Regex capture group + `strings.Trim(val, "\"'")` | Handles both single and double quotes; backtick template literals need separate handling for JS |
| Test file detection | Hardcoded filename checks | Configurable patterns per parser | Each language has different conventions; keep them declarative |

**Key insight:** The regex parsers are simpler than the GoParser (no AST, no import path resolution, no type system), but the simplicity is deceptive. The challenge is not writing the regex — it's controlling false positives. Every decision should prioritize precision over recall: it's better to miss a dependency than to report a false one.

## Common Pitfalls

### Pitfall 1: Python Import Aliasing Creates Silent Misses
**What goes wrong:** `import requests as req` followed by `req.get("http://api/path")` — the parser sees `req.get` but doesn't know `req` maps to `requests`.
**Why it happens:** Python allows arbitrary aliasing with `as`. Without tracking aliases, the import map is incomplete.
**How to avoid:** The import regex must capture the alias: `import\s+(\w+)\s+as\s+(\w+)` maps the alias (group 2) to the package (group 1). Similarly, `from X import Y as Z` maps Z to Y within package X.
**Warning signs:** Tests pass with `import requests` but fail with `import requests as req`.

### Pitfall 2: JS/TS Dual Module System (CommonJS vs ESM)
**What goes wrong:** Parser handles `import axios from 'axios'` but misses `const axios = require('axios')`, or vice versa.
**Why it happens:** JavaScript has two module systems, and real codebases often mix them (especially in TypeScript with `esModuleInterop`).
**How to avoid:** The JS import regex must handle both forms:
- ESM: `import\s+(\w+)\s+from\s+['"]([^'"]+)['"]`
- ESM named: `import\s+\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`
- CommonJS: `(?:const|let|var)\s+(\w+)\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`
**Warning signs:** JS test passes but equivalent TS test with `require()` fails (or vice versa).

### Pitfall 3: Destructured Imports in JS Lose Function Context
**What goes wrong:** `import { get, post } from 'axios'` followed by `get("http://api/path")` — `get` is a bare function call, not `axios.get`.
**Why it happens:** Destructured imports remove the package qualifier, making calls look like plain function calls.
**How to avoid:** Track destructured imports in the import map: `{ get, post }` from `axios` means `get` and `post` map to the `axios` package. Then match bare `get(...)` calls when `get` is in the import map for a known package.
**Warning signs:** `import axios from 'axios'; axios.get(...)` works but `import { get } from 'axios'; get(...)` doesn't.

### Pitfall 4: Test File Skip Patterns Must Be Parser-Specific
**What goes wrong:** Python test files named `test_handler.py` or in `tests/` directories get analyzed, producing false signals from test fixtures and mock URLs.
**Why it happens:** The current `AnalyzeDir` only skips `_test.go` files. Python and JS have different test file conventions.
**How to avoid:** Each parser's `ParseFile` should skip its own test file patterns (like GoParser does for `_test.go`). Additionally, add `tests/` and `__tests__/` to the `skipDirs` map in `analyzer.go`. Specific patterns:
- Python: `*_test.py`, `test_*.py`, `conftest.py`, files in `tests/` directories
- JS/TS: `*.test.js`, `*.test.ts`, `*.spec.js`, `*.spec.ts`, files in `__tests__/` directories
**Warning signs:** Signal count spikes when test data directories are present.

### Pitfall 5: Block Comments in JS/TS Cause False Positives
**What goes wrong:** Code inside `/* ... */` block comments matches function call patterns.
**Why it happens:** Line-by-line regex doesn't naturally handle multi-line block comments.
**How to avoid:** Track a `inBlockComment` boolean state while scanning lines. When `/*` is encountered, set true. When `*/` is encountered, set false. Skip all lines while `inBlockComment` is true. This is a simple state machine — don't try to use multi-line regex for it.
**Warning signs:** Signals detected with evidence from commented-out code blocks.

### Pitfall 6: Python Decorator Patterns Are Not Function Calls
**What goes wrong:** `@app.route("/api/v1/users")` matches a regex looking for function calls with URL arguments.
**Why it happens:** Decorators syntactically resemble function calls with string arguments.
**How to avoid:** When scanning for function calls, exclude lines starting with `@` (after whitespace stripping). Decorator URLs are route definitions, not outbound HTTP calls — they describe incoming traffic, not dependencies.
**Warning signs:** Flask/FastAPI route decorators showing up as HTTP call signals.

## Code Examples

### Python Import Regex Patterns
```go
// Source: Derived from Python language specification
var pyImportPatterns = []*regexp.Regexp{
    // import requests
    regexp.MustCompile(`^\s*import\s+(\w+)\s*$`),
    // import requests as req
    regexp.MustCompile(`^\s*import\s+(\w+)\s+as\s+(\w+)\s*$`),
    // from kafka import KafkaProducer
    regexp.MustCompile(`^\s*from\s+([\w.]+)\s+import\s+(.+)$`),
}
```

### JS/TS Import Regex Patterns
```go
// Source: Derived from ECMAScript and CommonJS module specifications
var jsImportPatterns = []*regexp.Regexp{
    // import axios from 'axios'
    regexp.MustCompile(`^\s*import\s+(\w+)\s+from\s+['"]([^'"]+)['"]`),
    // import { Pool } from 'pg'
    regexp.MustCompile(`^\s*import\s+\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`),
    // const axios = require('axios')
    regexp.MustCompile(`^\s*(?:const|let|var)\s+(\w+)\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`),
    // const { Pool } = require('pg')
    regexp.MustCompile(`^\s*(?:const|let|var)\s+\{([^}]+)\}\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`),
}
```

### Python Function Call Detection
```go
// Match: requests.get("http://payment-api:8080/pay")
// Match: redis.Redis(host="cache-01", port=6379)
// Match: psycopg2.connect("postgres://primary-db:5432/mydb")
var pyCallPattern = regexp.MustCompile(`(\w+)\.([\w]+)\s*\((.*)`)
```

### JS/TS Function Call Detection
```go
// Match: axios.get("http://payment-api:8080/pay")
// Match: mongoose.connect("mongodb://primary-db:27017/mydb")
// Match: fetch("http://user-api:3000/users")  (bare function, no package prefix)
var jsCallPattern = regexp.MustCompile(`(\w+)\.([\w]+)\s*\((.*)`)
var jsBareCallPattern = regexp.MustCompile(`\b(fetch)\s*\((.*)`) // for globals like fetch
```

### Source Component Inference Extension
```go
// InferSourceComponent needs extension for Python and JS:

// Python: check pyproject.toml first, then setup.py
// pyproject.toml: [project] name = "my-service"
// setup.py: name="my-service"

// JS/TS: check package.json
// package.json: {"name": "my-service"}
type packageJSON struct {
    Name string `json:"name"`
}
```

### AnalyzeDir Skip Directory Extension
```go
// Current skipDirs needs extension:
var skipDirs = map[string]bool{
    "vendor":       true,
    "node_modules": true,
    ".git":         true,
    // New for Phase 10:
    "__tests__":    true,
    "__pycache__":  true,
    ".venv":        true,
    "venv":         true,
    ".tox":         true,
}
```

### Comment Hint Pattern (Reusable Across Languages)
```go
// Python uses # comments: "# Calls payment-api", "# Depends on auth-service"
var pyCommentHintPattern = regexp.MustCompile(`#\s*(?:Calls|Depends on|Uses|Connects to)\s+(\S+)`)

// JS/TS uses // comments (same pattern as Go):
var jsCommentHintPattern = regexp.MustCompile(`//\s*(?:Calls|Depends on|Uses|Connects to)\s+(\S+)`)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Python `ast` module (Python stdlib) | Regex in Go | Project decision (v2.0 planning) | Avoids shelling out to Python; keeps pure-Go build |
| Tree-sitter for JS/TS parsing | Regex in Go | Project decision (v2.0 planning) | Avoids CGo dependency; deferred to v2.1 if regex insufficient |
| Single monolithic analyzer | Per-language parser subpackages | Phase 9 design | Each parser is independent, testable, and extensible |

**Deprecated/outdated:**
- `smacker/go-tree-sitter`: Requires CGo, explicitly ruled out per project constraints
- `tree-sitter/go-tree-sitter`: Also requires CGo, same constraint

## Open Questions

1. **Multi-line function calls in Python/JS**
   - What we know: Real-world code frequently breaks function calls across lines, e.g., `requests.get(\n    "http://api/path",\n    headers=headers\n)`
   - What's unclear: What percentage of real dependency calls are multi-line vs single-line
   - Recommendation: Start with single-line matching only. If precision is adequate (>85%), defer multi-line to a follow-up. If not, add a simple "continuation" heuristic: if a line matches `package.function(` but doesn't contain `)`, look at the next line for the URL argument.

2. **Python virtual environment detection**
   - What we know: `InferSourceComponent` walks up looking for `go.mod`. For Python, should also check `pyproject.toml` and `setup.py`.
   - What's unclear: Whether to check `setup.cfg` (older format) or just `pyproject.toml` + `setup.py`
   - Recommendation: Check `pyproject.toml` first (modern standard), then `setup.py` (legacy). Skip `setup.cfg` — it's rare for new projects and adds complexity. Extract `name` field from either.

3. **TypeScript path aliases**
   - What we know: `tsconfig.json` `paths` config allows `import { foo } from '@app/foo'` where `@app` maps to a local directory
   - What's unclear: Whether to resolve path aliases or treat them as unknown imports
   - Recommendation: Do NOT resolve path aliases in v2.0. Path aliases map to local modules, not external dependencies. Since we only detect calls to known infrastructure libraries (requests, axios, pg, etc.), path-aliased local imports won't match any patterns. No action needed.

4. **`AnalyzeDir` test directory skipping vs parser-level skipping**
   - What we know: GoParser skips `_test.go` in its own `ParseFile`. `AnalyzeDir` also skips `_test.go` files.
   - What's unclear: Whether to add Python/JS test patterns to `AnalyzeDir` or keep all test skipping in parsers
   - Recommendation: Add `__tests__/`, `__pycache__/`, `.venv/`, `venv/`, `.tox/` to the `skipDirs` map in `analyzer.go` (directory-level skipping). Keep file-level test skipping in each parser's `ParseFile` (e.g., `test_*.py`, `*.test.js`). This is consistent with how GoParser handles `_test.go` in both places.

## Python Detection Patterns (Complete Reference)

### HTTP Calls
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `requests` | `get`, `post`, `put`, `delete`, `patch`, `head`, `options` | 0.9 | Most common Python HTTP library |
| `httpx` | `get`, `post`, `put`, `delete`, `patch` | 0.9 | Modern async-capable HTTP library |
| `httpx.AsyncClient` | `get`, `post`, `put`, `delete` | 0.9 | Async variant — matched as method chain |
| `aiohttp.ClientSession` | `get`, `post`, `put`, `delete` | 0.9 | Async HTTP — often used with `async with` |
| `urllib.request` | `urlopen` | 0.85 | Stdlib, less common in modern code |

### Database Connections
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `psycopg2` | `connect` | 0.85 | PostgreSQL adapter |
| `psycopg` | `connect` | 0.85 | psycopg3 (newer) |
| `asyncpg` | `connect`, `create_pool` | 0.85 | Async PostgreSQL |
| `pymysql` | `connect` | 0.85 | MySQL adapter |
| `pymongo.MongoClient` | constructor | 0.85 | MongoDB — `MongoClient("mongodb://...")` |
| `sqlalchemy.create_engine` | `create_engine` | 0.85 | SQLAlchemy ORM |

### Cache Clients
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `redis.Redis` | constructor | 0.85 | Redis client |
| `redis.from_url` | `from_url` | 0.85 | Redis from URL |
| `redis.StrictRedis` | constructor | 0.85 | Older Redis client class |
| `pymemcache.client.base.Client` | constructor | 0.85 | Memcache client |

### Queue Clients
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `kafka.KafkaProducer` | constructor | 0.85 | kafka-python producer |
| `kafka.KafkaConsumer` | constructor | 0.85 | kafka-python consumer |
| `pika.BlockingConnection` | constructor | 0.85 | RabbitMQ |
| `boto3.client` | `client("sqs", ...)` | 0.85 | AWS SQS — match when first arg is "sqs" |

## JS/TS Detection Patterns (Complete Reference)

### HTTP Calls
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `axios` | `get`, `post`, `put`, `delete`, `patch`, `request` | 0.9 | Most common HTTP library |
| `fetch` (global) | bare call | 0.9 | Built-in, no import needed — special case |
| `node-fetch` | default import, bare call | 0.9 | Polyfill for older Node.js |
| `got` | `get`, `post`, `put`, `delete` | 0.9 | Alternative HTTP library |
| `http` / `https` (Node) | `request`, `get` | 0.85 | Node stdlib |

### Database Connections
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `pg` | `Pool` constructor, `Client` constructor | 0.85 | PostgreSQL — `new Pool(...)`, `new Client(...)` |
| `mysql2` | `createConnection`, `createPool` | 0.85 | MySQL |
| `mongoose` | `connect` | 0.85 | MongoDB ODM |
| `mongodb` | `MongoClient` constructor | 0.85 | MongoDB native driver |
| `@prisma/client` | `PrismaClient` constructor | 0.85 | Prisma ORM |
| `knex` | default import/require (factory call) | 0.85 | Query builder |
| `sequelize` | `Sequelize` constructor | 0.85 | ORM |

### Cache Clients
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `ioredis` | `Redis` constructor | 0.85 | `new Redis(...)` |
| `redis` | `createClient` | 0.85 | Node redis — `redis.createClient(...)` |

### Queue Clients
| Package | Functions | Confidence | Notes |
|---------|-----------|------------|-------|
| `kafkajs` | `Kafka` constructor | 0.85 | `new Kafka({brokers: [...]})` |
| `amqplib` | `connect` | 0.85 | RabbitMQ — `amqplib.connect(...)` |
| `@aws-sdk/client-sqs` | `SQSClient` constructor | 0.85 | AWS SQS v3 |
| `aws-sdk` | `SQS` constructor | 0.85 | AWS SQS v2 |

## Integration Points

### Where New Parsers Get Registered

Parsers are currently registered at three call sites in the codebase, all passing `goparser.NewGoParser()` to `code.RunCodeAnalysis()`:

1. `cmd/graphmd/main.go:181` — CLI direct invocation
2. `internal/knowledge/export.go:228` — Export command
3. `internal/knowledge/crawl_cmd.go:151` — Crawl command

All three must be updated to also pass `pyparser.NewPythonParser()` and `jsparser.NewJSParser()`. The variadic `parsers ...LanguageParser` parameter on `RunCodeAnalysis` was designed for exactly this:

```go
// Before (Phase 9):
signals, err := code.RunCodeAnalysis(dir, goparser.NewGoParser())

// After (Phase 10):
signals, err := code.RunCodeAnalysis(dir,
    goparser.NewGoParser(),
    pyparser.NewPythonParser(),
    jsparser.NewJSParser(),
)
```

### InferSourceComponent Extension

Currently only checks `go.mod`. Needs to also check:
- `pyproject.toml` — extract `[project] name = "..."` (TOML parsing with regex is sufficient for this single field)
- `setup.py` — extract `name="..."` from `setup()` call (regex)
- `package.json` — extract `"name": "..."` (JSON unmarshal)

Order of precedence when multiple manifest files exist: `go.mod` > `pyproject.toml` > `setup.py` > `package.json` > `filepath.Base(dir)` fallback.

### AnalyzeDir Updates

Add to `skipDirs` map: `__tests__`, `__pycache__`, `.venv`, `venv`, `.tox`, `dist`, `build`.

The current Go-specific `_test.go` skip in `AnalyzeDir` should remain (it's a performance optimization — avoids reading test files before dispatching to GoParser). Do NOT add Python/JS test file patterns to `AnalyzeDir` — let each parser handle its own test skipping in `ParseFile` for consistency with the GoParser pattern.

## Sources

### Primary (HIGH confidence)
- Existing codebase: `internal/code/analyzer.go`, `internal/code/signal.go`, `internal/code/goparser/parser.go`, `internal/code/goparser/patterns.go` — direct analysis of the interface and patterns that Python/JS parsers must conform to
- Existing codebase: `internal/code/integration.go` — entry point showing how parsers are registered and invoked
- Existing codebase: `cmd/graphmd/main.go`, `internal/knowledge/export.go`, `internal/knowledge/crawl_cmd.go` — all call sites that need parser registration updates
- Go standard library `regexp`, `net/url`, `encoding/json` — well-understood, stable APIs

### Secondary (MEDIUM confidence)
- Python library APIs (requests, psycopg2, redis, kafka-python) — function signatures based on training data knowledge; patterns are syntactically regular and well-established
- JS/TS library APIs (axios, pg, mongoose, ioredis, kafkajs) — function signatures based on training data knowledge; patterns are syntactically regular and well-established
- Python/JS import syntax — based on language specifications (ECMAScript modules, Python import system)

### Tertiary (LOW confidence)
- Multi-line function call frequency — estimated 20-30% of real dependency calls span multiple lines; this is an educated guess, not measured
- `aiohttp.ClientSession` detection complexity — async context manager pattern may require special handling beyond simple regex

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, no new dependencies, proven patterns from GoParser
- Architecture: HIGH — direct extension of established LanguageParser interface; three files per parser following GoParser template exactly
- Pitfalls: HIGH — derived from direct analysis of existing GoParser implementation and known language syntax differences
- Detection patterns: MEDIUM — library function signatures are well-known but not verified against current library versions via Context7

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (stable — no fast-moving dependencies)
