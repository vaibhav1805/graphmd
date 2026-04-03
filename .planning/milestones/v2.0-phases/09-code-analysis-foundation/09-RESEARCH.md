# Phase 9: Code Analysis Foundation - Research

**Researched:** 2026-03-30
**Domain:** Go source code static analysis for infrastructure dependency detection
**Confidence:** HIGH

## Summary

Phase 9 adds a Go source code parser that detects infrastructure dependencies (HTTP calls, database connections, message queue producers/consumers, cache clients) using the Go standard library's `go/ast` and `go/parser` packages. This is a well-understood domain: Go's AST tooling is mature, zero-dependency, and used by every major Go analysis tool (golangci-lint, staticcheck, go vet). The core task is writing AST visitor functions that match specific `*ast.CallExpr` patterns (like `sql.Open`, `http.Get`, `redis.NewClient`) and emit `CodeSignal` structs.

The phase also establishes the `LanguageParser` interface and `CodeSignal` type that Python and JS/TS parsers will implement in Phase 10. This abstraction must be language-agnostic: no Go-specific concepts leak into the interface. The Go parser is the proof-of-concept that validates the interface design before committing two more parsers to it.

Source component inference (answering "which service does this code belong to?") uses `golang.org/x/mod/modfile` to parse `go.mod` and extract the module path. This is a single function call (`modfile.ModulePath(data)`) with zero configuration required. Fallback: directory name when no `go.mod` exists.

**Primary recommendation:** Use `go/parser.ParseFile` with full AST (not `ImportsOnly`) so we can detect function calls, not just imports. Match `*ast.CallExpr` → `*ast.SelectorExpr` patterns against a curated table of known infrastructure functions. Emit one `CodeSignal` per detection. Skip `*_test.go` files at the scanner level.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Confidence tiered by detection strength:**
  - Direct API call (e.g., `http.Get("http://payment-api/...")`) → 0.9
  - Connection string usage (e.g., `sql.Open("postgres", connStr)`) → 0.85
  - Comment hint (e.g., `// Calls payment-api`) → 0.4
  - Note: import-only detections (e.g., `import "database/sql"` without usage) are NOT emitted as signals
- **Require usage evidence:** Import-only detections excluded. A signal is only emitted when actual function calls, connection creation, or client initialization is found
- **Test files excluded:** Files matching `*_test.go` (and equivalent patterns for Python/JS in Phase 10) are skipped. Test dependencies are not production dependencies
- **Target name extraction:** Parser should extract specific queue/cache names (e.g., topic name, queue name) when possible. Falls back to generic type name (e.g., "kafka", "redis") when specific name can't be determined
- **CodeSignal fields:** source_file, line_number, target_component, target_type, detection_kind, evidence, language, confidence
- **Source component inference:** Infer from `go.mod` module name or directory name. No explicit mapping config required
- **Opt-in flag:** `--analyze-code` available on export, crawl, and index commands
- **Default behavior unchanged:** Without the flag, all three commands stay markdown-only (preserves v1 performance)
- **All detected languages:** When `--analyze-code` is set, run all available parsers on files found. No per-language filtering
- **Consistent across commands:** Same `--analyze-code` flag behavior on export, crawl, and index

