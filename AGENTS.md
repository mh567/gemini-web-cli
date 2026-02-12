# AGENTS.md

This file is a lightweight control guide for coding agents working in this repo.
It is derived from `CLAUDE.md` and kept intentionally concise.

## Scope
- Project: `gemini-web-cli` (Go CLI + Bubbletea TUI).
- API source: reverse-engineered Gemini Web endpoints (not official Gemini API key flow).

## Control Rules
1. Read `CLAUDE.md` and this file before large changes.
2. Prefer existing build commands in `Makefile`:
   - `make build`
   - `make build-all`
3. In this environment, use `GOCACHE=/tmp/go-build` when running Go build/test commands.
4. Validate changes with:
   - `GOCACHE=/tmp/go-build go test ./...`
   - or at minimum `go build ./...`
5. For endpoint/payload/model-hash uncertainty, check reference repo first:
   - `/tmp/Gemini-API/` (HanaokaYuzu/Gemini-API)
6. Do not introduce destructive git operations unless explicitly requested.
7. Keep TUI behavior consistent with existing state machine patterns in `internal/tui/chat.go`.
8. Keep docs in sync when command behavior changes (`README.md`, `README_CN.md`).

## Release Control (Manual)
1. Ensure clean status and expected version/tag.
2. Build artifacts:
   - `GOCACHE=/tmp/go-build make build-all`
3. Package artifacts and checksums into `dist/`.
4. Verify GitHub auth first:
   - `gh auth status`
5. Create/push tag, then publish release with `gh release create`.

