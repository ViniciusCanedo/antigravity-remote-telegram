## Why

The Antigravity Telegram Remote project requires a foundational runtime layer providing structured console logging with ANSI styling, robust configuration loading with multi-source precedence, and embedded multi-language internationalization (i18n). Migrating these foundational utilities from TypeScript (`src/utils/config.ts`, `src/utils/configLoader.ts`, `src/utils/logger.ts`, `src/utils/i18n.ts`, `locales/*.json`) into native Go packages (`internal/logger`, `internal/config`, `internal/i18n`) eliminates Node.js dependencies while providing thread-safety, zero-allocation embedding, and 100% behavioral fidelity.

## What Changes

- Create `internal/logger`:
  - Thread-safe ANSI console logger implementing `Debug`, `Info`, `Warn`, `Error`, `Phase`, `Done`, and `Divider`.
  - Log level filtering (`debug` < `info` < `warn` < `error` < `none`).
  - Pluggable `ConsoleWriters` allowing stdout/stderr capture and deterministic time mock in unit tests.
  - Global `Default` logger and package-level delegate functions.
- Create `internal/config`:
  - Definition of runtime `AppConfig` and partial `PersistedConfig`.
  - `Loader` struct with customizable filesystem and environment hooks for testing.
  - Resolution precedence: `Environment Variables > ~/.antigravity-telegram-remote/config.json > Defaults`.
  - Validation rules: numeric Telegram user IDs (`^\d+$`), required tokens and IDs, tilde expansion (`~` and `~/...`).
  - Configuration persistence: atomic creation of config directory and 2-space indented JSON serialization.
  - Integration with `godotenv` for `.env` loading.
- Create `internal/i18n`:
  - Embedded JSON translation files for English (`en`), Japanese (`ja`), and Vietnamese (`vi`) using `//go:embed locales/*.json`.
  - Thread-safe `Translator` with default language `vi`, fallback to `en`, and ultimate fallback to the lookup key.
  - Literal `{{var}}` variable interpolation using `strings.ReplaceAll` avoiding regex overhead and character escaping issues.
  - Injected reader support (`InitI18nWithReader`) for unit test isolation.

## Capabilities

### New Capabilities
- `config-logger-i18n`: Configuration loading and persistence, ANSI structured logging, and embedded multi-language translation engine.

### Modified Capabilities

## Impact

- Provides foundational infrastructure used by all subsequent migration waves (CDP client, Telegram bot handlers, SQLite repositories, CLI commands).
- Pure Go implementation without CGO or external system dependencies.
- Zero runtime filesystem dependency for locales due to Go's `embed` package.
