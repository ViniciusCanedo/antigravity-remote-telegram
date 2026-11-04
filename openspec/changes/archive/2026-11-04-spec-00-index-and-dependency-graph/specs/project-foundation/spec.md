## ADDED Requirements

### Requirement: Go Module Declaration
The project SHALL define a `go.mod` file with module path `antigravity-telegram-remote` and Go version 1.23 or newer.

#### Scenario: Verify go.mod module and Go version
- **WHEN** `go.mod` is inspected
- **THEN** it specifies `module antigravity-telegram-remote` and Go version 1.23 or higher

### Requirement: Pinned External Dependencies
The Go project SHALL declare all standardized external dependencies in `go.mod` and `go.sum` to support subsequent implementation specs without requiring CGO:
- `github.com/go-telegram/bot`
- `github.com/coder/websocket`
- `modernc.org/sqlite`
- `github.com/spf13/cobra`
- `golang.org/x/term`
- `github.com/robfig/cron/v3`
- `github.com/joho/godotenv`

#### Scenario: Dependencies successfully resolved and tidied
- **WHEN** `go mod tidy` is executed
- **THEN** all specified dependencies are resolved in `go.mod` and checksums recorded in `go.sum` without error

#### Scenario: Pure Go compilation without CGO
- **WHEN** packages are built with `CGO_ENABLED=0 go build ./...`
- **THEN** the build succeeds without requiring a C compiler

### Requirement: Git Ignore Rules
The project SHALL maintain a `.gitignore` file that prevents tracking local artifacts, runtime databases, lock files, and build outputs.

#### Scenario: Runtime and build files ignored
- **WHEN** files matching `*.db`, `*.db-*`, `*.log`, `.bot.lock`, `coverage.out`, binary outputs, or `.worktrees/` are present in the workspace
- **THEN** `git status` reports them as ignored according to `.gitignore`
