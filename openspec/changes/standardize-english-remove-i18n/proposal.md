## Why

The current setup wizard CLI and Telegram bot interfaces display user-facing texts in Vietnamese rather than English, with an embedded internationalization module (`internal/i18n`) defaulting to Vietnamese. Standardizing all user-facing texts to English and removing the unused i18n subsystem simplifies the codebase, avoids runtime translation overhead, and provides a uniform, professional English experience for users on both terminal and Telegram platforms.

## What Changes

- **CLI Setup & Commands Standardization**:
  - Replace all Vietnamese strings in `internal/cli/setup.go` (steps, hints, validations, errors, success messages) with standard English.
  - Update `internal/cli/logo.go` to remove the author credit line (`QUAN NGUYEN - TNM`) and standardize the setup banner subtitle to English (`~ Antigravity Telegram Remote Setup ~`).
  - Standardize all command and flag short descriptions in `internal/cli/root.go` to English.
- **Telegram Bot UI Standardization**:
  - Update `BotCommands` menu descriptions in `internal/bot/bot.go` to English.
  - Standardize `internal/bot/ui/project_ui.go` and `internal/bot/ui/session_ui.go` texts (titles, status, pagination, buttons) to English.
  - Standardize `internal/formatter/stream_formatter.go` status lines ("Current mode", "Model", etc.) and truncation prefix to English (`"...(earlier output truncated)\n"`).
- **Removal of Internationalization (`internal/i18n`)**:
  - Delete `internal/i18n/` entirely (`i18n.go`, `i18n_test.go`, and `locales/` directory containing `en.json`, `ja.json`, `vi.json`).
  - Update all caller packages (`internal/bridge/mode_service.go`, `internal/bridge/auto_accept.go`, `internal/bridge/model_service.go`, `internal/bot/handlers_template.go`) to use direct English strings without `i18n.T(...)`.
- **Test Suite Updates**:
  - Update CLI, formatter, and bot tests to assert English strings.
  - Clean up any test dependencies on `internal/i18n`.

## Capabilities

### Modified Capabilities
- `config-logger-i18n`: Remove `internal/i18n` package requirements, keeping only `config` and `logger`.
- `cli-entrypoint`: Standardize setup wizard and CLI command descriptions to English.
- `telegram-ui`: Standardize project and session picker UI text to English.
- `formatters-log-buffers`: Standardize stream formatter mode/model headers and truncation prefixes to English.
- `bot-core`: Standardize `BotCommands` descriptions to English.

## Impact

- `internal/i18n` is completely removed.
- `internal/cli/`, `internal/bot/`, `internal/bridge/`, `internal/formatter/` code and unit tests updated.
- No external dependency changes in `go.mod`.
