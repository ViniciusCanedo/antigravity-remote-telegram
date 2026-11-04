# bridge-manager Specification

## Purpose
TBD - created by archiving change spec-12-error-usermessage-detectors-and-bridge. Update Purpose after archive.
## Requirements
### Requirement: Error Popup Detection and Cooldown
The `internal/bridge` package SHALL detect error dialogs and modal overlays matching error patterns using `DetectErrorPopupScript`, deduplicating notifications using key `${title}::${body[:100]}` and enforcing a 10-second cooldown window.

#### Scenario: Error popup callback fired on detection
- **WHEN** an error dialog matching error patterns is detected in the DOM
- **THEN** `OnErrorPopup(info)` is invoked with title, body, and extracted button labels

#### Scenario: Consecutive duplicate detection ignored
- **WHEN** the same error popup remains visible across consecutive polling cycles
- **THEN** `OnErrorPopup(info)` is invoked only once

#### Scenario: Same title and body prefix treated as duplicate
- **WHEN** an error popup has identical title and first 100 characters of body, but different button labels
- **THEN** `OnErrorPopup(info)` is not invoked again and stored info is silently updated

#### Scenario: Cooldown window suppresses rapid re-detection
- **WHEN** an error popup disappears and reappears within 10 seconds of the previous notification
- **THEN** `OnErrorPopup(info)` is suppressed until the 10-second cooldown expires

#### Scenario: OnResolved callback fired when popup disappears
- **WHEN** an error popup disappears from the DOM after being detected
- **THEN** `OnResolved()` is invoked, and `lastNotifiedAt` is preserved for cooldown tracking

#### Scenario: Disappearance without prior detection ignored
- **WHEN** polling detects no error popup without prior detection
- **THEN** `OnResolved()` is NOT invoked

### Requirement: Error Popup Remote Actions and Clipboard
The `internal/bridge` package SHALL provide remote button click helpers (`ClickDismissButton`, `ClickCopyDebugInfoButton`, `ClickRetryButton`) and clipboard reading (`ReadClipboard`).

#### Scenario: Click Dismiss button via CDP
- **WHEN** `ClickDismissButton(ctx)` is called
- **THEN** it executes a click script searching for button text `"Dismiss"` and returns true on success

#### Scenario: Click Copy debug info button via CDP
- **WHEN** `ClickCopyDebugInfoButton(ctx)` is called
- **THEN** it executes a click script searching for button text `"Copy debug info"` and returns true on success

#### Scenario: Click Retry button via CDP
- **WHEN** `ClickRetryButton(ctx)` is called
- **THEN** it executes a click script searching for button text `"Retry"` and returns true on success

#### Scenario: Read clipboard returns text
- **WHEN** `ReadClipboard(ctx)` is called
- **THEN** it executes `ReadClipboardScript` with `awaitPromise: true` and returns the clipboard string

#### Scenario: Read clipboard returns nil on empty or error
- **WHEN** the clipboard is empty or a CDP evaluation error occurs
- **THEN** `ReadClipboard(ctx)` returns `nil` without panicking

### Requirement: Desktop User Message Detection and Priming
The `internal/bridge` package SHALL detect user chat messages posted directly in the Antigravity desktop UI using `DetectUserMessageScript`, priming on initial poll without notification, and deduplicating using a FIFO ring buffer of up to 50 hashes.

#### Scenario: Priming on initial poll without callback invocation
- **WHEN** `Start()` is called and an existing user message is in the DOM during the first poll
- **THEN** the message hash is recorded into `lastDetectedHash` and `seenHashes` without invoking `OnUserMessage`

#### Scenario: Priming with empty DOM
- **WHEN** `Start()` is called and the DOM contains no user messages during the first poll
- **THEN** priming completes and subsequent new messages trigger `OnUserMessage`

#### Scenario: Detection of new user messages after priming
- **WHEN** a new user message appears in the DOM after priming
- **THEN** `OnUserMessage(info)` is invoked with the message text

#### Scenario: Consecutive duplicate user messages ignored
- **WHEN** the same user message remains in the DOM across consecutive cycles
- **THEN** `OnUserMessage(info)` is not invoked again

#### Scenario: Old messages suppressed by seen hashes ring buffer
- **WHEN** the DOM reverts to an older message recorded in `seenHashes`
- **THEN** `OnUserMessage(info)` is not invoked

#### Scenario: Seen hashes cleared on detector restart
- **WHEN** a detector is stopped and restarted
- **THEN** `seenHashes` is cleared, allowing previously seen messages to be detected again after priming

### Requirement: Anti-Echo Hash Suppression
The `internal/bridge` package SHALL provide an anti-echo hash registration mechanism (`AddEchoHash`) with automatic 60-second expiration to prevent Telegram bot outbound messages from echoing back.

