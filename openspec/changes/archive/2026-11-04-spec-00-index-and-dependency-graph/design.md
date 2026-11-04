## Context

The TypeScript implementation of Antigravity Telegram Remote relies on Node.js runtime, npm packages (`grammy`, `ws`, `better-sqlite3`, `dotenv`), and native C++ modules. The migration to Golang aims to replace all Node runtime dependencies with pure-Go equivalents compiled as a single static binary.

This specification (Spec 00) defines the foundational module structure and declares the agreed external dependencies in `go.mod` so that subsequent specs (Wave 1 to 6) can build on a unified dependency tree.

## Goals / Non-Goals

**Goals:**
- Initialize `go.mod` with module identifier `antigravity-telegram-remote` and Go version `>= 1.23`.
- Explicitly declare and verify external libraries:
  - `github.com/go-telegram/bot` for Telegram Bot API integration
  - `github.com/coder/websocket` for Chrome DevTools Protocol (CDP) WebSocket communication
  - `modernc.org/sqlite` for pure-Go SQLite persistence without CGO
  - `github.com/spf13/cobra` for CLI command dispatching
  - `golang.org/x/term` for terminal password masking and raw input
  - `github.com/robfig/cron/v3` for task scheduling
  - `github.com/joho/godotenv` for `.env` loading
- Configure `.gitignore` to prevent committing runtime databases, lock files, logs, coverage artifacts, built binaries, and git worktrees.
- Ensure all quality gates (`go vet`, `gofmt`, `go build`) succeed.

**Non-Goals:**
- Implementation of domain logic, commands, or CDP clients (delegated to Specs 01 through 17).
- Introduction of CGO dependencies (the project must remain 100% pure Go).

## Decisions

- **Module Identifier `antigravity-telegram-remote`**:
  - *Rationale*: Matches the root project identity and requirements.
  - *Alternative Considered*: `github.com/antigravity-dev/antigravity-telegram-remote`. Using `antigravity-telegram-remote` directly satisfies project conventions and avoids remote repository assumptions.
- **Pure-Go SQLite (`modernc.org/sqlite`) over `mattn/go-sqlite3`**:
  - *Rationale*: Avoids CGO requirements, ensuring seamless cross-compilation across macOS, Linux, and Windows without cross-toolchains.
- **`github.com/coder/websocket` over `gorilla/websocket`**:
  - *Rationale*: Actively maintained, native context cancellation, zero-allocation buffers, and strict RFC compliance for Chrome DevTools Protocol.
- **Go 1.23+ Requirement**:
  - *Rationale*: Leverages range-over-func iterators, enhanced `os/exec` process handling, and standard library optimizations.
- **Dependency Pinning via Root/Internal Module**:
  - *Rationale*: In Go, `go mod tidy` removes dependencies if no source files import them. An internal package references the declared external dependencies so `go.mod` and `go.sum` remain fully populated and verified for downstream feature branches.

## Risks / Trade-offs

- [Risk] Unused dependencies pruned by `go mod tidy`.
  → [Mitigation] Reference external packages in `internal/foundation/deps.go` ensuring `go mod tidy` preserves them and `go build ./...` validates their integrity.
- [Risk] Inadvertent commit of local databases or lockfiles during development.
  → [Mitigation] Explicit patterns in `.gitignore` covering `*.db`, `*.db-*`, `*.log`, `.bot.lock`, `coverage.out`, binaries, and `.worktrees/`.
