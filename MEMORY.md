# MEMORY.md

Persistent project memory for future sessions.

## Project Facts
- Language: Go
- CLI: `spf13/cobra`
- TUI: Bubbletea + Bubbles + Lipgloss + Glamour
- Entry: `main.go` -> `cmd/root.go`
- Core API client: `internal/api/`

## API/Runtime Notes
- Gemini Web auth uses browser cookies (`__Secure-1PSID*`).
- `Client.Init()` extracts dynamic tokens from Gemini web app page.
- Stream responses are full-text snapshots; UI should treat chunks as latest full state.
- Thought text is extracted from candidate path `cand[37][0][0]` (see `internal/api/generate.go`).

## TUI Notes
- Main interactive logic is in `internal/tui/chat.go`.
- History browsing is inline state inside `ChatModel`, not a separate screen process.
- Current behavior: thoughts are collapsed by default in chat mode.
- Toggle command: `/thoughts [on|off]`.
- Thought rendering normalizes whitespace to reduce oversized blank gaps.

## Build/Validation Memory
- Normal compile:
  - `make build`
  - `make build-all`
- In this sandbox, default Go cache path may fail with permissions.
  - Use: `GOCACHE=/tmp/go-build go test ./...`
  - Use: `GOCACHE=/tmp/go-build make build-all`

## Release Memory (Current Repo)
- There is no dedicated release script/workflow file in repo.
- Manual release steps used in this workspace:
  1. Build: `GOCACHE=/tmp/go-build make build-all`
  2. Package `bin/gemini-web-cli-*` into `dist/*.tar.gz`
  3. Generate checksums: `shasum -a 256 dist/*.tar.gz > dist/checksums.txt`
  4. Auth check: `gh auth status`
  5. Publish: `gh release create <tag> dist/*.tar.gz dist/checksums.txt -t "<tag>"`

## Release Prerequisites
- `gh` token must be valid (`gh auth status`).
- Prefer clean git tree before tagging.
- `VERSION` from Makefile uses `git describe --tags --always --dirty`; dirty tree yields `-dirty`.