### Claude's Discretion
- LanguageParser interface design (method signatures, return types)
- New `internal/code/` package structure
- Go AST traversal patterns for detecting HTTP/DB/queue/cache usage
- How go.mod module name is resolved for source component inference
- Detection pattern specifics (which function calls match which detection kinds)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CODE-01 | Go language parser detecting imports, HTTP client calls, database connections, message queue producers/consumers, and cache client usage | Full research coverage: AST patterns for each detection category documented below, standard library sufficient, detection pattern table provided |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go/ast` | stdlib (Go 1.24) | AST node types and traversal | Standard Go AST representation; used by golangci-lint, staticcheck, every Go analysis tool |
| `go/parser` | stdlib (Go 1.24) | Parse Go source to AST | Standard Go parser; `ParseFile` produces `*ast.File` for inspection |
| `go/token` | stdlib (Go 1.24) | Source positions (file:line) | Required for `token.FileSet` to map AST nodes to line numbers |
| `golang.org/x/mod/modfile` | latest | Parse go.mod for module path | Official x/ package for go.mod manipulation; `ModulePath()` extracts module name |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `path/filepath` | stdlib | File walking for code files | Extend scanner to walk `.go` files alongside `.md` |
| `strings` | stdlib | Import path and name matching | Match import paths against known infrastructure packages |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go/parser` (per-file) | `go/packages` (module-aware) | `go/packages` resolves types across packages but requires the target project to be buildable (needs `go` binary, module download). Per-file parsing with `go/parser` works on any source tree without build setup. For dependency detection, syntactic matching is sufficient. |
| `golang.org/x/mod/modfile` | Manual regex on go.mod | modfile handles edge cases (comments, retract directives, formatting). Regex would be fragile. modfile is lightweight (~2KB import). |

**Installation:**
```bash
go get golang.org/x/mod@latest
```

Note: `go/ast`, `go/parser`, `go/token` are stdlib — no installation needed.

## Architecture Patterns

### Recommended Project Structure
```
internal/
  code/
    signal.go           # CodeSignal type definition
    analyzer.go         # CodeAnalyzer orchestrator + LanguageParser interface
    analyzer_test.go    # Integration tests for analyzer
    goparser/
      parser.go         # Go AST-based parser implementing LanguageParser
      parser_test.go    # Unit tests with Go source fixtures
      patterns.go       # Detection pattern table (package → function → detection_kind)
```

### Pattern 1: AST Visitor for Function Call Detection
**What:** Use `ast.Inspect` to walk the AST of each Go file, matching `*ast.CallExpr` nodes whose `Fun` field is a `*ast.SelectorExpr` matching known infrastructure packages.
**When to use:** Every Go file analyzed.
**Source:** [Go AST function call detection gist](https://gist.github.com/cryptix/d1b129361cea51a59af2), [Basic AST Traversal](https://www.zupzup.org/go-ast-traversal/index.html)

```go
// Core detection loop inside GoParser.ParseFile
ast.Inspect(file, func(n ast.Node) bool {
    call, ok := n.(*ast.CallExpr)
    if !ok {
        return true
    }
    sel, ok := call.Fun.(*ast.SelectorExpr)
    if !ok {
        return true
    }
    ident, ok := sel.X.(*ast.Ident)
    if !ok {
        return true
    }
    // ident.Name = package alias (e.g., "http", "sql", "redis")
    // sel.Sel.Name = function name (e.g., "Get", "Open", "NewClient")
    // Match against detection pattern table
    if pattern, found := patterns[ident.Name+"."+sel.Sel.Name]; found {
        pos := fset.Position(call.Pos())
        signals = append(signals, code.CodeSignal{
            SourceFile:      filePath,
            LineNumber:      pos.Line,
            TargetComponent: extractTarget(call, pattern),
            TargetType:      pattern.TargetType,
            DetectionKind:   pattern.Kind,
            Evidence:        sourceSnippet(content, pos.Line),
            Language:        "go",
            Confidence:      pattern.Confidence,
        })
    }
    return true
})
```

### Pattern 2: Import-to-Alias Resolution
**What:** Before walking function calls, build a map of import alias → import path from `file.Imports`. This handles renamed imports (e.g., `pg "database/sql"`).
**When to use:** Every parsed Go file, before function call detection.

```go
// Build alias map: local name → full import path
aliases := make(map[string]string)
for _, imp := range file.Imports {
    path := strings.Trim(imp.Path.Value, `"`)
    if imp.Name != nil {
        aliases[imp.Name.Name] = path
    } else {
        // Default: last element of import path
        parts := strings.Split(path, "/")
        aliases[parts[len(parts)-1]] = path
    }
}
```

### Pattern 3: Detection Pattern Table (Data-Driven)
**What:** A static table mapping (package, function) → (detection_kind, target_type, confidence) rather than hard-coding detection logic per function.
**When to use:** Core detection configuration.

```go
type DetectionPattern struct {
    ImportPath    string          // e.g., "net/http"
    Function      string          // e.g., "Get"
    Kind          string          // e.g., "http_call"
    TargetType    string          // e.g., "service"
    Confidence    float64         // e.g., 0.9
    ExtractTarget TargetExtractor // function to pull target name from args
}

