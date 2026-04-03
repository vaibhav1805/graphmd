---
status: diagnosed
trigger: "Investigate why `graphmd mcp` doesn't respond to an MCP initialize request piped via stdin."
created: 2026-04-03T00:00:00Z
updated: 2026-04-03T00:00:00Z
---

## Current Focus

hypothesis: stdin EOF from `echo` pipe arrives before or simultaneously with the initialize message being processed; the SDK's read goroutine sends EOF into the jsonrpc2 connection, causing it to tear down, which cancels the context used to write the response before the write completes
test: traced SDK source path from StdioTransport.Connect -> newIOConn -> readIncoming -> acceptRequest -> handleAsync -> processResult -> write
expecting: confirmed — stdin EOF causes readErr to be set, connection enters shuttingDown state, write of initialize response is rejected
next_action: report diagnosis

## Symptoms

expected: graphmd mcp responds to a piped initialize request with a valid JSON-RPC 2.0 response
actual: "MCP server error: context canceled" on stderr, nothing on stdout; python3 reports empty output
errors: "MCP server error: context canceled" (stderr)
reproduction: echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | graphmd mcp 2>/dev/null | head -1 | python3 -m json.tool
started: unknown / always broken with piped stdin

## Eliminated

- hypothesis: stdout guard (redirect os.Stdout to os.Stderr during setup) interferes with StdioTransport
  evidence: The stdout guard is restored to origStdout BEFORE server.Run is called (line 41 of server.go). StdioTransport.Connect calls os.Stdin and os.Stdout at Connect time (inside Run), so os.Stdout is already restored.
  timestamp: 2026-04-03

- hypothesis: The MCP SDK requires a notifications/initialized message after initialize before it can respond
  evidence: The sequence is: client sends initialize → server responds → client sends notifications/initialized. The server must respond to initialize first. This cannot be the cause of no response.
  timestamp: 2026-04-03

## Evidence

- timestamp: 2026-04-03
  checked: internal/mcp/server.go Run() function
  found: stdout guard redirects os.Stdout -> os.Stderr for tool registration, then restores before calling server.Run(ctx, &mcpsdk.StdioTransport{}). The signal-cancellable context is passed to Run.
  implication: Stdout guard is not the issue. Signal context is fine for interactive use but ALSO cancels when server.Run returns normally (defer stop()).

- timestamp: 2026-04-03
  checked: go-sdk/mcp/transport.go StdioTransport.Connect
  found: Directly captures os.Stdin and os.Stdout at connect time: newIOConn(rwc{os.Stdin, nopCloserWriter{os.Stdout}})
  implication: Uses the real stdin/stdout file descriptors directly.

- timestamp: 2026-04-03
  checked: go-sdk/mcp/transport.go newIOConn
  found: Spawns a goroutine that calls json.NewDecoder(rwc).Decode() in a loop. After each successful decode, it reads ONE trailing byte to check for \n or \r. If that trailing read gets io.EOF, it is treated as a non-error (only errors other than io.EOF become decode errors). The decoded message is sent to the `incoming` channel. If the NEXT loop iteration's Decode returns io.EOF, the error is forwarded into `incoming`.
  implication: After echo sends one JSON line, the pipe closes. The goroutine decodes the JSON successfully. The trailing-byte read returns (0, io.EOF) which is ignored. Then the next Decode returns io.EOF, which is sent to `incoming`. This EOF arrives VERY QUICKLY after the message.

- timestamp: 2026-04-03
  checked: go-sdk/internal/jsonrpc2/conn.go readIncoming and acceptRequest
  found: When EOF arrives from `incoming`, readIncoming sets s.readErr = io.EOF. The connection then enters shuttingDown state. If the initialize request was already accepted (acceptRequest called) but not yet processed by handleAsync, the incoming counter is > 0, so the connection waits. BUT if the response write happens via processResult -> write -> c.write, the write path calls shuttingDown(ErrServerClosing) first — if s.readErr is already set, shuttingDown returns a non-nil error wrapping ErrServerClosing and the write is blocked/rejected.
  implication: The race: EOF from the closed pipe arrives on the `incoming` channel. If it is consumed by the jsonrpc2 read loop BEFORE the initialize handler writes its response, the write is rejected with ErrServerClosing. The server.Run then returns ctx.Err() (context.Canceled from the signal context defer stop()) because ss.Wait() returns after the connection closes.

