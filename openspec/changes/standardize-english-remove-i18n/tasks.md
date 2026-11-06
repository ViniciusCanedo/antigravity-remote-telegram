## 1. Remove i18n Dependencies from Callers

- [x] 1.1 Replace `i18n.T(...)` calls with static English strings in `internal/bridge/mode_service.go`
- [x] 1.2 Replace `i18n.T(...)` calls with static English strings in `internal/bridge/auto_accept.go`
- [x] 1.3 Replace `i18n.T(...)` calls with static English strings in `internal/bridge/model_service.go`
- [x] 1.4 Replace `i18n.T(...)` calls with static English strings in `internal/bot/handlers_template.go`
- [x] 1.5 Update `internal/bot/handlers_template_test.go` and `internal/bot/handlers_messages_test.go` to remove `i18n` imports and assert English strings

## 2. Delete Internationalization Module

- [x] 2.1 Delete `internal/i18n/` directory completely (`i18n.go`, `i18n_test.go`, and `locales/*.json`)
- [x] 2.2 Verify no broken imports or references remain across the repository

## 3. Standardize CLI Texts to English

- [x] 3.1 Standardize `internal/cli/setup.go` (step headers, instructions, prompts, validations, confirmations, completion) to English
- [x] 3.2 Update `internal/cli/logo.go` to remove the author line (`QUAN NGUYEN - TNM`) and update the subtitle banner to `~ Antigravity Telegram Remote Setup ~`
- [x] 3.3 Standardize `internal/cli/root.go` short descriptions and persistent flag descriptions to English
- [x] 3.4 Update `internal/cli/setup_test.go` and `internal/cli/root_test.go` to assert the English strings

## 4. Standardize Telegram Bot UI & Formatter to English

- [x] 4.1 Update `BotCommands` menu descriptions in `internal/bot/bot.go` to standard English
- [x] 4.2 Standardize `internal/bot/ui/project_ui.go` texts ("📁 Projects", "No projects found.", pagination, buttons "◀ Prev" and "Next ▶")
- [x] 4.3 Update `internal/bot/ui/project_ui_test.go` to assert the English strings
- [x] 4.4 Standardize `internal/bot/ui/session_ui.go` texts ("🔗 Attach Session", "No sessions found in Antigravity control panel.", etc.)
- [x] 4.5 Update `internal/bot/ui/session_ui_test.go` to assert the English strings
- [x] 4.6 Standardize `internal/formatter/stream_formatter.go` status headers and truncation prefix `"...(earlier output truncated)\n"`
- [x] 4.7 Update `internal/formatter/stream_formatter_test.go` to assert the English strings

## 5. Verification & Testing

- [x] 5.1 Run `go test ./...` across all packages and ensure all tests pass cleanly
- [x] 5.2 Build the binary with `go build -o antigravity-telegram-remote ./cmd/antigravity-bot` to verify compilation
