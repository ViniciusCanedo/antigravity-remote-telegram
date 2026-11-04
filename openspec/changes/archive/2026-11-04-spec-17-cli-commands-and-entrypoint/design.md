# Design: CLI Commands, Entrypoint, ASCII Banners, and Update Checker

## Architecture Overview

The CLI architecture wraps the entire application functionality into an idiomatic `cobra.Command` tree under `internal/cli/`, driven by a lightweight entrypoint at `cmd/antigravity-bot/main.go`.

```
cmd/antigravity-bot/
  └── main.go                  -> Calls cli.Execute()

internal/cli/
  ├── logo.go                  -> ASCII art constants (Logo, SetupLogo)
  ├── welcome.go               -> Welcome box layout, ANSI length calculation, PrintWelcome
  ├── root.go                  -> Root Cobra command, flags (--verbose, --quiet), fallback logic
  ├── start.go                 -> 'start' subcommand, lockfile acquisition, update check, bot startup
  ├── setup.go                 -> 'setup' subcommand, terminal raw mode, token verification, config saving
  ├── open.go                  -> 'open' subcommand, CDP port discovery, OS process spawning
  ├── doctor.go                -> 'doctor' subcommand, 7 system checks, --selectors audit
  ├── doctor_selectors.go      -> DOM selector auditor via CDP (already in place)
  └── update_check.go          -> Non-blocking NPM registry update check with 24h cache
```

## Data Flow & Invariants

1. **Root Invocation Flow**:
   - `Execute()` invokes `NewRootCmd().Execute()`.
   - If no subcommand is provided:
     - Check `config.ConfigExists()` and `.env` presence in current working directory.
     - If neither exists: display `PrintWelcome()` and execute `SetupAction(ctx)`.
     - Otherwise: execute `StartAction(ctx, rootOpts, nil)`.

2. **Start Subcommand Flow**:
   - Resolve CLI log level: `--verbose` -> `debug`, `--quiet` -> `error`, otherwise fall through to config/env.
   - If specified, update global logger level.
   - Print `Logo`.
   - Acquire lock via `platform.AcquireLock()` to guarantee a single running bot instance.
   - Spawn asynchronous goroutine `go CheckForUpdates(ctx, Version)`.
   - Call `bot.StartBot(ctx, cliLevel)`.

3. **Setup Subcommand Flow**:
   - Step 1: Prompt for `TELEGRAM_BOT_TOKEN`. In terminal mode, mask keystrokes with `*` and handle backspace/Ctrl+C safely with terminal restoration. Verify token online via `https://api.telegram.org/bot<token>/getMe` (timeout 10s).
   - Step 2: Prompt for `ALLOWED_USER_IDS`. Validate comma-separated numeric IDs.
   - Step 3: Prompt for `WORKSPACE_BASE_DIR`. Default to `~/Code`. Support path autocompletion, expand `~`, confirm creation if path does not exist.
   - Persist to `~/.antigravity-telegram-remote/config.json` via `config.Save()`.

4. **Open Subcommand Flow**:
   - Check ports in `[9222, 9223, 9333, 9444, 9555, 9666]` using ephemeral TCP listeners.
   - Select first available port.
   - Launch Antigravity with `--remote-debugging-port=<port>` per operating system (`open -a Antigravity` on macOS, shell spawn on Windows, executable spawn on Linux).

5. **Doctor Subcommand Flow**:
   - If `--selectors` flag is set: audit 7 DOM selectors using CDP (`RunDoctorSelectors`).
   - Otherwise, perform 7 checks:
     1. Config directory presence.
     2. Config file presence.
     3. `.env` file presence and loading.
     4. Required environment variables (`TELEGRAM_BOT_TOKEN`, `ALLOWED_USER_IDS`).
     5. Go runtime version.
     6. Antigravity binary installation (ADR-001).
     7. CDP port responsiveness (`http://127.0.0.1:<port>/json/list`).

6. **Update Check Flow**:
   - Check `~/.antigravity-telegram-remote/update-check.json`.
   - If file does not exist, is invalid, or `now - lastCheck >= 24h`, perform HTTP GET to npm registry (5s timeout).
   - If registry version > current version, print notice.
   - Write updated `lastCheck` to cache file.
   - Never block or crash the host process on failure.

## Trade-offs & Decisions

- **Terminal Raw Mode**: Secret input in `setup` uses `golang.org/x/term` with careful state restoration (`defer term.Restore`) to prevent terminal corruption on abrupt exit or SIGINT.
- **Testability**: Interactive prompts, lockfile, CDP checks, and HTTP requests are abstracted or injected via dependency structs and functional arguments so all components can be thoroughly unit tested without requiring real terminals, network access, or browser processes.