- timestamp: 2026-04-03
  checked: go-sdk/internal/jsonrpc2/conn.go write() method (line 783-818)
  found: write() calls shuttingDown(ErrServerClosing) before calling c.writer.Write(). If shuttingDown returns non-nil (because readErr is set), the write is abandoned immediately and returns ErrServerClosing.
  implication: The initialize response is silently dropped — nothing is written to stdout.

- timestamp: 2026-04-03
  checked: go-sdk/mcp/server.go Run() (line 935-962)
  found: Run() calls ss.Wait(). When the connection closes, Wait returns. Then Run returns ss.Wait()'s error. Back in cmdMCP, if Run returns non-nil, fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err) is printed. The error is context.Canceled because after ss closes, the ctx.Done() arm of the select fires (since the signal context's defer stop() is called, but actually server.Run returns the ssClosed error path, not the ctx.Done path). The actual error returned by Wait is the readErr from the connection which could be the EOF-shutdown error, but the conn.wait() function filters out io.EOF (line 523) and closeErr may be nil, so it returns nil. Then server.Run returns nil. But cmdMCP only prints on non-nil...
  implication: Need to reconsider. The "context canceled" error suggests the ctx.Done path fires in server.Run's select.

- timestamp: 2026-04-03
  checked: internal/mcp/server.go signal context and defer stop()
  found: signal.NotifyContext returns a context that cancels when SIGTERM/SIGINT is received OR when stop() is called. The defer stop() runs after server.Run returns. But server.Run itself blocks on select{ctx.Done, ssClosed}. When stdin closes (echo pipe ends), the connection tears down, ss.Wait() completes, ssClosed channel receives. This should take the ssClosed arm, not ctx.Done.
  implication: The "context canceled" error must come from somewhere else. Looking at conn.Wait() -> wait(true): it checks readErr (ignores io.EOF), writeErr, closeErr. The closeErr from ioConn.Close() calls rwc.Close() which closes os.Stdin (rc.Close()) and os.Stdout (wc.Close() via nopCloserWriter which returns nil). So closeErr is nil or stdin close error. writeErr might be non-nil if the response write failed with ErrServerClosing. But ErrServerClosing wraps ErrUnknown... Let's check: the write failure in c.write() does NOT set s.writeErr (line 798: only sets writeErr when ctx.Err() == nil AND not ErrRejected). ErrServerClosing is not ErrRejected, so writeErr IS set. Then Wait() returns writeErr = ErrServerClosing. server.Run's ssClosed arm receives ErrServerClosing. server.Run returns ErrServerClosing (not context.Canceled).

- timestamp: 2026-04-03
  checked: The actual "context canceled" error text
  found: The error message is "MCP server error: context canceled". This is context.Canceled. Looking at server.Run more carefully: it waits on select{ ctx.Done OR ssClosed }. If ssClosed fires first with an ErrServerClosing error, it returns that. If ctx fires first... Actually, when the echo pipe closes and the jsonrpc2 connection tears down rapidly, there's a race. The ctx passed to server.Run is a signal context that does NOT cancel on stdin EOF — it only cancels on SIGTERM/SIGINT or explicit stop() call. So ctx.Done should NOT fire during this test. UNLESS: the context passed down into the jsonrpc2 connection propagates cancellation back up. Looking at conn.go NewConnection: it wraps ctx with notDone{ctx} (line 219) — this strips cancellation propagation! So the connection's internal context cannot cancel due to the outer ctx. The "context canceled" error must come from somewhere else in the call chain.
  implication: The error text "context canceled" might be from the write attempt inside the MCP server handler, where the request context gets canceled when the connection closes (req.cancel() is called on all in-flight requests when readErr is set or when shuttingDown).

## Resolution

