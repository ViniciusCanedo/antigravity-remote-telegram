## 1. Platform Lockfile Implementation

- [x] 1.1 Implement `DefaultLockFilePath`, `AcquireLock`, and `AcquireLockWithDeps` in `internal/platform/lockfile.go`
- [x] 1.2 Implement graceful process termination with SIGTERM, 50ms polling up to 5000ms, and SIGKILL fallback
- [x] 1.3 Implement PID verification and automatic release hooks on process exit
- [x] 1.4 Write comprehensive unit tests for lockfile lifecycle in `internal/platform/lockfile_test.go`

## 2. Paths and Hint Resolution Implementation

- [x] 2.1 Implement `GetAntigravityCliPath` with `ANTIGRAVITY_PATH` file validation and OS defaults in `internal/platform/paths.go`
- [x] 2.2 Implement `ExtractProjectNameFromPath` supporting POSIX and Windows delimiters
- [x] 2.3 Implement `GetAntigravityCdpHint` and `GetAntigravityCdpHintForOS`
- [x] 2.4 Write unit tests covering all path scenarios and platforms in `internal/platform/paths_test.go`

## 3. CDP Launcher Port Verification Implementation

- [x] 3.1 Define `CDPPorts` and implement `CheckPort` with JSON array validation in `internal/platform/launcher.go`
- [x] 3.2 Implement parallel port verification and 70-column warning banner in `EnsureAntigravityRunning`
- [x] 3.3 Write unit tests for successful detection and warning banner output in `internal/platform/launcher_test.go`

## 4. Process Discovery and Quota Service Implementation

- [x] 4.1 Define models, `ProcessFinder`, and parsing functions (`ParseUnixProcessInfo`, `ParseLsofListeningPorts`, `ParseSSListeningPorts`, `ParseWindowsProcessInfo`, `ParseNetstatListeningPorts`) in `internal/platform/proc.go`
- [x] 4.2 Implement Unix process and port discovery in `internal/platform/proc_unix.go`
- [x] 4.3 Implement Windows process and port discovery in `internal/platform/proc_windows.go`
- [x] 4.4 Implement `QuotaService` with TLS bypass, header injection, caching, and retry in `internal/platform/proc.go`
- [x] 4.5 Write unit tests for parsers, command runners, and QuotaService in `internal/platform/proc_test.go`

## 5. Quality Gate and Verification

- [x] 5.1 Run `go test -v -race ./...` to verify zero data races and 100% test pass
- [x] 5.2 Run `go vet ./...` to ensure clean static analysis
- [x] 5.3 Run `gofmt -w .` and verify `gofmt -l .` reports no formatting discrepancies
- [x] 5.4 Run `go build ./...` to confirm successful package compilation
