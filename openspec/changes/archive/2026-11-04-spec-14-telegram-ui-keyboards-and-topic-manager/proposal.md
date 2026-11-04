# Proposal: Spec 14 — Telegram UI Keyboards and Topic Manager

## Why

The Antigravity Telegram Remote bot interacts with users through Telegram inline keyboards and Forum Topics. Porting from TypeScript (`src/ui/*.ts` and `src/services/telegramTopicManager.ts`) to Go provides:
1. `internal/bot/ui`: 8 modular inline keyboard builders and UI senders that format Telegram HTML messages and build action-oriented inline reply markups with strict callback data rules.
2. `internal/bot`: `TelegramTopicManager`, providing thread-safe forum topic creation, naming sanitization, topic ID caching, and lifecycle operations (rename, close, delete) for workspace and session isolation.

## What Changes

- Create `internal/bot/ui/types.go`:
  - `InlineKeyboardButton`, `InlineKeyboardMarkup`, `UIPayload`, and `SendUIFunc` contracts.
  - Fluent builder methods `NewInlineKeyboard()`, `Text(label, callbackData)`, `Row()`, and `ToTelegramModel()` for compatibility with `github.com/go-telegram/bot/models`.
  - HTML escaping helper `EscapeHTML(text)`.
- Create `internal/bot/ui/autoaccept_ui.go` and `internal/bot/ui/autoaccept_ui_test.go`:
  - `AutoAcceptStateProvider` interface.
  - `BuildAutoAcceptUI` and `SendAutoAcceptUI` with status badge (`🟢 ON` / `⚪ OFF`) and toggle buttons.
- Create `internal/bot/ui/mode_ui.go` and `internal/bot/ui/mode_ui_test.go`:
  - `ModeCdpProvider` and `ModeServiceProvider` interfaces.
  - `BuildModeUI` and `SendModeUI` with live CDP mode sync and mode selection buttons.
- Create `internal/bot/ui/models_ui.go` and `internal/bot/ui/models_ui_test.go`:
  - `QuotaInfo`, `ModelQuotaEntry`, and `ModelsCdpProvider` interfaces.
  - `BuildModelsUI` and `SendModelsUI` with quota matching, exhaustion indicators (`⛔`), and countdown timers.
- Create `internal/bot/ui/plan_ui.go` and `internal/bot/ui/plan_ui_test.go`:
  - `MarkdownToTelegramHTML` converter supporting headings, bold, italic, code blocks, and bullets.
  - `PaginatePlanContent` dividing plans on newline boundaries with hard-split fallbacks.
  - `BuildPlanNotificationUI` and `BuildPlanContentUI` with paginated navigation and plan action buttons.
- Create `internal/bot/ui/project_ui.go` and `internal/bot/ui/project_ui_test.go`:
  - `ParseProjectPageID` and `IsProjectSelectID`.
  - `BuildProjectListUI` with 100-item page clamping, Vietnamese localization, and navigation buttons.
- Create `internal/bot/ui/screenshot_ui.go` and `internal/bot/ui/screenshot_ui_test.go`:
  - `ScreenshotCapturer` interface, `SendPhotoFunc`, and `SendTextFunc`.
  - `HandleScreenshot` with photo dispatch and error handling.
- Create `internal/bot/ui/session_ui.go` and `internal/bot/ui/session_ui_test.go`:
  - `IsSessionSelectID`.
  - `BuildSessionPickerUI` capping at 25 sessions with active session indicators.
- Create `internal/bot/ui/template_ui.go` and `internal/bot/ui/template_ui_test.go`:
  - `ParseTemplateButtonID`.
  - `BuildTemplateUI` and `SendTemplateUI` capping at 25 buttons and truncating prompt previews to 60 chars.
- Create `internal/bot/topic_manager.go` and `internal/bot/topic_manager_test.go`:
  - `ForumTopicAPI` interface for abstracting Telegram Bot API topic calls.
  - `TelegramTopicManager` with thread-safe `EnsureTopic`, `CreateSessionTopic`, `RenameTopic`, `CloseTopic`, `DeleteTopic`, `SanitizeName`, and topic cache.

## Capabilities

### New Capabilities
- `telegram-ui`: Telegram inline keyboard constructors and forum topic manager for workspace and session lifecycle management.

### Modified Capabilities

## Impact

- Enables rich interactive Telegram message cards and menus for the Antigravity Telegram Remote bot.
- Isolates distinct workspaces and chat sessions into dedicated Telegram supergroup forum topics.
- Completely decouples UI constructors from bot frameworks via interfaces and clean abstractions.
