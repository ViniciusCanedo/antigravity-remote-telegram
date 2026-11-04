# Proposal: CLI Commands, Entrypoint, ASCII Banners, and Update Checker

## Why

The `antigravity-telegram-remote` project requires a single, unified entrypoint executable (`cmd/antigravity-bot/main.go`) and an idiomatic Go CLI structure (`internal/cli/`) powered by `spf13/cobra`. In the original TypeScript codebase, entrypoints were split between `src/index.ts` and `src/bin/cli.ts`. In Go, as mandated by ADR-003, these entrypoints are unified into a single binary that provides:
1. Root command execution with auto-detection: if no configuration (`~/.antigravity-telegram-remote/config.json` or `.env`) exists, display the ASCII welcome banner and launch the interactive setup wizard; otherwise acquire the lockfile and start the bot daemon.
2. The four subcommands: `start`, `setup`, `open`, and `doctor` (including the `--selectors` flag for DOM health auditing).
3. Non-blocking update check with 24-hour cooldown against the npm registry.
4. Interactive setup wizard with secret masking (`golang.org/x/term`), Telegram token verification via Telegram Bot API `getMe`, and workspace directory auto-completion.

## What Changes

- Create `cmd/antigravity-bot/main.go`: Entrypoint calling `cli.Execute()`.
- Create `internal/cli/logo.go`: ASCII banners `Logo` and `SetupLogo`.
- Create `internal/cli/welcome.go` and `internal/cli/welcome_test.go`: `VisibleLength`, `FormatWelcomeBox`, `PrintWelcome`.
- Create `internal/cli/root.go` and `internal/cli/root_test.go`: Cobra root command registering `start`, `setup`, `open`, and `doctor`, plus `ResolveCliLogLevel`.
- Create `internal/cli/start.go` and `internal/cli/start_test.go`: `start` subcommand with lock acquisition, non-blocking update checking, and bot daemon startup.
- Create `internal/cli/setup.go` and `internal/cli/setup_test.go`: 3-step interactive setup wizard, token validation, allowed user IDs validation, and workspace directory setup.
- Create `internal/cli/open.go` and `internal/cli/open_test.go`: `open` subcommand to find an available CDP port and open Antigravity on macOS, Windows, or Linux.
- Create `internal/cli/doctor.go` and `internal/cli/doctor_test.go`: `doctor` subcommand with 7 system diagnosis steps, ADR-001 cross-platform binary check, and `--selectors` DOM health audit (integrating `RunDoctorSelectors`).
- Create `internal/cli/update_check.go` and `internal/cli/update_check_test.go`: `UpdateCheckService` with 24h cache, semver comparison, and non-blocking notification, matching 1:1 with `tests/services/updateCheckService.test.ts`.

## Capabilities

### New Capabilities
- `cli-entrypoint`: Unified CLI entrypoint, Cobra root command, 4 subcommands (`start`, `setup`, `open`, `doctor`), ASCII banners, interactive wizard, and non-blocking update checker.

### Modified Capabilities

## Impact

- Provides the main application binary (`cmd/antigravity-bot`).
- Integrates all internal packages (`internal/bot`, `internal/config`, `internal/platform`, `internal/logger`, `internal/cdp`, `internal/monitor`).
- Produces a production-ready standalone executable.
