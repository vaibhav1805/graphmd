# Phase 11: Connection Strings + Comment Analysis - Research

**Researched:** 2026-04-01
**Domain:** Cross-language connection string parsing and comment-based dependency extraction
**Confidence:** HIGH

## Summary

Phase 11 adds two shared cross-language utilities that enhance all three existing parsers (Go, Python, JS/TS): (1) a connection string detector that parses URLs, DSNs, and environment variable references into component names, and (2) a comment analyzer that extracts dependency hints from inline comments, docstrings, and TODO/FIXME annotations across all supported languages. Both utilities produce `CodeSignal` output and are callable from any `LanguageParser` implementation.

The existing codebase already has partial implementations of both capabilities. Each parser currently has its own `extractURLHost` function (Go uses `net/url` on AST `BasicLit` nodes; Python and JS use `net/url` plus host:port parsing via regex). The Go parser already detects `// Calls X` / `// Depends on X` patterns via `commentHintPattern`, and the Python/JS parsers have equivalent `commentHintRe` / `commentHintPattern` regexes. Phase 11's task is to (a) consolidate and significantly extend these into shared utilities, and (b) add new capabilities: DSN parsing, env var reference detection, multi-syntax comment handling, docstring extraction, TODO/FIXME scanning, and tiered confidence scoring for known vs. new components.