var defaultPatterns = []DetectionPattern{
    // HTTP clients
    {ImportPath: "net/http", Function: "Get", Kind: "http_call", TargetType: "service", Confidence: 0.9},
    {ImportPath: "net/http", Function: "Post", Kind: "http_call", TargetType: "service", Confidence: 0.9},
    {ImportPath: "net/http", Function: "NewRequest", Kind: "http_call", TargetType: "service", Confidence: 0.9},

    // Database connections
    {ImportPath: "database/sql", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.85},
    {ImportPath: "github.com/jmoiron/sqlx", Function: "Connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85},
    {ImportPath: "github.com/jmoiron/sqlx", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.85},
    {ImportPath: "gorm.io/gorm", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.85},
    {ImportPath: "go.mongodb.org/mongo-driver/mongo", Function: "Connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85},

    // Cache clients
    {ImportPath: "github.com/redis/go-redis/v9", Function: "NewClient", Kind: "cache_client", TargetType: "cache", Confidence: 0.85},
    {ImportPath: "github.com/go-redis/redis/v8", Function: "NewClient", Kind: "cache_client", TargetType: "cache", Confidence: 0.85},
    {ImportPath: "github.com/bradfitz/gomemcache/memcache", Function: "New", Kind: "cache_client", TargetType: "cache", Confidence: 0.85},

    // Message queues
    {ImportPath: "github.com/IBM/sarama", Function: "NewSyncProducer", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85},
    {ImportPath: "github.com/IBM/sarama", Function: "NewAsyncProducer", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85},
    {ImportPath: "github.com/IBM/sarama", Function: "NewConsumer", Kind: "queue_consumer", TargetType: "message-broker", Confidence: 0.85},
    {ImportPath: "github.com/IBM/sarama", Function: "NewConsumerGroup", Kind: "queue_consumer", TargetType: "message-broker", Confidence: 0.85},
    {ImportPath: "github.com/rabbitmq/amqp091-go", Function: "Dial", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85},
    {ImportPath: "github.com/nats-io/nats.go", Function: "Connect", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85},
    {ImportPath: "github.com/aws/aws-sdk-go-v2/service/sqs", Function: "NewFromConfig", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85},
}
```

### Pattern 4: Source Component Inference from go.mod
**What:** Walk up from the analyzed file's directory to find `go.mod`, parse its module name, use the last path element as the source component name.
**When to use:** Once per analysis run, to set the `source_component` on all emitted signals.

```go
func InferSourceComponent(dir string) string {
    // Walk up to find go.mod
    for d := dir; d != "/" && d != "."; d = filepath.Dir(d) {
        data, err := os.ReadFile(filepath.Join(d, "go.mod"))
        if err != nil {
            continue
        }
        modPath := modfile.ModulePath(data)
        if modPath != "" {
            parts := strings.Split(modPath, "/")
            return parts[len(parts)-1] // e.g., "graphmd" from "github.com/graphmd/graphmd"
        }
    }
    return filepath.Base(dir) // fallback to directory name
}
```

### Pattern 5: Target Name Extraction from Function Arguments
**What:** Extract the actual target (URL, connection string, queue name) from the `*ast.CallExpr.Args` when the argument is a string literal.
**When to use:** After matching a detection pattern, to get a specific target name instead of a generic one.

```go
func extractURLHost(call *ast.CallExpr, argIndex int) string {
    if len(call.Args) <= argIndex {
        return ""
    }
    lit, ok := call.Args[argIndex].(*ast.BasicLit)
    if !ok || lit.Kind != token.STRING {
        return "" // variable reference — can't resolve statically
    }
    urlStr := strings.Trim(lit.Value, `"` + "`")
    u, err := url.Parse(urlStr)
    if err != nil || u.Host == "" {
        return ""
    }
    return u.Hostname() // e.g., "payment-api" from "http://payment-api:8080/pay"
}
```

### Anti-Patterns to Avoid
- **Import-only detection:** The user explicitly decided imports alone are NOT signals. Only function calls, connection creation, or client initialization emit signals.
- **Type resolution dependency:** Do NOT use `go/types` or `go/packages` for Phase 9. It would require the target project to be buildable. Syntactic matching of `package.Function` via import alias resolution is sufficient for dependency detection.
- **Monolithic parser:** Don't put all detection logic in one giant function. Use the pattern table to drive detection; each pattern entry is declarative data, not imperative code.
- **Leaking Go concepts into LanguageParser:** The interface must work for Python/JS too. `ParseFile(path string, content []byte) ([]CodeSignal, error)` — no `*ast.File`, no `token.FileSet` in the interface.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| go.mod parsing | Regex on `module` line | `golang.org/x/mod/modfile.ModulePath()` | Handles comments, formatting, edge cases |
| Go source parsing | Custom tokenizer/regex | `go/parser.ParseFile()` | Handles all Go syntax, comments, string literals correctly |
| Line number mapping | Manual byte-offset counting | `token.FileSet.Position()` | Standard approach, handles multi-byte chars, tabs |
| URL parsing from string literals | Custom regex | `net/url.Parse()` | Standard URL parser, handles edge cases |

**Key insight:** Go's stdlib gives us everything we need for Go source analysis. The entire Go parser is free (zero dependencies, zero build complexity). The complexity is in the detection pattern table, not the parsing infrastructure.

## Common Pitfalls

### Pitfall 1: Matching Package Alias Instead of Import Path
**What goes wrong:** Code matches `sql.Open` by checking if the local name is "sql", but the user might have `import pg "database/sql"` — so the call is `pg.Open`, not `sql.Open`.
**Why it happens:** Shortcutting by matching local names without building the import alias map.
**How to avoid:** Always build `alias → import path` map from `file.Imports` first. Match detection patterns against the resolved import path, not the local alias name.
**Warning signs:** Test case with renamed imports fails to detect the call.

### Pitfall 2: ParseFile with ImportsOnly Mode
**What goes wrong:** Using `parser.ImportsOnly` flag only parses import declarations, not function bodies. All function call detection silently produces zero results.
**Why it happens:** ImportsOnly is faster and seems sufficient if you think "we need imports." But the user explicitly requires usage evidence, not import-only detection.
**How to avoid:** Use `parser.ParseComments` mode (or 0 for default). This parses the full file including function bodies and comments.
**Warning signs:** Tests pass for import detection but fail for function call detection.

### Pitfall 3: String Literal Extraction Across Backticks and Quotes
**What goes wrong:** `ast.BasicLit.Value` includes the surrounding quotes. Forgetting to strip them produces URLs like `"http://..."` instead of `http://...`.
**Why it happens:** The AST preserves the literal source representation, including quote characters and backtick raw strings.
**How to avoid:** Always `strings.Trim(lit.Value, "\"` + "`")` before parsing string values. Handle both regular strings and raw strings.
**Warning signs:** URL parsing fails because `url.Parse` gets confused by leading quote characters.

### Pitfall 4: Test File Inclusion
**What goes wrong:** Analyzing `*_test.go` files produces signals for test infrastructure (test databases, mock HTTP servers) that aren't production dependencies.
**Why it happens:** The scanner walks all `.go` files without filtering.
**How to avoid:** Skip files matching `*_test.go` before parsing. The user explicitly decided test files are excluded.
**Warning signs:** Graph shows dependencies on test fixtures, mock servers, or test databases.

### Pitfall 5: Variable Arguments Can't Be Resolved
**What goes wrong:** `sql.Open("postgres", connectionString)` where `connectionString` is a variable — the AST only gives us the variable name, not its value.
**Why it happens:** Static analysis without type resolution can't follow variable assignments across scopes.
**How to avoid:** When the argument is not a `*ast.BasicLit`, fall back to the generic target name (e.g., "postgres" from the driver name argument, or "database" from the pattern's TargetType). Don't try to resolve variables — that requires full type checking.
**Warning signs:** Attempts to resolve variable values lead to complex scope-tracking code with diminishing returns.

## Code Examples

### Complete GoParser.ParseFile Implementation Pattern
```go
func (p *GoParser) ParseFile(filePath string, content []byte) ([]code.CodeSignal, error) {
    // Skip test files
    if strings.HasSuffix(filePath, "_test.go") {
        return nil, nil
    }

    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
    if err != nil {
        return nil, fmt.Errorf("parse %s: %w", filePath, err)
    }

    // Step 1: Build import alias → import path map
    aliases := buildAliasMap(f.Imports)

    // Step 2: Walk AST for function calls matching detection patterns
    var signals []code.CodeSignal
    ast.Inspect(f, func(n ast.Node) bool {
        call, ok := n.(*ast.CallExpr)
        if !ok {
            return true
        }
        sel, ok := call.Fun.(*ast.SelectorExpr)
        if !ok {
            return true
        }
        ident, ok := sel.X.(*ast.Ident)
        if !ok {
            return true
        }

        // Resolve alias to full import path
        importPath, ok := aliases[ident.Name]
        if !ok {
            return true
        }

        // Check against detection pattern table
        key := importPath + "." + sel.Sel.Name
        pattern, ok := p.patterns[key]
        if !ok {
            return true
        }

        pos := fset.Position(call.Pos())
        target := pattern.ExtractTarget(call)
        if target == "" {
            target = pattern.DefaultTarget
        }

        signals = append(signals, code.CodeSignal{
            SourceFile:      filePath,
            LineNumber:      pos.Line,
            TargetComponent: target,
            TargetType:      pattern.TargetType,
            DetectionKind:   pattern.Kind,
            Evidence:        evidenceSnippet(content, pos.Line),
            Language:        "go",
            Confidence:      pattern.Confidence,
        })
        return true
    })

    // Step 3: Scan comments for dependency hints
    for _, cg := range f.Comments {
        for _, c := range cg.List {
            if hint := extractCommentHint(c.Text); hint != nil {
                pos := fset.Position(c.Pos())
                signals = append(signals, code.CodeSignal{
                    SourceFile:      filePath,
                    LineNumber:      pos.Line,
                    TargetComponent: hint.Target,
                    TargetType:      hint.Type,
                    DetectionKind:   "comment_hint",
                    Evidence:        c.Text,
                    Language:        "go",
                    Confidence:      0.4,
                })
            }
        }
    }

    return signals, nil
}
```

### LanguageParser Interface
```go
// internal/code/analyzer.go

// CodeSignal represents a dependency detected from source code.
type CodeSignal struct {
    SourceFile      string  `json:"source_file"`
    LineNumber      int     `json:"line_number"`
    TargetComponent string  `json:"target_component"`
    TargetType      string  `json:"target_type"`
    DetectionKind   string  `json:"detection_kind"`
    Evidence        string  `json:"evidence"`
    Language        string  `json:"language"`
    Confidence      float64 `json:"confidence"`
}

// LanguageParser extracts dependency signals from source files of a specific language.
type LanguageParser interface {
    // Name returns the parser's language name (e.g., "go", "python", "javascript").
    Name() string

    // Extensions returns file extensions this parser handles (e.g., [".go"]).
    Extensions() []string

    // ParseFile analyzes a single file and returns dependency signals.
    // content is the raw file bytes; filePath is relative to the project root.
    ParseFile(filePath string, content []byte) ([]CodeSignal, error)
}

// CodeAnalyzer orchestrates all registered language parsers.
type CodeAnalyzer struct {
    parsers    map[string]LanguageParser // extension → parser
    sourceComp string                    // inferred source component name
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `go/ast` with `Scope`/`Obj` resolution | `go/ast` with `SkipObjectResolution` | Go 1.22+ | Object resolution in `go/ast` is deprecated; use `go/types` if needed. For our use case (syntactic matching), skip it entirely for better performance. |
| `go/parser` per-file | `go/packages` for module-aware loading | Go 1.11+ (modules) | `go/packages` is recommended for tools needing type info. We don't need type info — `go/parser` per-file is simpler and works on any source tree. |
| Regex on go.mod | `golang.org/x/mod/modfile` | 2019+ | Official package for go.mod manipulation; use `ModulePath()` for simple extraction. |

**Deprecated/outdated:**
- `go/ast` object resolution (`Scope`, `Obj` fields): Deprecated since Go 1.22. Not needed for our pattern matching approach.

## Open Questions

1. **Comment hint patterns: what syntax to recognize?**
   - What we know: Confidence for comment hints is 0.4 (lowest tier). Need to detect patterns like `// Calls payment-api`, `// Depends on primary-db`.
   - What's unclear: Exact regex patterns for comment hints. No standard exists.
   - Recommendation: Start with `// (Calls|Depends on|Uses|Connects to) <name>` patterns. Keep it simple; this is the lowest-confidence detection.

2. **How to handle method calls on struct fields?**
   - What we know: `client.Get(url)` where `client` is an `*http.Client` field. The AST shows a `SelectorExpr` on a variable, not a package.
   - What's unclear: Whether to detect `client.Get()` / `client.Do()` patterns in addition to `http.Get()`.
   - Recommendation: Phase 9 focuses on package-level function calls (e.g., `http.Get`, `sql.Open`). Method calls on struct instances (e.g., `client.Do()`) are a Phase 10+ enhancement. This avoids needing type resolution.

3. **Multiple go.mod files in a monorepo?**
   - What we know: `InferSourceComponent` walks up from each file to find `go.mod`. In a monorepo, different subdirectories may have different `go.mod` files.
   - What's unclear: Whether to support per-directory source component inference or one global source.
   - Recommendation: Walk up per-file. Each file gets the nearest `go.mod`'s module name. Cache the result per directory to avoid re-parsing.

## Sources

### Primary (HIGH confidence)
- [go/ast package docs](https://pkg.go.dev/go/ast) — AST node types, Inspect function
- [go/parser package docs](https://pkg.go.dev/go/parser) — ParseFile function, parser modes
- [go/token package docs](https://pkg.go.dev/go/token) — FileSet, Position for line numbers
- [golang.org/x/mod/modfile](https://pkg.go.dev/golang.org/x/mod/modfile) — Parse, ModulePath functions for go.mod parsing
- Existing codebase (`internal/knowledge/*.go`) — Direct code reading of scanner, edge, discovery, aggregator patterns

### Secondary (MEDIUM confidence)
- [Go AST function call detection gist](https://gist.github.com/cryptix/d1b129361cea51a59af2) — isPkgDot pattern for SelectorExpr matching
- [Basic AST Traversal in Go](https://www.zupzup.org/go-ast-traversal/index.html) — ast.Inspect walkthrough
- [Ad-hoc static code analysis in Go](https://engineering.countingup.com/custom-go-vet/) — Custom go-vet for detecting database library misuse
- [Rewriting Go source code with AST tooling](https://eli.thegreenplace.net/2021/rewriting-go-source-code-with-ast-tooling/) — AST manipulation patterns
- [Take a walk the Go AST](https://nakabonne.dev/posts/take-a-walk-the-go-ast/) — AST traversal patterns

### Tertiary (LOW confidence)
- None — all findings verified against official docs or multiple sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — All stdlib except `golang.org/x/mod` which is official x/ ecosystem
- Architecture: HIGH — Based on direct analysis of existing codebase patterns (scanner, discovery pipeline, edge types, signal aggregation)
- Pitfalls: HIGH — All pitfalls derived from Go AST documentation and verified against known patterns

**Research date:** 2026-03-30
**Valid until:** 2026-05-30 (stable domain — Go AST API has been stable since Go 1.0)
