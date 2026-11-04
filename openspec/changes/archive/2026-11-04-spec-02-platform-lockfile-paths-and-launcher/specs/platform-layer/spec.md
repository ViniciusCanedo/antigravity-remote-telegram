## ADDED Requirements

### Requirement: Single-Instance Lockfile Lifecycle
The `internal/platform` package SHALL manage a process lockfile at `~/.antigravity-telegram-remote/.bot.lock` to ensure only one instance of the bot runs simultaneously, terminating any existing instance gracefully and cleaning up on exit.

#### Scenario: Acquiring lock when no lock exists
- **WHEN** `AcquireLock()` or `AcquireLockWithDeps()` is called and no lock file exists
- **THEN** the parent directory is created if missing, the current process PID is written to the lock file, and a release function is returned

#### Scenario: Cleaning up stale lock file from exited process
- **WHEN** a lock file exists containing a PID that is no longer running
- **THEN** a warning is logged indicating a stale lock file, the stale file is removed, and the new process PID is recorded

#### Scenario: Terminating running existing process
- **WHEN** a lock file exists with an active process PID differing from the current process
- **THEN** an info message is logged and `SIGTERM` is sent to the existing PID, polling every 50ms up to 5000ms until the process terminates; if it does not terminate within 5000ms, `SIGKILL` is sent before acquiring the lock

#### Scenario: Handling non-numeric lock file content gracefully
- **WHEN** a lock file exists containing non-numeric data
- **THEN** the invalid file is removed without panicking and the current PID is written

#### Scenario: Releasing lock on clean exit
- **WHEN** the returned release function is invoked and the lock file contains the matching current PID
- **THEN** the lock file is deleted and an info message is logged

#### Scenario: Releasing lock does not delete file if PID was overwritten
- **WHEN** the returned release function is invoked but the lock file contains a different PID
- **THEN** the lock file is preserved and not deleted

#### Scenario: Registration of exit handlers
- **WHEN** `AcquireLock()` is invoked
- **THEN** exit signal listeners (`SIGINT`, `SIGTERM`) are registered to release the lock on process shutdown

### Requirement: Antigravity CLI and Workspace Path Resolution
The `internal/platform` package SHALL provide functions to resolve the Antigravity CLI binary path safely, extract project workspace names, and provide platform-specific CDP launch hints.

#### Scenario: Resolving CLI path via valid ANTIGRAVITY_PATH environment variable
- **WHEN** `ANTIGRAVITY_PATH` is set in the environment and points to an existing regular file
- **THEN** `GetAntigravityCliPath()` returns the path without error

#### Scenario: Validating ANTIGRAVITY_PATH rejects directories
- **WHEN** `ANTIGRAVITY_PATH` points to a directory
- **THEN** `GetAntigravityCliPath()` returns an error containing `ANTIGRAVITY_PATH must be a file`

#### Scenario: Validating ANTIGRAVITY_PATH rejects non-existent paths
- **WHEN** `ANTIGRAVITY_PATH` points to a path that does not exist
- **THEN** `GetAntigravityCliPath()` returns an error containing `ANTIGRAVITY_PATH validation failed`

#### Scenario: Default CLI path resolution on macOS
- **WHEN** `ANTIGRAVITY_PATH` is unset and the platform is `darwin`
- **THEN** `GetAntigravityCliPath()` returns `/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity`

#### Scenario: Default CLI path resolution on Windows
- **WHEN** `ANTIGRAVITY_PATH` is unset, the platform is `windows`, and `LOCALAPPDATA` is set
- **THEN** `GetAntigravityCliPath()` returns `<LOCALAPPDATA>\Programs\Antigravity\Antigravity.exe` (or `Antigravity.exe` if `LOCALAPPDATA` is unset)

#### Scenario: Default CLI path resolution on Linux
- **WHEN** `ANTIGRAVITY_PATH` is unset and the platform is `linux`
- **THEN** `GetAntigravityCliPath()` returns `antigravity`

