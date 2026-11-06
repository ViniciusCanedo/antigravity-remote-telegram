## Context

Antigravity Telegram Remote was originally ported from TypeScript with hardcoded Vietnamese strings in the setup CLI and Telegram UI, alongside an `internal/i18n` package that embedded English, Japanese, and Vietnamese JSON dictionaries but defaulted to Vietnamese (`LangVI`). Because no other language selection mechanism exists, the application has been displaying Vietnamese in the terminal setup wizard and in Telegram bot menus/headers.

## Goals / Non-Goals

**Goals:**
- Replace all Vietnamese strings in setup (`internal/cli/setup.go`), CLI commands (`internal/cli/root.go`, `internal/cli/logo.go`), Telegram bot UI (`internal/bot/bot.go`, `internal/bot/ui/project_ui.go`, `internal/bot/ui/session_ui.go`), and Telegram formatter (`internal/formatter/stream_formatter.go`) with clear, idiomatic English.
- Clean up `internal/cli/logo.go` by removing the author credit line (`QUAN NGUYEN - TNM`) and updating the setup banner subtitle to `~ Antigravity Telegram Remote Setup ~`.
- Completely remove the `internal/i18n` package and its embedded `locales/*.json` dictionaries.
- Replace all calls to `i18n.T(...)` across `internal/bridge/` and `internal/bot/` with static English strings or standard `fmt.Sprintf` calls.
- Update all associated unit tests to assert the standardized English strings.

**Non-Goals:**
- Retaining multilingual fallback or locale switching (all text will be natively English in code).
- Modifying DOM selectors or CDP logic that detects UI buttons in Antigravity.

## Decisions

### 1. Direct English String Literals
- **Decision**: Remove `i18n.T(...)` and provide static strings directly in source code.
- **Rationale**: Removes runtime dictionary lookup, reflection, sync mutex overhead, and embedded JSON assets while satisfying the requirement to maintain only English.
- **Alternatives Considered**: Keeping `i18n` with only `en.json`. Rejected because the user explicitly approved removing the internationalization module.

### 2. Stream Truncation Message
- **Decision**: Use `"...(earlier output truncated)\n"` as the truncation prefix in `FitForSingleEmbedDescription`.
- **Rationale**: Clear, professional, and directly conveys why text is missing without ambiguity.

### 3. Setup CLI Prompts and Validations
- **Decision**: Standardize prompt lines, step headers (`[Step 1/3]`), validation errors (e.g. `"Please enter at least one user ID."`, `"Invalid user ID: \"%s\" — must be numeric."`), and post-setup instructions (`"Setup completed!"`, `"Next steps:"`) to clean English.

## Risks / Trade-offs

- **[Risk]** Existing unit tests asserting Vietnamese text fail.
  → **Mitigation**: Update all affected unit tests in `internal/cli/setup_test.go`, `internal/cli/root_test.go`, `internal/bot/ui/project_ui_test.go`, `internal/bot/ui/session_ui_test.go`, `internal/formatter/stream_formatter_test.go`, `internal/bot/handlers_messages_test.go`, and `internal/bot/handlers_template_test.go`.
- **[Risk]** Breaking imports when deleting `internal/i18n`.
  → **Mitigation**: Grep entire repository and remove all references to `antigravity-telegram-remote/internal/i18n` before deleting the package. Verify with `go test ./...`.
