# Gemini Web CLI

Terminal client for Google Gemini, powered by your Google One web subscription.

Gemini Web CLI lets you interact with Google Gemini directly from the terminal — no API key needed. It uses your browser cookies to authenticate with the Gemini web interface, giving you access to the same models available through gemini.google.com.

## Features

- **Interactive Chat** — Full-featured TUI with markdown rendering, streaming responses, and conversation management
- **Single-shot Queries** — Quick `ask` command for scripting and one-off questions
- **Multiple Models** — Switch between Gemini 3.0 Pro, Flash, and Flash Thinking
- **Gems (System Prompts)** — Create, list, and use Gems to customize model behavior
- **Thinking Mode** — View the model's reasoning process with `--show-thoughts`
- **Image Extraction** — Displays web images and ImageFX-generated images from responses
- **File Upload** — Attach files to your conversations
- **Conversation History** — Browse and manage past conversations
- **Multi-Account** — Manage multiple Google accounts with easy switching
- **Cookie Auto-Refresh** — Background PSIDTS rotation keeps sessions alive
- **Retry with Backoff** — Automatic retry on transient errors and rate limits
- **Proxy Support** — HTTP/SOCKS5 proxy via CLI flag, config, or environment variables
- **Custom Models** — Define custom model hashes in config for new/experimental models

## Installation

### Build from source

```bash
git clone https://github.com/harris/gemini-web-cli.git
cd gemini-web-cli
make build
```

The binary will be at `bin/gemini-web-cli`.

### Cross-compile

```bash
make build-all  # darwin-arm64, darwin-amd64, linux-arm64
```

## Quick Start

```bash
# 1. Log in (opens browser for Google authentication)
gemini-web-cli login

# 2. Ask a question
gemini-web-cli ask "What is the capital of France?"

# 3. Start interactive chat
gemini-web-cli chat
```

## Usage

### Ask (non-interactive)

```bash
gemini-web-cli ask "Explain quicksort"
gemini-web-cli ask --model gemini-3.0-flash "Summarize this article"
gemini-web-cli ask --gem "Code Reviewer" "Review this function"
gemini-web-cli ask --show-thoughts "Solve: 23 * 47"
```

### Chat (interactive TUI)

```bash
gemini-web-cli chat
gemini-web-cli chat --model gemini-3.0-pro
gemini-web-cli chat --gem "Writing Assistant"
```

In-chat commands:
- `/new` — Start a new conversation
- `/model <name>` — Switch model (or interactive picker)
- `/upload <path> [question]` — Attach a file (optionally ask about it immediately)
- `/history` — Browse conversation history (arrow keys to navigate, Enter to open, continue past conversations)
- `Enter` — Send message
- `Double ESC` — Cancel generation in progress
- `Ctrl+C` — Quit

### Gems

```bash
gemini-web-cli gems list
gemini-web-cli gems create --name "Code Reviewer" --prompt "You are an expert code reviewer..." --desc "Reviews code"
gemini-web-cli gems delete <gem-id>
```

### History

```bash
gemini-web-cli history                      # List all conversations (CLI)
```

In chat mode, `/history` opens an interactive browser:
- **↑↓** — Move cursor to select a conversation
- **← →** — Switch pages (10 per page)
- **Enter** — Open and view conversation messages
- **c** — Continue the selected conversation (restores session context)
- **b** — Back to conversation list
- **↑↓** (in detail view) — Scroll through messages
- **ESC** — Exit history mode

### Accounts

```bash
gemini-web-cli login                        # Log in (default account)
gemini-web-cli login --account work         # Log in with named account
gemini-web-cli accounts list                # List all accounts
gemini-web-cli accounts switch work         # Switch default account
gemini-web-cli accounts remove work         # Remove an account
```

### Global Flags

```bash
--proxy socks5://127.0.0.1:1080       # Use proxy for all requests
```

## Available Models

| Name | Description |
|------|-------------|
| `default` | Server default (no model header) |
| `gemini-3.0-pro` | Gemini 3.0 Pro |
| `gemini-3.0-flash` | Gemini 3.0 Flash |
| `gemini-3.0-flash-thinking` | Gemini 3.0 Flash Thinking (supports `--show-thoughts`) |

## Configuration

Config file: `~/.config/gemini-web-cli/config.json` (respects `XDG_CONFIG_HOME`)

```json
{
  "default_account": "default",
  "default_model": "gemini-3.0-pro",
  "request_timeout": 120,
  "request_delay_ms": 500,
  "proxy": "socks5://127.0.0.1:1080",
  "custom_models": {
    "my-model": {
      "name": "My Custom Model",
      "header_val": "[1,null,null,null,\"abcdef1234567890\",null,null,0,[4],null,null,1]"
    }
  }
}
```

## Project Structure

```
gemini-web-cli/
├── main.go                          # Entry point
├── cmd/                             # CLI commands (cobra)
│   ├── root.go                      # Root command, global flags
│   ├── ask.go                       # Single-shot query
│   ├── chat.go                      # Interactive TUI
│   ├── gems.go                      # Gem management
│   ├── history.go                   # Conversation history
│   ├── login.go                     # Browser-based login
│   ├── accounts.go                  # Multi-account management
│   └── helpers.go                   # Shared client init, gem resolution
├── internal/
│   ├── api/                         # Gemini Web API client
│   │   ├── client.go                # HTTP client, auth, cookie refresh, retry
│   │   ├── generate.go              # StreamGenerate endpoint, response parsing
│   │   ├── conversations.go         # batchexecute RPC, conversation CRUD
│   │   ├── gems.go                  # Gem CRUD via batchexecute
│   │   ├── models.go                # Model definitions and lookup
│   │   ├── errors.go                # Structured error codes
│   │   ├── upload.go                # File upload
│   │   └── parsing.go               # Frame parser utilities
│   ├── auth/                        # Authentication
│   │   ├── cookies.go               # Cookie handling, validation, rotation
│   │   ├── login.go                 # Browser-based login flow
│   │   └── store.go                 # Credential persistence
│   ├── config/                      # Configuration
│   │   └── config.go                # JSON config load/save
│   └── tui/                         # Terminal UI (bubbletea)
│       ├── app.go                   # Top-level app model
│       ├── chat.go                  # Chat view with streaming
│       ├── history.go               # History browser
│       └── styles.go                # Lipgloss styles
├── pkg/version/                     # Version info (ldflags)
├── Makefile                         # Build targets
└── go.mod
```

## How It Works

Gemini Web CLI reverse-engineers the Gemini web interface endpoints:

1. **Authentication** — Uses browser cookies (`__Secure-1PSID`, `__Secure-1PSIDTS`, `__Secure-1PSIDCC`) extracted via headless browser login
2. **Session Init** — Fetches `gemini.google.com/app` to extract CSRF token (`SNlM0e`), request context (`cfb2h`), and session ID (`FdrFJe`)
3. **Generation** — POSTs to the `StreamGenerate` endpoint with a 69-element request array, double-JSON encoded
4. **Model Selection** — Sets the `x-goog-ext-525001261-jspb` header with model-specific hex hashes
5. **Conversations & Gems** — Uses the `batchexecute` RPC endpoint with specific RPC IDs for CRUD operations
6. **Cookie Refresh** — Background goroutine rotates `PSIDTS` every 9 minutes via Google's `RotateCookies` endpoint

## Requirements

- Go 1.21+
- Google One subscription (for Gemini access)
- Chrome/Chromium (for initial login flow)

## License

MIT