**Primary recommendation:** Create two new shared packages (`internal/code/connstring/` and `internal/code/comments/`) that the three existing parsers call instead of their current inline implementations. The connection string package uses `net/url` from stdlib for URL-format strings, regex for DSN formats, and regex for environment variable references. The comment analyzer uses a single regex-driven scanner that handles all comment syntaxes (`//`, `#`, `/* */`, `""" """`, `''' '''`, `/** */`) and produces signals at tiered confidence levels.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Connection String Detection (CODE-04):**
- Detect URL-format (postgres://host/db, redis://host:6379), DSN-format, and environment variable references ($DATABASE_URL, os.Getenv("REDIS_URL"))
- Shared utility callable from all three language parsers
- Extract target component name from URL hostname/path
- Confidence 0.85 for parsed connection strings (same as code-based detection)

**Comment Analysis (CODE-05):**
- Comment pattern types to extract:
  - Explicit dependency mentions: "// Calls payment-api", "# Depends on Redis", "// Uses primary-db"
  - TODO/FIXME with component references: "// TODO: migrate to new-db", "// FIXME: redis timeout"
  - Docstrings / function docs: Python docstrings, JSDoc, Go doc comments describing service interactions
  - URL references in comments: "// endpoint: http://user-service/api/v1", "# redis://cache:6379"
- Confidence tiered by discovery type:
  - Known component referenced in comment -> 0.5 (corroborating evidence for existing graph node)
  - New component from explicit pattern ("Depends on X", "Calls X") -> 0.4 (discovery signal)
  - New component from ambiguous pattern -> 0.3 (speculative)
- Detection kind: `comment_hint` for all comment-derived signals
- Shared utility function called by all 3 language parsers (not language-specific parsing)
- One comment analyzer handles all comment syntaxes (// # /** */ """ ''')
- Can both enhance known components AND discover new ones

### Claude's Discretion
- Exact regex patterns for comment extraction
- How to identify "component-like" names in ambiguous comments
- Whether to scan .env files as part of connection string detection
- Connection string detection integration point (shared utility vs per-parser)
- How known component list is passed to the comment analyzer

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CODE-04 | Connection string detection and parsing across all languages -- URLs, DSNs, environment variable references, config file patterns | Shared `connstring` package with URL parser, DSN regex patterns, env var reference detection; called from all three parsers' `extractTarget` methods |
| CODE-05 | Code comment analysis extracting dependency hints and component references from inline comments and docstrings | Shared `comments` package with multi-syntax scanner, tiered confidence, known-component boosting; replaces per-parser comment hint regexes |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/url` | stdlib | URL parsing for connection strings | Already used by all three parsers; handles postgres://, redis://, mongodb://, http:// schemes |
| `regexp` | stdlib | DSN pattern matching, env var detection, comment patterns | Already used throughout; no external dependency needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `strings` | stdlib | Comment syntax stripping, line scanning | Comment prefix detection, whitespace handling |
| `path/filepath` | stdlib | .env file discovery | Only if .env scanning is included |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `net/url` for all URL parsing | `xo/dburl@v0.24.2` for DB scheme aliases | xo/dburl handles 50+ DB schemes (jdbc:, odbc:, etc.) but adds a dependency; net/url handles the common cases (postgres://, redis://, mongodb://) without any dependency. Recommend starting with net/url, noting xo/dburl as a v2.1 enhancement if exotic schemes are needed. |
| Custom env var regex | `os.ExpandEnv` | os.ExpandEnv actually resolves env vars at runtime; we want to detect references statically. Custom regex is correct here. |

## Architecture Patterns

### Recommended Project Structure
```
internal/code/
  connstring/
    connstring.go          # ParseConnectionString, ParseEnvVarRef
    connstring_test.go
  comments/
    comments.go            # AnalyzeComments, CommentSignal
    comments_test.go
  goparser/parser.go       # Modified: call connstring + comments utilities
  pyparser/parser.go       # Modified: call connstring + comments utilities
  jsparser/parser.go       # Modified: call connstring + comments utilities
```

### Pattern 1: Shared Utility Callable from Parsers

**What:** Connection string and comment analysis are shared packages imported by all three language parsers, not embedded in each parser.

**When to use:** When functionality is language-agnostic and currently duplicated across parsers.

**Integration approach:**

For connection strings, each parser already has a `extractTarget` / `extractURLHost` function that attempts URL parsing on call arguments. The new `connstring.Parse(raw string) (target string, targetType string, confidence float64, ok bool)` function replaces these inline URL parse attempts. Each parser calls `connstring.Parse()` on string literal arguments it already extracts, getting richer target resolution (URL hostnames, DSN parsing, env var reference names).

For comments, the existing per-parser comment scanning loops are replaced with a call to `comments.Analyze(lines []string, syntax CommentSyntax, knownComponents []string) []CodeSignal`. Each parser passes its file content and the appropriate comment syntax enum. The comment analyzer returns signals that the parser appends to its own signal list.

### Pattern 2: Known Component Boosting for Comments

**What:** The comment analyzer accepts an optional list of known component names (from the graph built so far or from other signals). When a comment mentions a known component, it gets confidence 0.5 instead of 0.4.

**When to use:** Per CONTEXT.md decision on tiered confidence.

**How the known component list reaches the comment analyzer:**

Option A (recommended): Extend `LanguageParser.ParseFile` to accept an optional context parameter, or add a `SetContext(ctx AnalysisContext)` method. The `AnalysisContext` carries known component names collected from a first pass or from the existing graph.

Option B: Two-pass analysis -- first pass collects all signals without boosting, then a post-pass boosts any comment_hint whose TargetComponent matches a signal from code detection. This avoids changing the LanguageParser interface.

**Recommendation:** Option B (two-pass) is simpler and doesn't change the established LanguageParser interface. The boost pass can live in `RunCodeAnalysis` or in a new `BoostKnownComponents(signals []CodeSignal) []CodeSignal` function in the `code` package.

### Pattern 3: Connection String Scheme-to-Type Mapping

**What:** Map URL schemes to target types automatically: `postgres://` -> database, `redis://` -> cache, `amqp://` -> message-broker, `http://` / `https://` -> service, `mongodb://` -> database, `mysql://` -> database, `nats://` -> message-broker.

**When to use:** Whenever a connection string is parsed, the scheme provides target type information that the parser alone doesn't have (e.g., a generic `sql.Open(driver, dsn)` call knows it's a DB, but a comment containing `redis://cache:6379` can infer cache type from the scheme).

### Anti-Patterns to Avoid
- **Duplicating URL parsing in each parser:** Currently each parser has its own `extractURLHost`. Consolidate into `connstring` package.
- **Changing LanguageParser interface for known-component awareness:** Adding parameters to `ParseFile` changes a well-established contract. Use a post-processing pass instead.
- **Over-extracting from comments:** Matching every noun in a comment as a potential component creates noise. Stick to explicit patterns ("Calls X", "Depends on X", "Uses X", "Connects to X") and URL patterns. Ambiguous patterns (TODO/FIXME with names) get 0.3 confidence as specified.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL parsing | Custom URL regex | `net/url.Parse` | Handles scheme, userinfo, host, port, path, query correctly; already battle-tested |
| Host:port splitting | Custom string split | Existing `extractURLHost` pattern (already in pyparser) | Handles edge cases like IPv6 brackets |
| Comment syntax stripping | Per-language comment removal | Single multi-syntax scanner with comment syntax enum | Three parsers already have slight variations; consolidating prevents drift |

**Key insight:** The hard part of connection string parsing is not the parsing itself (net/url handles URLs, regex handles DSNs) -- it's correctly mapping parsed results to component names and types. The scheme-to-type mapping table and hostname extraction logic are where the value is.

## Common Pitfalls

### Pitfall 1: Env Var References Without Resolution
**What goes wrong:** Detecting `os.Getenv("DATABASE_URL")` or `$DATABASE_URL` but having no value to parse, resulting in a signal with target "DATABASE_URL" instead of a meaningful component name.
**Why it happens:** Static analysis can't resolve runtime environment variables.
**How to avoid:** For env var references, emit a signal with `DetectionKind: "env_var_ref"` (or a sub-kind of `conn_string`), target = the env var name (e.g., "DATABASE_URL"), and a slightly lower confidence (0.7 instead of 0.85) since the actual target is unknown. The env var name itself is still useful for graph construction -- an AI agent can correlate "DATABASE_URL" references across services.
**Warning signs:** Tests that expect hostname extraction from env var references will fail. Accept that env vars produce env-var-name targets, not resolved hostnames.

### Pitfall 2: False Positives from URL-Like Strings in Comments
**What goes wrong:** Matching `http://example.com` or `https://docs.python.org` in comments as dependency signals.
**Why it happens:** Documentation URLs, example URLs, and reference links are common in comments.
**How to avoid:** Filter out known documentation/example domains: `example.com`, `example.org`, `localhost`, `127.0.0.1`, common doc sites. For comment-sourced URLs, use the lower comment confidence (0.4-0.5) rather than the connection string confidence (0.85).
**Warning signs:** Many signals pointing to `example.com` or `docs.python.org`.

### Pitfall 3: DSN Format Ambiguity
**What goes wrong:** Trying to parse DSN strings like `user:pass@tcp(host:3306)/dbname` (MySQL DSN) or `host=db port=5432 dbname=app` (PostgreSQL key-value DSN) with `net/url`.
**Why it happens:** DSN formats vary wildly between database drivers and don't follow URL conventions.
**How to avoid:** After `net/url.Parse` fails or returns no hostname, fall back to targeted regex patterns for common DSN formats: MySQL DSN (`@tcp(host:port)/`), PostgreSQL key-value (`host=X`), and generic `host:port/dbname` patterns. Keep the number of DSN regexes small (3-5) and document which formats are supported.
**Warning signs:** `net/url.Parse` succeeds but returns empty hostname on DSN strings.

### Pitfall 4: Comment Pattern Overlap with Existing Parser Comment Hints
**What goes wrong:** Both the existing per-parser comment hint regex AND the new shared comment analyzer match the same comment, producing duplicate signals.
**Why it happens:** The Go parser's `commentHintPattern` and Python/JS equivalents already match `// Calls X` patterns.
**How to avoid:** When integrating the shared comment analyzer, remove the existing per-parser comment hint scanning code. The shared analyzer completely replaces it. This is a key migration step in the plan.
**Warning signs:** Doubled comment_hint signals in test output.

### Pitfall 5: Docstring Detection Scope Creep
**What goes wrong:** Attempting to parse docstring content with NLP-like intelligence to extract component names.
**Why it happens:** Docstrings contain natural language descriptions that look like they should be analyzable.
**How to avoid:** Treat docstrings as "just more comments" -- apply the same regex patterns (Calls X, Depends on X, URL references) to docstring content. Don't try to understand sentence structure. The patterns are the same regardless of whether text is in `//`, `#`, or `"""`.
**Warning signs:** Complex docstring-specific parsing code that doesn't reuse the standard comment patterns.

## Code Examples

### Connection String Parser Core

```go
// internal/code/connstring/connstring.go

package connstring

import (
    "net/url"
    "regexp"
    "strings"
)

// Result holds the parsed connection string information.
type Result struct {
    Host       string  // Extracted hostname or env var name
    TargetType string  // database, cache, message-broker, service, unknown
    Confidence float64 // Detection confidence
    Kind       string  // conn_string or env_var_ref
}

// schemeTypes maps URL schemes to target types.
var schemeTypes = map[string]string{
    "postgres":  "database",
    "postgresql":"database",
    "mysql":     "database",
    "mongodb":   "database",
    "mongodb+srv":"database",
    "redis":     "database",  // Note: will be "cache" -- corrected
    "rediss":    "cache",
    "amqp":      "message-broker",
    "amqps":     "message-broker",
    "nats":      "message-broker",
    "http":      "service",
    "https":     "service",
}

// Parse attempts to extract a target component from a connection string.
// Returns ok=false if the string is not a recognizable connection string.
func Parse(raw string) (Result, bool) {
    // Try URL format first
    u, err := url.Parse(raw)
    if err == nil && u.Hostname() != "" {
        targetType := schemeTypes[u.Scheme]
        if targetType == "" {
            targetType = "unknown"
        }
        return Result{
            Host:       u.Hostname(),
            TargetType: targetType,
            Confidence: 0.85,
            Kind:       "conn_string",
        }, true
    }

    // Try DSN formats (MySQL, PostgreSQL key-value, etc.)
    if r, ok := parseDSN(raw); ok {
        return r, true
    }

    // Try host:port format
    if r, ok := parseHostPort(raw); ok {
        return r, true
    }

    return Result{}, false
}
```

### Environment Variable Reference Detection

```go
// Patterns for env var references across languages
var envVarPatterns = []*regexp.Regexp{
    // Go: os.Getenv("VAR"), os.LookupEnv("VAR")
    regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\s*\(\s*["'](\w+)["']\s*\)`),
    // Python: os.environ["VAR"], os.environ.get("VAR"), os.getenv("VAR")
    regexp.MustCompile(`os\.(?:environ\s*\[\s*["'](\w+)["']\s*\]|environ\.get\s*\(\s*["'](\w+)["']|getenv\s*\(\s*["'](\w+)["'])`),
    // JS: process.env.VAR, process.env["VAR"]
    regexp.MustCompile(`process\.env\.(\w+)|process\.env\[["'](\w+)["']\]`),
    // Shell-style: $VAR, ${VAR} (in string literals)
    regexp.MustCompile(`\$\{?(\w+)\}?`),
}

