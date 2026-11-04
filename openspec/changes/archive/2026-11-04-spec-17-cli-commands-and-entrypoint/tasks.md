## 1. ASCII Banners & Welcome Box

- [x] 1.1 Implement `internal/cli/logo.go` with `Logo` and `SetupLogo` ASCII constants
- [x] 1.2 Implement `internal/cli/welcome.go` with `VisibleLength`, `FormatWelcomeBox`, and `PrintWelcome`
- [x] 1.3 Implement unit tests in `internal/cli/welcome_test.go` verifying ANSI stripping and line widths

## 2. Update Checker Service

- [x] 2.1 Implement `internal/cli/update_check.go` with `ShouldCheckForUpdates`, `FetchLatestVersion`, `CompareSemver`, and `CheckForUpdates`
- [x] 2.2 Implement unit tests in `internal/cli/update_check_test.go` mirroring `tests/services/updateCheckService.test.ts` (1:1 parity)

## 3. Subcommands Implementation

- [x] 3.1 Implement `internal/cli/start.go` and `internal/cli/start_test.go` (`StartAction`, `StartDeps`)
- [x] 3.2 Implement `internal/cli/setup.go` and `internal/cli/setup_test.go` (interactive wizard, validation, tilde expansion, completer)
- [x] 3.3 Implement `internal/cli/open.go` and `internal/cli/open_test.go` (`IsPortAvailable`, `FindAvailablePort`, platform launchers)
- [x] 3.4 Implement `internal/cli/doctor.go` and `internal/cli/doctor_test.go` (7 diagnostic checks and `--selectors` routing)

## 4. Root Command & Entrypoint

- [x] 4.1 Implement `internal/cli/root.go` and `internal/cli/root_test.go` with Cobra command tree and auto-fallback
- [x] 4.2 Implement `cmd/antigravity-bot/main.go` calling `cli.Execute()`

## 5. Quality Gate & Archive

- [x] 5.1 Run full quality gate (`go test -v -race ./...`, `go vet ./...`, `gofmt -w .`, `go build ./...`)
- [x] 5.2 Sync spec deltas to `openspec/specs/cli-entrypoint/spec.md`
- [x] 5.3 Archive change with `openspec archive -y spec-17-cli-commands-and-entrypoint`
- [x] 5.4 Re-verify quality gate and final binary build (`go build -o antigravity-bot ./cmd/antigravity-bot`)
