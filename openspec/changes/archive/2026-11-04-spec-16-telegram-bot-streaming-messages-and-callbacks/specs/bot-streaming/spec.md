## ADDED Requirements

### Requirement: Serial Task Queue
The `internal/bot` package SHALL provide `SerialTaskQueue` to sequentially execute tasks in strict FIFO order per category name and trace ID.

#### Scenario: Sequential task execution
- **WHEN** multiple asynchronous tasks are enqueued to a `SerialTaskQueue`
- **THEN** each task executes only after the previous task has completed

#### Scenario: Fault tolerance
- **WHEN** an enqueued task returns an error
- **THEN** the error is logged and subsequent enqueued tasks continue to execute

### Requirement: HTML Processing Utilities
The `internal/bot` package SHALL provide `BalanceHTMLTags`, `StripHTMLForFile`, `StripHTML`, `IsJunkEntry`, and `FormatActivityLine`.

#### Scenario: Balance HTML unclosed tags
- **WHEN** an HTML string has unclosed start tags (e.g., `<b><i>hello`)
- **THEN** `BalanceHTMLTags` closes open tags in reverse order (`<b><i>hello</i></b>`)

#### Scenario: Balance HTML stray closing tags
- **WHEN** an HTML string contains orphan closing tags without opening tags (e.g., `hello</i></b> world`)
- **THEN** `BalanceHTMLTags` discards the orphan closing tags

#### Scenario: Strip HTML for Markdown file attachment
- **WHEN** Telegram HTML is converted via `StripHTMLForFile`
- **THEN** `<pre><code class="language-go">...</code></pre>` becomes fenced markdown code blocks, `<b>` becomes `**`, `<i>` becomes `*`, `<s>` becomes `~~`, `<a>` becomes `[text](url)`, and `<blockquote>` has each line prefixed with `> `

#### Scenario: Activity line formatting and junk filtering
- **WHEN** an activity entry has length `< 5`, is purely digits, or is a single word `< 8` chars
- **THEN** `IsJunkEntry` returns true
- **WHEN** a valid activity string is passed to `FormatActivityLine`
- **THEN** multiple spaces and newlines are collapsed to a single space, length is truncated to 120 characters, and HTML is escaped

### Requirement: Streaming Response Pipeline
The `internal/bot` package SHALL provide `SendPromptToAntigravity` executing the complete cycle of prompt injection, `ResponseMonitor` tracking, and chunked delivery.

#### Scenario: Disconnected CDP
- **WHEN** `cdp.IsConnected()` is false
- **THEN** `SendPromptToAntigravity` sends a Connection Error message and exits without injecting the prompt

#### Scenario: Prompt and image injection
- **WHEN** inbound images are provided
- **THEN** it attempts to inject using image file paths, falling back to URL references on failure

#### Scenario: Quota limit detected
- **WHEN** the monitor detects phase `quotaReached` or `quotaDetected`
- **THEN** it updates the activity message and response message with `⚠️ Quota Reached` and sends the model picker UI

#### Scenario: Collapsible thinking block
- **WHEN** thinking blocks are extracted from the DOM upon generation complete
- **THEN** they are formatted and sent enclosed in `<blockquote expandable>`

#### Scenario: Chunked response delivery
- **WHEN** response body exceeds 4096 characters
- **THEN** it splits the formatted HTML into chunks up to 5 parts, and attaches `response-<timestamp>.md` if chunks exceed 5

### Requirement: Callback Query Router
The `internal/bot` package SHALL provide `HandleCallbackQuery` routing all 11 families of callback data.

#### Scenario: Mode selection callback
- **WHEN** callback data starts with `mode_select:<mode>`
- **THEN** it sets the mode on `modeService` and CDP, edits the message UI, and answers the callback with the mode name

#### Scenario: Model exhausted callback
- **WHEN** callback data starts with `model_exhausted_<model>`
- **THEN** it answers the callback query with `show_alert: true` and the model exhausted message

#### Scenario: Approval action callback
- **WHEN** callback data is an approval action (`approve_action`, `always_allow_action`, `deny_action`)
- **THEN** it executes the action on the approval detector and removes the inline keyboard on success

#### Scenario: Planning action callback
- **WHEN** callback data is a planning action (`open`, `proceed`, `edit`, `refresh`, `page`)
- **THEN** it triggers the corresponding detector operation or sets plan edit pending state

#### Scenario: Error popup action callback
- **WHEN** callback data is an error popup action (`dismiss`, `copy_debug`, `retry`)
- **THEN** it executes the detector action and answers callback query

#### Scenario: Cleanup action callback
- **WHEN** callback data is `cleanup_archive:<days>`, `cleanup_delete:<days>`, or `cleanup_cancel`
- **THEN** it executes the cleanup operation or cancels, updating message text accordingly

### Requirement: Message Handlers and Passive Mirroring
The `internal/bot` package SHALL provide `HandleTextMessage`, `HandlePhotoMessage`, `HandleVoiceMessage`, and `StartPassiveMirrorForUserMessage`.

#### Scenario: Plan edit cancellation
- **WHEN** a channel is in `planEditPendingChannels` and the message text is `/cancel`
- **THEN** pending state is cleared and reply confirms cancellation

#### Scenario: Plan edit submission
- **WHEN** a channel is in `planEditPendingChannels` and non-cancel text is received
- **THEN** pending state is cleared and feedback prompt is dispatched to Antigravity

#### Scenario: Photo message processing
- **WHEN** a photo message is received
- **THEN** the largest photo is downloaded, dispatched to prompt dispatcher, and temporary files cleaned up

#### Scenario: Voice message transcription
- **WHEN** a voice message is received
- **THEN** it verifies Whisper, downloads the audio, transcribes it, and dispatches the prompt or executes the slash command

#### Scenario: Passive PC message mirroring
- **WHEN** a user types a message directly on the PC
- **THEN** `StartPassiveMirrorForUserMessage` sends `🖥️ Sent from PC:\n\n<text>` to Telegram and initiates `ResponseMonitor.StartPassive()`