// IsConnectionEnvVar returns true if the env var name suggests a connection string.
func IsConnectionEnvVar(name string) bool {
    connSuffixes := []string{"_URL", "_DSN", "_URI", "_HOST", "_ADDR", "_ENDPOINT", "_CONNECTION"}
    upper := strings.ToUpper(name)
    for _, suffix := range connSuffixes {
        if strings.HasSuffix(upper, suffix) {
            return true
        }
    }
    connPrefixes := []string{"DATABASE_", "DB_", "REDIS_", "MONGO_", "RABBIT_", "KAFKA_", "NATS_", "AMQP_"}
    for _, prefix := range connPrefixes {
        if strings.HasPrefix(upper, prefix) {
            return true
        }
    }
    return false
}
```

### Comment Analyzer Core

```go
// internal/code/comments/comments.go

package comments

import (
    "regexp"
    "strings"

    "github.com/graphmd/graphmd/internal/code"
    "github.com/graphmd/graphmd/internal/code/connstring"
)

// CommentSyntax identifies which comment syntaxes to recognize.
type CommentSyntax int

const (
    SyntaxGo         CommentSyntax = iota // // and /* */
    SyntaxPython                          // # and """ and '''
    SyntaxJavaScript                      // //, /* */, and /** */
)

// explicit dependency patterns (language-agnostic, applied to comment text)
var explicitPatterns = regexp.MustCompile(
    `(?i)(?:calls|depends on|uses|connects to|talks to|sends to|reads from|writes to)\s+` +
    `([\w][\w.-]*)`,  // component-like name: alphanumeric with hyphens and dots
)

