# CLAUDE.md — Gemini Web CLI Project Guide

## Project Overview

Gemini Web CLI is a Go terminal client for Google Gemini, powered by Google One web subscriptions (NOT the official Gemini API). It reverse-engineers the Gemini web interface endpoints, using browser cookies for authentication. This is **not** an official Google product.

## Reference Implementation

When adding new features or fixing bugs, **always check the [HanaokaYuzu/Gemini-API](https://github.com/HanaokaYuzu/Gemini-API) Python library first** for reference. This project reverse-engineers the same Gemini web endpoints, and its implementation has been the authoritative source for:

- Correct payload formats (e.g., file reference structure `[[url], filename]`)
- Model header hashes (`x-goog-ext-525001261-jspb` values)
- New/changed endpoints and RPC IDs
- Streaming response parsing logic
- Error codes and their meanings

A local clone is kept at `/tmp/Gemini-API/` for quick reference. Re-clone if stale:
```bash
git clone https://github.com/HanaokaYuzu/Gemini-API /tmp/Gemini-API
```

## Build & Run

```bash
# Build
make build                    # → bin/gemini-web-cli (current platform)
make build-all                # Cross-compile: darwin-arm64, darwin-amd64, linux-arm64
make clean                    # Remove bin/

# Quick compile check
go build ./...

# Run
bin/gemini-web-cli login                          # Browser-based Google login
bin/gemini-web-cli ask "hello"                    # Single-shot query
bin/gemini-web-cli ask "hello" --model gemini-3.0-flash  # With specific model
bin/gemini-web-cli ask "hello" --gem "MyGem"      # With Gem system prompt
bin/gemini-web-cli ask "hello" --show-thoughts    # Show thinking process
bin/gemini-web-cli chat                           # Interactive TUI
bin/gemini-web-cli chat --gem "MyGem"             # TUI with Gem
bin/gemini-web-cli gems list                      # List Gems
bin/gemini-web-cli gems create --name X --prompt Y  # Create Gem
bin/gemini-web-cli gems delete <id>               # Delete Gem
bin/gemini-web-cli history                        # Conversation history
bin/gemini-web-cli accounts list                  # Multi-account management
bin/gemini-web-cli version                        # Print version info
```

There are **no tests** in this project. Use `go build ./...` to verify compilation.

## Project Structure

```
main.go                         Entry point → cmd.Execute()
cmd/                            CLI commands (Cobra)
  root.go                       Root command, --proxy global flag, version subcommand
  ask.go                        Single-shot query (--model, --gem, --show-thoughts)
  chat.go                       Interactive TUI launcher (--gem)
  gems.go                       Gem CRUD commands (list/create/update/delete)
  history.go                    Conversation history command
  login.go                      Browser-based login (go-rod headless Chrome)
  accounts.go                   Multi-account management (list/switch/remove)
  helpers.go                    Shared: initClient(), resolveGemName()
internal/api/                   Gemini Web API client (core logic)
  client.go                     HTTP client, cookie refresh goroutine, retry with backoff
  generate.go                   StreamGenerate endpoint, true streaming parser, response extraction
  conversations.go              batchexecute RPC, conversation list/get/delete
  gems.go                       Gem CRUD via batchexecute RPCs
  models.go                     Built-in + custom model definitions and lookup
  errors.go                     Structured error codes (1013/1037/1050/1052/1060)
  upload.go                     Streaming file upload via io.Pipe + multipart
  parsing.go                    Frame parser (bracket-matching JSON extraction, NavJSON helper)
internal/auth/                  Authentication
  cookies.go                    Cookie header building, validation (streaming scan), PSIDTS rotation
  login.go                      Headless browser login flow (go-rod)
  store.go                      Credential persistence (system keyring via go-keyring)
internal/config/                Configuration
  config.go                     JSON config (~/.config/gemini-web-cli/config.json), XDG support
internal/tui/                   Terminal UI (Bubbletea)
  app.go                        Top-level model (ModeChat ↔ ModeHistory switching)
  chat.go                       Chat view (streaming, render cache, slash commands, model picker)
  history.go                    History browser (legacy, mostly unused — history now inline in chat.go)
  styles.go                     Lipgloss style definitions
pkg/version/                    Version info (injected via ldflags at build time)
```

## Architecture & Key Patterns

### Gemini Web API

All API communication reverse-engineers the Gemini web interface:

- **Authentication**: Browser cookies (`__Secure-1PSID`, `__Secure-1PSIDTS`, `__Secure-1PSIDCC`, optional `NID`)
- **Session init**: `Client.Init()` → GET `gemini.google.com/app` → regex extract `SNlM0e` (CSRF), `cfb2h` (bl param), `FdrFJe` (f.sid)
- **Generation**: POST to `/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate`
- **Conversations & Gems**: POST to `/_/BardChatUi/data/batchexecute` with RPC IDs (must include `source-path=%2Fapp` query param)
- **Model selection**: `x-goog-ext-525001261-jspb` header with hex hash
- **Cookie refresh**: Background goroutine rotates `__Secure-1PSIDTS` every 9 minutes via `RotatePSIDTS()`
- **Retry**: `Client.Retry()` wraps calls with exponential backoff (attempt * 5s) for retryable errors (1013, 1037)

### Client Initialization Flow

```
initClient() [cmd/helpers.go]
  → config.Load()                    # Load ~/.config/gemini-web-cli/config.json
  → auth.NewStore()                  # Open system keyring
  → store.LoadCookies(account)       # Load cookies for default account
  → api.NewClientWithProxy()         # Create HTTP client (with optional proxy)
  → client.SetStore(store)           # Enable cookie persistence on refresh
  → client.Init()                    # Fetch tokens from Gemini page
  → client.StartCookieRefresh()      # Start background PSIDTS rotation
```

Proxy priority: CLI `--proxy` flag > config `proxy` field > Go stdlib `HTTP_PROXY`/`HTTPS_PROXY` env vars.

### StreamGenerate Request Format

- Form field `f.req`: double-JSON encoded **69-element array**
- Key indices: `[0]`=message, `[2]`=conversation metadata, `[7]`=1 (streaming), `[19]`=gem ID
- Message format: `[prompt, 0, null, fileData, null, null, 0]`
- New chat metadata: `["","","",null,null,null,null,null,null,""]`
- Query params: `_reqid`, `rt=c`, `bl=<cfb2h>`, `f.sid=<FdrFJe>`

### Streaming Response

- Google's length-prefixed frame protocol: `)]}\n<length>\n<json>`
- **Each frame contains FULL text so far, not deltas** — must track last text and compute delta
- Line-based parsing: `readStream()` reads lines, skips `)]}'` and numeric lines, parses `[`-prefixed lines as JSON (do NOT use length-prefixed byte reading — breaks with multi-byte UTF-8)
- Helper functions: `navRaw()`, `navInt()`, `navString()` for safe nested JSON navigation
- Response structure: `inner[1]` = conversation metadata, `inner[4]` = candidates array, `inner[5][2][0][1][0]` = error code (deep nested)
- Candidate structure:
  - `cand[0]` = choiceID (rcid)
  - `cand[1][0]` = text (fallback `cand[22][0]` for googleusercontent URLs)
  - `cand[37][0][0]` = thoughts (NOT `cand[3]`)
  - `cand[12][1][*]` = web images → URL `[0][0][0]`, title `[7][0]`, alt `[0][4]`
  - `cand[12][7][0][*]` = generated images → URL `[0][3][3]`

### batchexecute RPC IDs

| Operation | RPC ID |
|-----------|--------|
| List conversations | `MaZiqc` |
| Get conversation | `hNvQHb` |
| Delete conversation | `GzXR5e` |
| List Gems | `CNgdBe` |
| Create Gem | `oMH3Zd` |
| Update Gem | `kHv0Vd` |
| Delete Gem | `UXcSJb` |

**Important**: `batchExecute()` builds `f.req` via `json.Marshal` (not string concatenation) to safely handle payloads containing quotes. The URL must include `source-path=%2Fapp` — without it, the server returns null data.

### Conversation Detail Parsing (hNvQHb)

Turn structure from `inner[0]` array: `[0]=[convID,responseID], [1]=null, [2]=userMsg, [3]=modelResp, [4]=timestamp`

- **User text**: `turn[2][0][0]`
- **Model text**: `turn[3][0][0][1][0]`
- **ResponseID**: `lastTurn[0][1]` (for conversation continuation)
- **ChoiceID**: `lastTurn[3][3]` (for conversation continuation)

### TUI Architecture (Bubbletea)

- **Value-type models**: Bubbletea passes models by value, NOT pointer. Never store channels or mutable state that relies on pointer identity.
- **AppModel** → top-level router, switches between `ModeChat` and `ModeHistory`, tracks window size
- **ChatModel** → handles streaming, render cache (`rendered []string`), slash commands (`/new`, `/model`, `/upload`, `/history`), model picker, and inline history browser
- **History state machine** (inline in ChatModel, NOT a separate model):
  - States: `historyNone` → `historyLoading` → `historyList` ↔ `historyLoading` → `historyView`
  - `historyList`: cursor-based selection rendered in `View()`, arrow keys navigate, Enter opens, n/p for pages
  - `historyView`: scrollable message view, `c` to continue conversation, `b` to go back
  - ESC from any history state returns to `historyNone` (normal chat)
  - All history UI rendered in `View()` (in-place updates), NOT via `tea.Println`
- Streaming pattern: `sendMessageWithFiles()` → first `streamChunkMsg` → `handleStreamChunk()` chains next read via `msg.ch`
- **Render cache**: `updateViewport()` only renders new messages, reuses cached output for existing ones. Cache invalidated on window resize or `/new`.
- **Glamour renderer**: Recreated on window resize with `msg.Width-4` word wrap for adaptive layout
- **Double-ESC to cancel**: During streaming, first ESC records time, second ESC within 500ms interrupts generation

### Model Header Format

```
x-goog-ext-525001261-jspb: [1,null,null,null,"<hex_hash>",null,null,0,[4],null,null,1]
```

Model hashes are opaque hex strings that change when Google updates models. Empty header = server default.

## Configuration

Config file: `~/.config/gemini-web-cli/config.json` (respects `XDG_CONFIG_HOME`)

```json
{
  "default_account": "default",
  "default_model": "gemini-2.5-pro",
  "request_timeout": 120,
  "request_delay_ms": 500,
  "proxy": "socks5://127.0.0.1:1080",
  "custom_models": {
    "my-model": {
      "name": "My Custom Model",
      "header_val": "[1,null,null,null,\"abcdef123456\",null,null,0,[4],null,null,1]"
    }
  }
}
```

Custom models are looked up after built-in models in `GetModel()`. Users can reference them with `--model my-model`.

## Code Conventions

- **Language**: Go, module `github.com/harris/gemini-web-cli`
- **CLI framework**: `spf13/cobra` — commands in `cmd/`, each file registers via `init()`
- **TUI framework**: `charmbracelet/bubbletea` + `bubbles` + `glamour` + `lipgloss`
- **Error handling**: Wrap with `fmt.Errorf("context: %w", err)`, use `errors.As` for typed errors
- **Structured errors**: `GeminiError` with numeric codes, `IsRetryable()` for retry logic
- **Cookie safety**: `sync.RWMutex` protects cookie header access (`cookieMu`); cached header string avoids rebuilding on every request
- **Streaming uploads**: `io.Pipe` streams multipart body without buffering entire file in memory
- **String building**: Use `strings.Builder` for concatenation in loops (not `+=`)
- **No test files** — verify changes with `go build ./...`
- **Version injection**: via `ldflags` in Makefile (`-X pkg/version.Version=...`, `-X ...GitCommit=...`, `-X ...BuildDate=...`)

## Common Pitfalls

### HTTP / API
1. **Old endpoint 404**: Always use `BardChatUi/data/...` path, never `BardChatGenerate`
2. **Accept-Encoding**: Do NOT set `Accept-Encoding` header — Go can't auto-decompress if you do
3. **utls incompatibility**: Project uses standard Go `net/http`, not utls (caused HTTP/2 issues previously)
4. **Host header**: Must set `Host: gemini.google.com` on API requests
5. **f.sid param**: Must include `f.sid` query parameter on all API requests
6. **X-Same-Domain header**: Required on batchexecute requests (`X-Same-Domain: 1`)

### Data Format
7. **Model hashes**: These are NOT human-readable names — they're opaque hex hashes that Google can change at any time
8. **batchexecute f.req encoding**: `batchExecute()` builds `f.req` via `json.Marshal` to safely handle payloads containing quotes — never use string concatenation for this
9. **Double-JSON encoding**: `f.req` field is `[null, "<inner JSON as string>"]` — the inner array is JSON-encoded then embedded as a string in the outer array
10. **HTML entities**: Response text contains HTML entities (`&amp;`, `&lt;`, etc.) — always call `html.UnescapeString()` on extracted text

### TUI
11. **Bubbletea value types**: Never use pointer receivers expecting mutation to persist — return updated model from `Update()`
12. **Stream channel passing**: Carry `<-chan StreamChunk` through `streamChunkMsg.ch`, not on the model struct
13. **History state in View()**: History browser renders inline in `View()` for in-place updates — never use `tea.Println` for interactive UI elements (it appends to scrollback instead of updating in place)
14. **Render cache invalidation**: Must set `m.rendered = nil` when window resizes or conversation resets (`/new`)

### Parsing
15. **Silent parse failures**: Parsing functions should surface errors, not silently `continue` on every failure
16. **RPC ID filtering**: batchexecute responses contain multiple RPC results — always filter by expected RPC ID (e.g., `MaZiqc` for conversations)
17. **source-path query param**: batchexecute URL must include `source-path=%2Fapp` — without it, the server returns null data with error code `[3]`
18. **Conversation list at inner[2]**: `MaZiqc` response structure is `[null, null, [[conv1], [conv2], ...]]` — conversations are at index 2, not 0
19. **Conversation detail turn indices**: User text is at `turn[2][0][0]` (NOT `turn[0]`), model text at `turn[3][0][0][1][0]` (NOT `turn[1]`). ResponseID/ChoiceID come from the last turn, not from `inner[1]` (which is null)

## Available Models

Built-in models defined in `internal/api/models.go`:

| Key | Display Name | Notes |
|-----|-------------|-------|
| `default` | Default (server picks) | Empty header, uses Google's default |
| `gemini-3.0-pro` | Gemini 3.0 Pro | Hash: `9d8ca3786ebdfbea` |
| `gemini-3.0-flash` | Gemini 3.0 Flash | Hash: `fbb127bbb056c959` |
| `gemini-3.0-flash-thinking` | Gemini 3.0 Flash Thinking | Hash: `5bf011840784117a`, supports thoughts |

Hashes sourced from HanaokaYuzu/Gemini-API Python library. These **will break** when Google updates model hashes — check the Python library for updated values.

## Error Codes

Defined in `internal/api/errors.go`:

| Code | Constant | Retryable | Meaning |
|------|----------|-----------|---------|
| 1013 | `ErrTransient` | Yes | Transient server error |
| 1037 | `ErrRateLimited` | Yes | Rate limited |
| 1050 | `ErrModelMismatch` | No | Model not available |
| 1052 | `ErrInvalidHeader` | No | Invalid model header value |
| 1060 | `ErrIPBanned` | No | IP temporarily banned |

## Dependencies

| Library | Purpose |
|---------|---------|
| `spf13/cobra` | CLI command framework |
| `charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `charmbracelet/glamour` | Terminal Markdown rendering |
| `charmbracelet/lipgloss` | Terminal styling |
| `charmbracelet/bubbles` | TUI components (textarea, viewport, spinner, list) |
| `go-rod/rod` | Headless Chrome for browser-based login |
| `zalando/go-keyring` | System keychain storage (macOS Keychain, Linux Secret Service) |
