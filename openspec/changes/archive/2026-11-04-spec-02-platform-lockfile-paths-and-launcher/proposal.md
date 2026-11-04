## Why

The Antigravity Telegram Remote service requires a reliable platform foundation to guarantee single-instance execution via lockfiles, discover Antigravity binary paths across operating systems, monitor Chrome DevTools Protocol (CDP) ports, and discover local language_server processes and listening ports to query model quota usage. Porting these capabilities from TypeScript (`src/utils/lockfile.ts`, `src/utils/pathUtils.ts`, `src/services/antigravityLauncher.ts`, `src/utils/cdpPorts.ts`, and `src/services/quotaService.ts`) into a unified Go package `internal/platform` ensures process lifecycle safety, command-injection protection, parallel CDP port probing, cross-platform process discovery (macOS, Linux, and Windows per ADR-001 and ADR-003), and thread-safe quota querying.

## What Changes

- Create `internal/platform`:
  - **Lockfile Management (`lockfile.go`, `lockfile_test.go`)**:
    - Single-instance enforcement via `~/.antigravity-telegram-remote/.bot.lock`.
    - Active PID verification and graceful termination: `SIGTERM` followed by polling at 50ms intervals up to 5000ms, falling back to `SIGKILL` on timeout.
    - Automatic cleanup on exit signals (`SIGINT`, `SIGTERM`) and clean release checking PID ownership.
    - Injectable dependencies (`LockfileDeps`) for deterministic testing.
  - **Cross-Platform Paths and Hints (`paths.go`, `paths_test.go`)**:
    - `GetAntigravityCliPath()`: Resolves CLI executable with precedence `ANTIGRAVITY_PATH` environment variable > OS defaults (`darwin`, `windows`, `linux`). Validates that `ANTIGRAVITY_PATH` is a regular file to prevent command injection.
    - `ExtractProjectNameFromPath()`: Robust extraction of workspace folder name across POSIX (`/`) and Windows (`\`) paths.
    - `GetAntigravityCdpHint()`: OS-tailored launch command instructions for CDP remote debugging port (default 9222).
    - Injectable dependencies (`PathDeps`) for multi-platform unit testing.
  - **CDP Launcher Port Polling (`launcher.go`, `launcher_test.go`)**:
    - `CDPPorts = [9222, 9223, 9333, 9444, 9555, 9666]`.
    - Parallel polling of `http://127.0.0.1:<port>/json/list` with 2000ms timeout per port.
    - `EnsureAntigravityRunning()`: Emits a 70-column warning banner if no CDP port is active; logs success at debug level if a port is responding.
    - Mockable HTTP GET for isolated testing.
  - **Process Discovery and Quota Service (`proc.go`, `proc_unix.go`, `proc_windows.go`, `proc_test.go`)**:
    - Platform-specific discovery of `language_server` PID and `--csrf_token` via `pgrep`/`/proc` on Unix and `Get-CimInstance`/`wmic` on Windows.
    - Discovery of listening TCP ports via `lsof`/`ss` on Unix and `netstat -ano` on Windows.
    - Universal parsing functions (`ParseUnixProcessInfo`, `ParseLsofListeningPorts`, `ParseSSListeningPorts`, `ParseWindowsProcessInfo`, `ParseNetstatListeningPorts`) testable on any OS.
    - `QuotaService`: Thread-safe querying of local HTTPS `LanguageServerService/GetUserStatus` endpoint with loopback TLS support, PID/token/port caching, and single-retry resilience.

## Capabilities

### New Capabilities
- `platform-layer`: Single-instance lockfile lifecycle, secure path resolution, parallel CDP launcher verification, cross-platform process discovery, and quota status retrieval.

### Modified Capabilities

## Impact

- Establishes the OS and platform interaction layer required by the bot runtime, CLI entrypoints, and quota commands.
- Isolates operating-system-specific process commands behind build tags and testable abstractions.
- No external runtime dependencies; purely native Go libraries and standard OS command runners.