// TODO/FIXME patterns with component-like names
var todoPattern = regexp.MustCompile(
    `(?i)(?:TODO|FIXME|HACK|XXX)[:\s]+.*?([\w][\w.-]*(?:-(?:service|api|db|cache|queue|broker|cluster))[\w.-]*)`,
)

// Analyze scans lines for comment-based dependency signals.
// knownComponents is used for confidence boosting (0.5 for known, 0.4 for new explicit, 0.3 for ambiguous).
func Analyze(lines []string, syntax CommentSyntax, knownComponents map[string]bool) []code.CodeSignal {
    // Implementation: iterate lines, detect comment regions,
    // extract text, apply patterns, build signals with tiered confidence
}
```

### Integration Point: Replacing Per-Parser Comment Hints

```go
// In goparser/parser.go -- REMOVE existing comment scanning:
//   for _, cg := range f.Comments {
//       for _, c := range cg.List { ... commentHintPattern ... }
//   }
// REPLACE WITH:
commentSignals := comments.Analyze(lines, comments.SyntaxGo, nil)
signals = append(signals, commentSignals...)

// In pyparser/parser.go -- REMOVE existing:
//   if strings.HasPrefix(trimmed, "#") { ... commentHintRe ... }
// REPLACE WITH:
commentSignals := comments.Analyze(lines, comments.SyntaxPython, nil)
signals = append(signals, commentSignals...)

