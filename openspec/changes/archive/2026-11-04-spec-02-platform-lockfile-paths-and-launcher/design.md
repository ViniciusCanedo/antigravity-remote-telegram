## Context

The Antigravity Telegram Remote service interacts directly with the local operating system to enforce a single running daemon instance, resolve system-dependent binaries, communicate with local Chrome DevTools Protocol (CDP) endpoints exposed by Antigravity, and discover local language_server processes to query quota metrics. In the TypeScript implementation, these features were distributed across `src/utils/lockfile.ts`, `src/utils/pathUtils.ts`, `src/services/antigravityLauncher.ts`, `src/utils/cdpPorts.ts`, and `src/services/quotaService.ts`, with some macOS-specific assumptions (`pgrep`, `lsof`, fixed application paths).

Per ADR-001, ADR-002, and ADR-003, the Go port must unify these system-level capabilities in `internal/platform`, support macOS, Linux, and Windows reliably, protect against command injection on custom paths, allow isolated dependency injection for unit testing, and provide thread-safe caching for quota requests.

## Goals / Non-Goals

**Goals:**
- Provide `.bot.lock` file acquisition and release with graceful `SIGTERM` (50ms polling up to 5s) and forceful `SIGKILL` fallback.
- Securely resolve the Antigravity executable path, validating against directory traversal or injection in `ANTIGRAVITY_PATH`.
- Extract workspace project names from POSIX and Windows path representations.
- Probe standard CDP ports (`9222, 9223, 9333, 9444, 9555, 9666`) in parallel with a 2000ms timeout and log the required 70-column banner when inactive.
- Discover `language_server` PID and `--csrf_token` and its active TCP listening ports across Unix (`pgrep`/`lsof`/`ss`) and Windows (`powershell`/`netstat`).
- Implement `QuotaService` with loopback TLS certificate bypass, header authorization, thread-safe cache invalidation, and single retry logic.

**Non-Goals:**
- Managing background worker child processes (deprecated legacy `ProcessManager` per ADR-006 is excluded).
- Handling Telegram bot polling or message routing (covered by bot packages).
- Spawning Antigravity GUI automatically (the launcher issues actionable warnings and commands).

## Decisions

### Decision 1: Dependency Injection via Structs vs Package Globals
- **Choice**: Provide both standard parameterless public API functions (`AcquireLock()`, `GetAntigravityCliPath()`, `EnsureAntigravityRunning()`, `GetLanguageServerProcessInfo()`) and dependency-injected variants (`AcquireLockWithDeps()`, `GetAntigravityCliPathWithDeps()`, and functional options / runner arguments).
- **Rationale**: Keeps production entrypoints simple while enabling 100% deterministic, hermetic unit tests for filesystem, PID killing, sleep intervals, and external command execution without needing live system mutations or root permissions.
- **Alternatives Considered**: Mocking package-level global variables during tests (prone to race conditions when tests run in parallel).

### Decision 2: Cross-Platform Process Discovery Separation
- **Choice**: Separate OS command execution using Go build tags:
  - `proc_unix.go`: tagged `//go:build darwin || linux`, using `pgrep`, `/proc`, `lsof`, `ss`.
  - `proc_windows.go`: tagged `//go:build windows`, using PowerShell `Get-CimInstance` / `wmic` and `netstat -ano`.
  - Common data types, `QuotaService`, and universal text parser functions (`ParseUnixProcessInfo`, `ParseLsofListeningPorts`, `ParseSSListeningPorts`, `ParseWindowsProcessInfo`, `ParseNetstatListeningPorts`) located in `proc.go` without build tags.
- **Rationale**: Isolates platform-specific system calls while allowing all parsing logic to be tested and validated across all platforms (e.g. testing Windows parsing on macOS development machines and CI).
- **Alternatives Considered**: Using a monolithic file with runtime `runtime.GOOS` switches (leads to compilation errors if platform-specific syscalls or headers are referenced).

### Decision 3: Parallel CDP Probing with Fixed Slice Writing
- **Choice**: Use a slice `results := make([]bool, len(CDPPorts))` where goroutine `i` writes only to `results[i]`, coordinated with `sync.WaitGroup`.
- **Rationale**: Eliminates synchronization mutexes or channel buffering overhead while guaranteeing that the port evaluation order matches the deterministic preference order (`indexOf(true)`).
- **Alternatives Considered**: Early exit via buffered channel (acceptable, but checking all 6 ports in parallel completes in <2s and ensures all goroutines cleanly terminate before proceeding).

### Decision 4: QuotaService Thread-Safety and Caching
- **Choice**: Protect `cachedPort`, `cachedPid`, and `cachedCsrfToken` with a `sync.Mutex`. If a cached request fails, clear `cachedPort` and retry once with fresh port discovery.
- **Rationale**: Prevents data races when multiple Telegram commands (e.g. `/status` in different topics) arrive concurrently.
- **Alternatives Considered**: Lock-free atomic pointers (more complex with multiple correlated state variables: port, pid, and token).

## Risks / Trade-offs

- **[Risk]** Process termination in tests could accidentally send signals to host processes.
  - **Mitigation**: `LockfileDeps` abstracts `KillProcess`, allowing tests to record and verify signals without issuing real `os.Kill` or `syscall.Kill`.
- **[Risk]** Windows PowerShell command execution overhead during quota checks.
  - **Mitigation**: Successful discovery caches the working port and PID, so PowerShell/netstat is only invoked upon startup or when Antigravity restarts.
- **[Risk]** Loopback HTTPS self-signed certificate rejection.
  - **Mitigation**: Dedicated `http.Transport` with `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}` scoped strictly to loopback calls `127.0.0.1`.

## Migration Plan

- The `internal/platform` package is newly created and will be integrated into the CLI entrypoint (`cmd/antigravity-bot`), the bot startup sequence, and the quota commands in subsequent migration waves.
- No rollback or database schema migration required.

## Open Questions

- None. All behavior directly matches `src/utils/lockfile.ts`, `src/utils/pathUtils.ts`, `src/services/antigravityLauncher.ts`, `src/utils/cdpPorts.ts`, and `src/services/quotaService.ts`.
