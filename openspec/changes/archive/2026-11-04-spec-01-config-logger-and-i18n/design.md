## Context

The TypeScript codebase provides configuration management (`config.ts`, `configLoader.ts`), colored console logging (`logger.ts`), and localized message translation (`i18n.ts`). In Go, these utilities must be modular, highly performant, thread-safe for concurrent goroutines, and easily testable without global test interference.

## Goals / Non-Goals

**Goals:**
- Implement `internal/logger` with full ANSI color palette, 24h timestamp `[HH:MM:SS]`, level filtering (`debug`, `info`, `warn`, `error`, `none`), and helper tags (`[PHASE]`, `[DONE]`, `Divider`).
- Implement `internal/config` with `AppConfig`, `PersistedConfig`, path helpers (`~/.antigravity-telegram-remote`), tilde expansion, precedence merging (`env > file > defaults`), numeric user ID validation (`^\d+$`), and partial JSON persistence.
- Implement `internal/i18n` with embedded `locales/en.json`, `locales/ja.json`, `locales/vi.json`, thread-safe translation lookup, fallback mechanism (`current -> en -> key`), and double-curly-brace variable interpolation.
- Achieve 100% test coverage matching the original Jest test suites with `-race` enabled.

**Non-Goals:**
- Creating CLI commands or interactive prompts (handled in Spec 07).
- Initializing the SQLite database connection (handled in Spec 03).

## Decisions

- **Thread-Safety with `sync.RWMutex`**:
  - `Logger` and `Translator` state can be read concurrently from multiple goroutines while log level or active language is modified at runtime. `sync.RWMutex` protects internal state against data races.
- **Dependency Injection for Testability**:
  - `ConsoleWriters` in `internal/logger` allows injecting `io.Writer` targets (`bytes.Buffer`) and a fixed `NowFunc` to verify timestamps and ANSI output deterministically.
  - `Loader` struct in `internal/config` exposes hook functions (`HomeDirFunc`, `LookupEnv`, `ReadFile`, `WriteFile`, `StatExists`, `MkdirAll`), defaulting to standard library functions (`os.UserHomeDir`, `os.LookupEnv`, `os.ReadFile`, etc.) while enabling unit tests to simulate filesystems and environments without modifying actual user disks.
  - `InitI18nWithReader` in `internal/i18n` allows mocking translation dictionaries without altering embedded assets.
- **Embedding Locales via `//go:embed`**:
  - Embedding `locales/*.json` inside `internal/i18n` guarantees that compiled Go binaries are self-contained and do not require external asset folders at runtime.
- **Literal Variable Replacement**:
  - Uses `strings.ReplaceAll(text, "{{"+vKey+"}}", strVal)` rather than regular expressions. This ensures keys with dots (e.g. `{{file.name}}`) or special regex characters are treated as plain literals without escaping artifacts.
- **Numeric Telegram User ID Validation**:
  - User IDs in Telegram are integers. A regular expression `^\d+$` is applied to each trimmed user ID in both environment variables and persisted JSON to prevent invalid strings.

## Risks / Trade-offs

- [Risk] Environment variable leakage across parallel unit tests.
  - → [Mitigation] In tests, use localized instances of `Loader` with isolated `LookupEnv` maps, avoiding `os.Setenv` during concurrent execution.
- [Risk] Character width variations in `Divider(label)` for multi-byte Unicode runes.
  - → [Mitigation] Compute padding using rune counts (`utf8.RuneCountInString(label)`) so ANSI divider lines align cleanly regardless of character encoding.