// In jsparser/parser.go -- REMOVE existing:
//   if matches := commentHintPattern.FindStringSubmatch(trimmed) ...
// REPLACE WITH:
commentSignals := comments.Analyze(lines, comments.SyntaxJavaScript, nil)
signals = append(signals, commentSignals...)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-parser URL extraction | Each parser has `extractURLHost` using `net/url` | Phase 9-10 (current) | Works but duplicated across 3 parsers; no DSN or env var support |
| Per-parser comment hint regex | Go: `commentHintPattern`, Py: `commentHintRe`, JS: `commentHintPattern` | Phase 9-10 (current) | Only matches 4 verbs (Calls, Depends on, Uses, Connects to); no docstrings, no TODO/FIXME, no URL-in-comments, no confidence tiers |
| Flat 0.4 confidence for all comment hints | Same across all parsers | Phase 9 decision | No distinction between known-component corroboration and new discovery |

## Open Questions

1. **Should .env files be scanned?**
   - What we know: .env files contain `KEY=value` pairs that often hold connection strings. Scanning them would provide high-confidence dependency signals.
   - What's unclear: .env files are in `.gitignore` by default and may not be present in the analyzed directory. Also, .env is not a "source code" file -- it's configuration.
   - Recommendation: Skip .env scanning for Phase 11. The env var *reference* detection (os.Getenv, process.env) is sufficient. .env scanning could be a Phase 12 or v2.1 enhancement. If included, it would be a separate utility in `connstring/` that walks for `.env` / `.env.example` files, not part of LanguageParser.

2. **How to handle multi-line docstrings?**
   - What we know: Python `"""..."""` and JS `/** ... */` docstrings can span many lines. The current line-by-line scanning approach would need to track state across lines (already done for JS block comments in the JS parser).
   - What's unclear: Whether the performance cost of stateful scanning is worth the signal quality from docstrings.
   - Recommendation: Use the same block-comment state tracking already implemented in `jsparser/parser.go` (lines 89-129). Apply it generically in the comment analyzer with syntax-aware open/close markers. This is straightforward -- the pattern exists.

3. **Detection kind for connection string signals**
   - What we know: Current signals use kinds like `http_call`, `db_connection`, `cache_client`, `queue_producer`. CONTEXT.md specifies `comment_hint` for comment-derived signals.
   - What's unclear: What detection kind should connection-string-derived signals use? Options: reuse existing kinds (db_connection, cache_client) since the connection string reveals the type, or use a new `conn_string` kind.
   - Recommendation: Use existing kinds (`db_connection`, `cache_client`, etc.) when the scheme maps to a known type. The connection string parser is a *target extraction* utility, not a new detection kind -- it enhances the existing detection kinds with better target resolution. For env var references where the actual type is unknown, use `env_var_ref` as a new kind.

## Sources

### Primary (HIGH confidence)
- Existing codebase: `internal/code/goparser/parser.go` (lines 104-123) -- current Go comment hint implementation
- Existing codebase: `internal/code/pyparser/parser.go` (lines 69-84) -- current Python comment hint implementation
- Existing codebase: `internal/code/jsparser/parser.go` (lines 89-148) -- current JS block comment + hint implementation
- Existing codebase: `internal/code/pyparser/parser.go` (lines 316-342) -- Python `extractURLHost` with host:port support
- Existing codebase: `internal/code/goparser/parser.go` (lines 219-243) -- Go `extractURLHost` using net/url
- Go stdlib `net/url` package -- URL parsing capabilities verified against current usage in all three parsers

### Secondary (MEDIUM confidence)
- CONTEXT.md user decisions -- confidence tiers, pattern types, architecture constraints
- Phase 9-10 summaries -- established patterns for parser structure and signal emission

### Tertiary (LOW confidence)
- DSN format coverage: The MySQL DSN format (`user:pass@tcp(host:port)/db`) and PostgreSQL key-value format (`host=X port=Y dbname=Z`) are based on driver documentation knowledge. Exact regex patterns should be validated against real-world connection strings during implementation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all stdlib, no new dependencies needed
- Architecture: HIGH -- shared utility pattern is well-established; existing duplicated code clearly shows the consolidation path
- Pitfalls: HIGH -- pitfalls identified from direct analysis of existing code (duplicate signals, env var resolution limits, URL false positives)
- Code examples: MEDIUM -- examples are illustrative of the approach; exact regex patterns need implementation-time validation

**Research date:** 2026-04-01
**Valid until:** 2026-05-01 (stable domain, no external dependency changes expected)