#### Scenario: Echo hashes skipped during polling
- **WHEN** `AddEchoHash(text)` is called prior to DOM detection
- **THEN** messages matching the echo hash are skipped without invoking `OnUserMessage`

#### Scenario: Echo hashes preserved across stop and start cycles
- **WHEN** a detector is stopped and restarted
- **THEN** `echoHashes` is NOT cleared and continues honoring its 60-second TTL

### Requirement: CDP Bridge State and Channel Routing
The `internal/bridge` package SHALL maintain central bridge state, resolve connected CDP instances, and route notifications by session title or workspace name.

#### Scenario: Initial bridge state
- **WHEN** `InitCdpBridge(autoApproveDefault, pool, quota, autoAccept)` is called
- **THEN** `LastActiveWorkspace` and `LastActiveChannel` are initialized to nil

#### Scenario: Resolution by session title when linked
- **WHEN** a channel is registered via `RegisterApprovalSessionChannel` for a specific session title
- **THEN** `ResolveApprovalChannelForCurrentChat` resolves to that channel when the current chat title matches (case-insensitive and trimmed)

#### Scenario: Fallback to workspace channel when session does not match or is nil
- **WHEN** no session channel matches the current chat title or current chat title is empty/nil
- **THEN** `ResolveApprovalChannelForCurrentChat` falls back to the workspace channel registered via `RegisterApprovalWorkspaceChannel`

#### Scenario: Preference for session channel over workspace channel
- **WHEN** both workspace and session channels are registered for a project
- **THEN** `ResolveApprovalChannelForCurrentChat` returns the session channel when the title matches

#### Scenario: Nil return when workspace is unregistered
- **WHEN** neither session nor workspace channel is registered for a project
- **THEN** `ResolveApprovalChannelForCurrentChat` returns nil

### Requirement: Action Custom ID Encoding and Parsing
The `internal/bridge` package SHALL build and parse Telegram callback data strings for approvals, planning, and error popups with backward compatibility for legacy custom IDs without channel IDs.

#### Scenario: Approval action custom ID round-trip and legacy parsing
- **WHEN** `BuildApprovalCustomID("approve", "ws", "123")` is called
- **THEN** it formats `"approve_action:ws:123"`, and `ParseApprovalCustomID` parses action, project name, and channel ID; legacy `"approve_action:ws"` parses with nil channel ID

#### Scenario: Planning action custom ID round-trip and legacy parsing
- **WHEN** `BuildPlanningCustomID("open", "ws", "123")` is called
- **THEN** it formats `"planning_open_action:ws:123"`, and `ParsePlanningCustomID` parses action, project name, and channel ID; invalid prefixes return nil

#### Scenario: Error popup custom ID round-trip and legacy parsing
- **WHEN** `BuildErrorPopupCustomID("dismiss", "ws", "123")` is called
- **THEN** it formats `"error_popup_dismiss_action:ws:123"`, and `ParseErrorPopupCustomID` parses action, project name, and channel ID; legacy `"error_popup_dismiss_action:ws"` parses with nil channel ID

### Requirement: Bridge Detector Orchestration
The `internal/bridge` package SHALL instantiate, start, register, and manage background detectors (`EnsureApprovalDetector`, `EnsurePlanningDetector`, `EnsureErrorPopupDetector`, `EnsureUserMessageDetector`), automatically clearing inline button keyboards on resolution.

#### Scenario: EnsureApprovalDetector starts and registers single instance
- **WHEN** `EnsureApprovalDetector` is called for a workspace
- **THEN** it creates and registers an `ApprovalDetector` if none is active; subsequent calls while active are no-ops

#### Scenario: EnsurePlanningDetector starts and registers single instance
- **WHEN** `EnsurePlanningDetector` is called for a workspace
- **THEN** it creates and registers a `PlanningDetector` if none is active; subsequent calls while active are no-ops

#### Scenario: EnsureErrorPopupDetector starts and registers single instance
- **WHEN** `EnsureErrorPopupDetector` is called for a workspace
- **THEN** it creates and registers an `ErrorPopupDetector` if none is active; subsequent calls while active are no-ops

#### Scenario: EnsureUserMessageDetector starts and registers single instance
- **WHEN** `EnsureUserMessageDetector` is called for a workspace
- **THEN** it creates and registers a `UserMessageDetector` if none is active; subsequent calls while active are no-ops

#### Scenario: Message reply markup removed on resolution callback
- **WHEN** a detector's `OnResolved` callback is invoked after a notification was sent
- **THEN** `ClearMessageReplyMarkup` is invoked for the last sent message and local message IDs are reset