root_cause: |
  The root cause is a race condition between stdin EOF propagation and the MCP initialize response write, caused by the echo pipe closing stdin immediately after sending the message.

  Detailed mechanism:
  1. `echo` sends one JSON line and closes the pipe. The server's stdin (os.Stdin) gets EOF immediately after the message.
  2. In `newIOConn`, a background goroutine reads from stdin using json.NewDecoder. It successfully decodes the initialize message and sends it to the `incoming` channel. The goroutine then loops, calls Decode again, gets io.EOF, and sends that error to `incoming`.
  3. The jsonrpc2 connection's `readIncoming` goroutine reads the initialize message from `incoming` and calls `acceptRequest`, which enqueues it to `handleAsync`.
  4. The jsonrpc2 connection's `readIncoming` goroutine ALSO reads the EOF from `incoming` (step 2) and sets `s.readErr = io.EOF`, putting the connection into `shuttingDown` state.
  5. Steps 3 and 4 race. Steps 3+4 happen nearly simultaneously because the echo pipe produces EOF immediately.
  6. When `handleAsync` processes the initialize request and the MCP SDK tries to write the response via `processResult -> write -> c.write`, the `c.write` method calls `shuttingDown(ErrServerClosing)`. Since `s.readErr` is already set (step 4), `shuttingDown` returns a non-nil error and the write is abandoned — the initialize response is never sent to stdout.
  7. The in-flight initialize request's context is cancelled (via `req.cancel()` called during connection teardown), causing the "context canceled" error that propagates back up through server.Run.

  The fundamental problem is that `StdioTransport` uses `os.Stdin` directly, which closes (EOF) as soon as the piped input is exhausted. The jsonrpc2 library's design treats an EOF read error as a signal to stop accepting new work and reject writes, meaning a server that gets its full input in a single `echo` pipe will always race between processing the message and detecting EOF.

fix: |
  Three viable approaches:

  1. **Use a persistent stdin (recommended for real MCP clients):** Real MCP clients (Claude Desktop, Cursor, etc.) keep stdin open and send `notifications/initialized` after the `initialize` response. The server is designed for this interactive use. The `echo | graphmd mcp` test pattern is not a valid MCP session — it closes stdin before the server can respond.

  2. **For testing/scripting: use a FIFO or process substitution that keeps stdin open:**
     ```bash
     # Keep stdin open by also sending a blocking read
     { echo '{"jsonrpc":"2.0",...}'; cat; } | graphmd mcp
     # Then Ctrl-C to stop
     ```
     Or use a named pipe:
     ```bash
     mkfifo /tmp/mcp-test
     graphmd mcp < /tmp/mcp-test &
     echo '{"jsonrpc":"2.0",...}' > /tmp/mcp-test
     ```

  3. **Code fix — buffer stdin before connecting:** In `internal/mcp/server.go Run()`, wrap `os.Stdin` in a custom `io.ReadCloser` that reads all available data then blocks (never returns EOF) until the context is cancelled. This prevents the SDK from seeing EOF when the pipe closes:
     ```go
     // Instead of &mcpsdk.StdioTransport{}, use:
     return server.Run(ctx, &mcpsdk.IOTransport{
         Reader: &noEOFReader{r: os.Stdin, ctx: ctx},
         Writer: os.Stdout,
     })
     ```
     Where `noEOFReader` blocks on Read after getting EOF until ctx is done.

     Alternatively, wrap with `mcpsdk.IOTransport` and a buffered reader that converts EOF into a blocking read.

  4. **Code fix — use IOTransport with stdin wrapped to block on EOF:** The cleanest fix that preserves the single-shot test case is to wrap stdin so that EOF blocks until context cancellation, making the server outlive the pipe:
     ```go
     // In Run(), replace StdioTransport with:
     pr, pw := io.Pipe()
     go func() {
         io.Copy(pw, os.Stdin)
         <-ctx.Done()  // hold pipe open until context ends
         pw.Close()
     }()
     return server.Run(ctx, &mcpsdk.IOTransport{
         Reader: pr,
         Writer: nopCloser{os.Stdout},
     })
     ```

files_changed: []
