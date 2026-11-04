## Why

The Antigravity Telegram Remote project is being migrated from TypeScript/Node.js to pure Golang (1.23+) to achieve zero-runtime-dependency single-binary distribution, lower memory usage, and robust multiplatform concurrency without CGO. Before implementing modular components (Wave 1 through Wave 6), the Go workspace foundation must be established with the unified Go module declaration, verified external dependencies, and version control configurations.

## What Changes

- Initialize `go.mod` for module `antigravity-telegram-remote` targeting Go 1.23+.
- Pin and tidy standardized external dependencies required across all migration specs:
  - `github.com/go-telegram/bot` (Telegram Bot API client)
  - `github.com/coder/websocket` (Chrome DevTools Protocol WebSocket transport)
  - `modernc.org/sqlite` (Pure-Go SQLite engine with WAL support)
  - `github.com/spf13/cobra` (Command-line interface commands and flags)
  - `golang.org/x/term` (Interactive masked terminal input)
  - `github.com/robfig/cron/v3` (Cron expression scheduling engine)
  - `github.com/joho/godotenv` (`.env` file parser)
- Create standard `.gitignore` tailored for Go development, ignoring runtime artifacts, SQLite databases (`*.db`, `*.db-*`), logs (`*.log`), lockfiles (`.bot.lock`), test coverage (`coverage.out`), built binaries, and git worktrees (`.worktrees/`).

## Capabilities

### New Capabilities
- `project-foundation`: Core Go module declaration, pinned external dependencies, and repository hygiene rules.

### Modified Capabilities

## Impact

- Establishes root project configuration (`go.mod`, `go.sum`, `.gitignore`).
- Unblocks all downstream implementation waves (Wave 1 through Wave 6).
- Validates pure Go dependencies compatible with `CGO_ENABLED=0`.
