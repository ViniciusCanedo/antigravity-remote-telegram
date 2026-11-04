## 1. Setup & Asset Preparation

- [x] 1.1 Copy `locales/*.json` from root project to `internal/i18n/locales/`
- [x] 1.2 Verify embedded assets exist and match source dictionaries

## 2. Logger Package Implementation (`internal/logger`)

- [x] 2.1 Implement `internal/logger/logger.go` with ANSI colors, log levels, `Logger` interface, and helper tags
- [x] 2.2 Implement `internal/logger/logger_test.go` covering all levels, runtime level change, divider padding, tag formatting, and default arguments

## 3. Configuration Package Implementation (`internal/config`)

- [x] 3.1 Implement `internal/config/config.go` with `AppConfig`, `PersistedConfig`, enums, and `ResolveResponseDeliveryMode()`
- [x] 3.2 Implement `internal/config/loader.go` with path resolution, tilde expansion, precedence merging, numeric user ID regex, and JSON persistence
- [x] 3.3 Implement `internal/config/config_test.go` mirroring `tests/config.test.ts`
- [x] 3.4 Implement `internal/config/loader_test.go` mirroring `tests/utils/configLoader.test.ts`

## 4. Internationalization Package Implementation (`internal/i18n`)

- [x] 4.1 Implement `internal/i18n/i18n.go` with `//go:embed locales/*.json`, thread-safe `Translator`, fallback logic, and literal `{{var}}` replacement
- [x] 4.2 Implement `internal/i18n/i18n_test.go` mirroring `tests/utils/i18n.test.ts`

## 5. Verification & Quality Gates

- [x] 5.1 Run `go test -v -race ./...` and ensure 100% pass
- [x] 5.2 Run `go vet ./...`
- [x] 5.3 Run `gofmt -w .` and verify `gofmt -l .` produces no diff
- [x] 5.4 Run `go build ./...`
