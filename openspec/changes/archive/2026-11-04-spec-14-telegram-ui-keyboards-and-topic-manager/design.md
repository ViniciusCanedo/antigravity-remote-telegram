# Design: Spec 14 — Telegram UI Keyboards and Topic Manager

## Architecture Overview

The `internal/bot/ui` and `internal/bot` packages implement the user presentation and forum topic management layers of the Antigravity Telegram Remote architecture.

```
                           +------------------------+
                           |  Telegram Bot Handlers |
                           +-----------+------------+
                                       |
                   +-------------------+-------------------+
                   |                                       |
                   v                                       v
         +--------------------+                 +--------------------+
         |  internal/bot/ui   |                 |   internal/bot     |
         |  (8 Keyboard UIs)  |                 | TelegramTopicMgr   |
         +---------+----------+                 +---------+----------+
                   |                                       |
       +-----------+-----------+                           |
       |           |           |                           |
       v           v           v                           v
   [CdpService] [ModeSvc] [AutoAcceptSvc]          [Telegram Bot API]
```

## Package Architecture & Components

### 1. Common UI Keyboard Types (`internal/bot/ui/types.go`)
- Provides `InlineKeyboardButton` and `InlineKeyboardMarkup` matching the Telegram Bot API schema.
- Fluent builder methods `NewInlineKeyboard()`, `Text(label, callbackData)`, and `Row()` mirror the ergonomics of Grammy's `InlineKeyboard`.
- Conversion method `ToTelegramModel() *models.InlineKeyboardMarkup` bridges to `github.com/go-telegram/bot/models`.
- Encapsulates `UIPayload` containing formatted HTML text and inline keyboard markup.
- Defines `SendUIFunc` callback signature for decoupling message dispatching from bot transports.
- Provides `EscapeHTML(text string) string` helper substituting `&`, `<`, and `>`.

### 2. Auto-Accept UI (`internal/bot/ui/autoaccept_ui.go`)
- Abstraction: `AutoAcceptStateProvider` with `IsEnabled() bool`.
- Format: HTML header, status emoji (`🟢 ON` or `⚪ OFF`), descriptions, and 2-row button grid (`autoaccept_btn_on`, `autoaccept_btn_off`, `autoaccept_btn_refresh`).

### 3. Mode UI (`internal/bot/ui/mode_ui.go`)
- Abstractions: `ModeCdpProvider` (`GetCurrentMode(ctx) (string, error)`), `ModeServiceProvider` (`GetCurrentMode() string`, `SetMode(string)`).
- Sync Flow: If `deps.GetCurrentCdp` is present and returns a non-nil provider, query `liveMode` via CDP and update `modeService.SetMode(liveMode)` prior to rendering.
- Available modes: `fast` ("Fast", Direct execution) and `plan` ("Planning", Deep research/collaborative).
- Callback format: `mode_select:<mode>`.

### 4. Models UI (`internal/bot/ui/models_ui.go`)
- Abstractions: `ModelsCdpProvider` (`GetUiModels(ctx)`, `GetCurrentModel(ctx)`).
- Quota Models: `QuotaInfo` (`RemainingFraction *float64`, `ResetTime string`), `ModelQuotaEntry`.
- Fuzzy Model Matching: Strips spaces, `-`, `_`, lowers case, and tests for exact matches or bidirectional substring inclusions against `label` and `model`.
- Quota Formatting: Computes percentage, selects status icon (`⛔` <= 0%, `🔴` <= 20%, `🟡` <= 50%, `🟢` > 50%), parses ISO reset time into countdown (`Ready`, `Xm`, `Xh Ym`).
- Limits: Capped at 24 model buttons (`MaxModelButtons`), label truncated to 40 chars, callback data safely truncated to 48 chars (staying strictly under Telegram's 64-byte callback_data ceiling).

### 5. Plan UI (`internal/bot/ui/plan_ui.go`)
- Markdown to HTML: Converts code blocks (`<pre>`), headings (`<b>`), bold (`<b>`), italic (`<i>`), inline code (`<code>`), and bulleted lists (`• `).
- Pagination: Splits text on newline boundaries into pages up to 3500 characters (`DefaultPageSize`), falling back to a hard split if no newline occurs.
- Notification Card: Displays title, workspace, description, and 4 action buttons (`plan_view_btn`, `plan_proceed_btn`, `plan_edit_btn`, `plan_refresh_btn`).
- Content Card: Shows current page text, `(X/Y)` indicator (if multi-page), navigation buttons (`plan_page:<p>:...`), and action buttons.

### 6. Project List UI (`internal/bot/ui/project_ui.go`)
- Workspace Pagination: 100 workspaces per page (`ProjectItemsPerPage`), page clamping between 0 and `totalPages - 1`.
- Vietnamese localization: Matches original string literals (`Không tìm thấy dự án nào`, `Trang X / Y`, `◀ Trước`, `Tiếp ▶`).
- Callback parsing: `ParseProjectPageID` validates `project_page:<page>` and `IsProjectSelectID` validates `project_select` and `workspace_select`.

### 7. Screenshot UI (`internal/bot/ui/screenshot_ui.go`)
- Abstractions: `ScreenshotCapturer` (`Capture(ctx, format) (ScreenshotResult, error)`), `SendPhotoFunc`, `SendTextFunc`.
- Error Handling: Checks for nil capturer (`"Not connected to Antigravity."`), capture failure (`"Screenshot failed: ..."`), or exceptions (`"Screenshot error: ..."`).
- Success: Sends image as `screenshot.png` with caption `"📸 Screenshot"`.

### 8. Session Picker UI (`internal/bot/ui/session_ui.go`)
- Abstraction: `SessionListItem` (`Title string`, `IsActive bool`).
- Limits: Caps display at 25 sessions (`MaxSessionSelectOptions`), title label truncated to 40 chars, callback data truncated to 49 chars.
- Active Indicator: Prefixes `✅ ` for currently active chat session.

### 9. Template UI (`internal/bot/ui/template_ui.go`)
- Data: `TemplateItem` (`ID int`, `Name string`, `Prompt string`, `CreatedAt string`).
- Parsing: `ParseTemplateButtonID` validates `template_btn_<id>`.
- Limits: Caps at 25 buttons (`MaxTemplateButtons`), truncates prompt previews in the message body to 60 chars (`MaxPromptPreviewLen`), displays hidden count footer (`"X templates are hidden."`).

### 10. Telegram Topic Manager (`internal/bot/topic_manager.go`)
- Abstraction: `ForumTopicAPI` (`CreateForumTopic`, `EditForumTopic`, `CloseForumTopic`, `DeleteForumTopic`).
- Prefixing: Workspace topics are prefixed with `"🗂️ "`, session topics with `"💬 "`.
- Sanitization: Removes trailing slashes, converts slashes to hyphens, collapses multiple hyphens, trims edge hyphens, and truncates to 128 characters.
- Concurrency: Synchronized with `sync.RWMutex` to ensure thread-safe reads and writes of `chatID` and `topicCache`.
- Error Isolation: `RenameTopic`, `CloseTopic`, and `DeleteTopic` log errors and suppress failures to prevent crashes during non-critical forum management operations.
