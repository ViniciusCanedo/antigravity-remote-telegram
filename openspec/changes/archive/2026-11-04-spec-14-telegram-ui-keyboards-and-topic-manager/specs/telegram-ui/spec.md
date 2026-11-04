## ADDED Requirements

### Requirement: Telegram UI Common Keyboards and Builder
The `internal/bot/ui` package SHALL provide fluent constructors for Telegram inline keyboards, HTML character escaping, and models conversion.

#### Scenario: Fluent keyboard builder chaining
- **WHEN** `NewInlineKeyboard()` is created and `Text(label, callbackData)` and `Row()` are chained
- **THEN** it produces an `InlineKeyboardMarkup` with the expected button layout and JSON serialization

#### Scenario: Conversion to go-telegram/bot models
- **WHEN** `ToTelegramModel()` is called on an `InlineKeyboardMarkup`
- **THEN** it converts into `*models.InlineKeyboardMarkup` maintaining row and button structure

#### Scenario: HTML character escaping
- **WHEN** `EscapeHTML` is invoked with strings containing `&`, `<`, or `>`
- **THEN** it replaces them with `&amp;`, `&lt;`, and `&gt;`

### Requirement: Auto-Accept UI
The `internal/bot/ui` package SHALL construct and dispatch the auto-accept management message and keyboard based on current status.

#### Scenario: Disabled auto-accept state
- **WHEN** `autoAcceptService.IsEnabled()` is false
- **THEN** the message shows `⚪ OFF`, the ON button text is `Turn ON`, and the OFF button text is `🔴 Turn OFF`

#### Scenario: Enabled auto-accept state
- **WHEN** `autoAcceptService.IsEnabled()` is true
- **THEN** the message shows `🟢 ON`, the ON button text is `✅ Turn ON`, and the OFF button text is `Turn OFF`

### Requirement: Mode UI Management and Live CDP Sync
The `internal/bot/ui` package SHALL construct the mode management interface and synchronize execution mode with CDP when available.

#### Scenario: Render mode management
- **WHEN** `BuildModeUI` is called
- **THEN** it formats current mode, mode descriptions, and generates buttons for `fast` and `plan` modes

#### Scenario: Live mode sync from CDP
- **WHEN** `deps.GetCurrentCdp` is provided and returns a non-empty mode
- **THEN** `modeService.SetMode` is called with the live mode before rendering

### Requirement: Models UI Quota Inspection and Selection
The `internal/bot/ui` package SHALL retrieve available models and quota statuses, format countdown timers and exhaustion indicators, and cap keyboard buttons at 24.

#### Scenario: Not connected or empty models error
- **WHEN** CDP is not connected or returns an empty model list
- **THEN** `SendModelsUI` dispatches descriptive error messages with an empty keyboard

#### Scenario: Exhausted model representation
- **WHEN** a model has `remainingFraction <= 0`
- **THEN** it is marked with `⛔`, displays `Exhausted`, and generates callback data prefixed with `model_exhausted_`

#### Scenario: Healthy model quota percentage
- **WHEN** a model has fractional remaining quota
- **THEN** it displays percentage, appropriate status icon (`🟢`, `🟡`, `🔴`), and countdown time to quota reset

### Requirement: Plan UI Notification, Markdown Conversion, and Pagination
The `internal/bot/ui` package SHALL convert plan Markdown to Telegram HTML, build plan notification cards, and paginate long plans across multiple pages.

#### Scenario: Plan notification card
- **WHEN** `BuildPlanNotificationUI` is called with `PlanningInfo`
- **THEN** it displays plan title, workspace, description fallback, and 4 action buttons

#### Scenario: Plan markdown conversion
- **WHEN** `MarkdownToTelegramHTML` is executed
- **THEN** it converts headings to `<b>`, bold to `<b>`, italic to `<i>`, inline code to `<code>`, code blocks to `<pre>`, and list markers to bullets

#### Scenario: Plan content pagination
- **WHEN** `PaginatePlanContent` is invoked with content exceeding `pageSize`
- **THEN** it splits content at newline boundaries or hard splits, preserving all text

### Requirement: Project and Workspace Selection UI
The `internal/bot/ui` package SHALL render paginated workspace selection menus, clamp out-of-range pages, and validate project callback IDs.

#### Scenario: Empty project list
- **WHEN** workspaces slice is empty
- **THEN** it returns the empty state message and an empty keyboard

#### Scenario: Paginated workspace list and clamping
- **WHEN** workspaces count exceeds 100
- **THEN** it displays page info, clamps negative or overflow pages, and provides `◀ Trước` and `Tiếp ▶` navigation buttons

### Requirement: Screenshot Capture and Transmission UI
The `internal/bot/ui` package SHALL capture and send screenshots, falling back to descriptive text errors when unavailable.

#### Scenario: Successful screenshot capture
- **WHEN** `capturer.Capture` returns valid image buffer
- **THEN** it dispatches a photo named `screenshot.png` with caption `📸 Screenshot`

#### Scenario: Error handling
- **WHEN** capturer is nil, capture fails, or an error occurs
- **THEN** it dispatches an explanatory text error via `sendText`

### Requirement: Session Picker UI
The `internal/bot/ui` package SHALL build an interactive session picker menu with active session badges, capped at 25 items.

#### Scenario: Active session indicator and truncation
- **WHEN** sessions are provided
- **THEN** active session is prefixed with `✅ `, titles are truncated, and buttons are capped at 25

### Requirement: Template Management UI
The `internal/bot/ui` package SHALL render registered prompt templates with 60-character prompt previews and a 25-button cap.

#### Scenario: Template listing and overflow
- **WHEN** more than 25 templates exist
- **THEN** 25 buttons are generated, prompts are truncated to 60 characters, and hidden template count is displayed in the footer

### Requirement: Forum Topic Manager Lifecycle and Sanitization
The `internal/bot` package SHALL manage Telegram supergroup forum topics with thread-safe caching, name sanitization, and graceful error handling.

#### Scenario: Workspace and session topic naming
- **WHEN** `EnsureTopic` or `CreateSessionTopic` is called
- **THEN** workspace topics are prefixed with `🗂️ ` and session topics with `💬 `

#### Scenario: Topic caching
- **WHEN** `EnsureTopic` is called for an already created workspace
- **THEN** it returns the cached `topicID` without invoking Telegram API

#### Scenario: Name sanitization
- **WHEN** `SanitizeName` is provided arbitrary strings
- **THEN** it strips trailing slashes, converts internal slashes to hyphens, collapses consecutive hyphens, trims edge hyphens, and truncates to 128 characters