#### Scenario: Extracting project name from POSIX and Windows paths
- **WHEN** `ExtractProjectNameFromPath()` is called with paths using forward slashes, backslashes, mixed slashes, or trailing slashes
- **THEN** it returns the last non-empty segment, or empty string if input is empty

#### Scenario: Generating CDP launch hint by OS
- **WHEN** `GetAntigravityCdpHint()` or `GetAntigravityCdpHintForOS()` is called with an optional port
- **THEN** it formats the platform-specific launch command (`open -a Antigravity --args` for macOS, `Antigravity.exe` for Windows, `antigravity` for Linux) with `--remote-debugging-port=<port>` (defaulting to 9222)

### Requirement: CDP Launcher Port Verification
The `internal/platform` package SHALL probe Chrome DevTools Protocol (CDP) ports in parallel to determine if an Antigravity instance is reachable.

#### Scenario: Checking active CDP port
- **WHEN** `CheckPort(port)` is called and `http://127.0.0.1:<port>/json/list` returns HTTP 200 with a valid JSON array within 2000ms
- **THEN** it returns `true`

#### Scenario: Checking inactive or invalid CDP port
- **WHEN** `CheckPort(port)` encounters a network error, timeout, or non-array JSON response
- **THEN** it returns `false`

#### Scenario: Parallel verification succeeds when any port responds
- **WHEN** `EnsureAntigravityRunning()` is called and at least one port among `9222, 9223, 9333, 9444, 9555, 9666` responds
- **THEN** all 6 ports are queried in parallel, an OK debug message is logged for the first matching port, and no warning banner is emitted

#### Scenario: Parallel verification fails and emits 70-column warning banner
- **WHEN** `EnsureAntigravityRunning()` is called and all 6 ports fail to respond
- **THEN** all 6 ports are queried in parallel and a 70-column warning banner is logged via `logger.Warn` containing instructions and manual CDP command hints

### Requirement: Process Discovery and Quota Service
The `internal/platform` package SHALL discover the running `language_server` process and its active TCP listening ports across operating systems, and provide a thread-safe `QuotaService` to query model quotas with caching and retry.

#### Scenario: Parsing Unix process command line
- **WHEN** `ParseUnixProcessInfo()` processes output from `pgrep -fl language_server` containing a valid PID and `--csrf_token <token>`
- **THEN** it extracts the PID and token correctly into `LanguageServerProcessInfo`

#### Scenario: Parsing listening ports from lsof output
- **WHEN** `ParseLsofListeningPorts()` parses output from `lsof -nP -a -iTCP -sTCP:LISTEN -p <pid>`
- **THEN** it returns a deduplicated list of integers representing TCP ports in `LISTEN` state for that PID

#### Scenario: Parsing listening ports from ss output
- **WHEN** `ParseSSListeningPorts()` parses output from `ss -tlnp`
- **THEN** it returns a deduplicated list of TCP listening ports associated with the specified PID

#### Scenario: Parsing Windows process information
- **WHEN** `ParseWindowsProcessInfo()` parses JSON or CSV output from PowerShell/WMIC containing `language_server` and `--csrf_token`
- **THEN** it extracts the PID and CSRF token correctly

#### Scenario: Parsing listening ports from netstat output
- **WHEN** `ParseNetstatListeningPorts()` parses output from `netstat -ano -p tcp`
- **THEN** it returns a deduplicated list of TCP ports in `LISTENING` status for that PID

#### Scenario: Fetching quota with successful port discovery
- **WHEN** `QuotaService.FetchQuota()` is called without cached port
- **THEN** it discovers the process info and listening ports, queries the HTTPS endpoint with `X-Codeium-Csrf-Token`, caches the successful port, and returns the parsed `[]ModelQuota`

#### Scenario: Fetching quota uses cached port and retries on failure
- **WHEN** `QuotaService.FetchQuota()` is called with a cached port that subsequently fails
- **THEN** it clears the cached port, re-scans the listening ports on a single retry, and queries again
