# Design: Error and User Message Detectors & Bridge Manager (`internal/bridge`)

## Architecture & Data Flow

`CdpBridge` serves as the central coordination layer uniting the CDP connection pool, the background DOM detectors, quota management, auto-accept logic, and the Telegram bot messenger.

```
                    ┌───────────────────────────────┐
                    │      Antigravity Windows       │
                    │   (DOM Contexts & Overlays)   │
                    └───────────────▲───────────────┘
                                    │
                                    │ CDP Runtime.evaluate
                                    ▼
                    ┌───────────────────────────────┐
                    │     CdpServiceWithContexts    │
                    │     (BridgeConnectionPool)    │
                    └───────────────▲───────────────┘
                                    │
       ┌────────────────────────────┼────────────────────────────┐
       ▼                            ▼                            ▼
┌──────────────┐             ┌──────────────┐             ┌──────────────┐
│ Error Popup  │             │ User Message │             │   Approval   │
│   Detector   │             │   Detector   │             │  & Planning  │
└──────┬───────┘             └──────┬───────┘             └──────┬───────┘
       │ OnErrorPopup               │ OnUserMessage              │ Callbacks
       │ OnResolved                 │                            │ OnResolved
       └────────────────────────────┼────────────────────────────┘
                                    ▼
                    ┌───────────────────────────────┐
                    │          CdpBridge            │
                    │  - ApprovalChannelByWorkspace │
                    │  - ApprovalChannelBySession   │
                    │  - AutoAccept / Quota         │
                    └───────────────┬───────────────┘
                                    │ SendMessage / ClearMessageReplyMarkup
                                    ▼
                    ┌───────────────────────────────┐
                    │     TelegramBotMessenger      │
                    │   (Chat & Forum Topic IDs)    │
                    └───────────────────────────────┘
```

## Key Components

### 1. `ErrorPopupDetector` (`internal/bridge/error_detector.go`)

- **DOM Inspection (`DetectErrorPopupScript`)**:
  - Searches for dialog/modal elements (`[role="dialog"]`, `[role="alertdialog"]`, `.modal`, `.dialog`) that are visible or have `aria-modal="true"`.
  - Fallback overlay detection: searches for `div` with `fixed` or `absolute` positioning, `zIndex > 10`, and at least one `<button>`.
  - Filters elements matching error patterns: `"agent terminated"`, `"terminated due to error"`, `"unexpected error"`, `"something went wrong"`, `"an error occurred"`.
  - Extracts title via heading elements (`h1-h4, [class*="title"], [class*="heading"]`, defaulting to `"Error"`).
  - Extracts body using `TreeWalker` excluding title and button texts, truncated to 1000 characters.
  - Extracts visible button labels.
- **Deduplication & Cooldown**:
  - Key format: `${info.Title}::${info.Body[:100]}`.
  - Cooldown: 10 seconds (`ErrorPopupCooldown = 10 * time.Second`). Suppresses duplicate notifications if the popup reappears or is re-detected within 10 seconds, even after state reset.
- **Remote Actions & Clipboard**:
  - `ClickDismissButton`: clicks button with text `"Dismiss"`.
  - `ClickCopyDebugInfoButton`: clicks button with text `"Copy debug info"`.
  - `ClickRetryButton`: clicks button with text `"Retry"`.
  - `ReadClipboard`: executes `READ_CLIPBOARD_SCRIPT` with `awaitPromise: true` to read `navigator.clipboard.readText()`.

### 2. `UserMessageDetector` (`internal/bridge/user_message_detector.go`)

- **DOM Inspection (`DetectUserMessageScript`)**:
  - Primary Strategy: queries `[class*="bg-gray-500/15"][class*="select-text"] .whitespace-pre-wrap` inside `.antigravity-agent-side-panel`.
  - Fallback Strategy: finds individual non-nested bubble containers `[class*="bg-gray-500/15"][class*="rounded-lg"][class*="select-text"]` and extracts `.whitespace-pre-wrap` or `[style*="word-break"]`.
- **Normalization & Hashing**:
  - `NormalizeForHash`: trims text, collapses whitespace (`\s+` $\rightarrow$ `' '`), and takes the first 200 characters.
  - `ComputeEchoHash`: calculates SHA-256 over normalized text and returns the first 16 hex characters.
- **Priming & Echo Defense**:
  - Priming: on the initial poll cycle after `Start()`, existing messages in the DOM are recorded in `lastDetectedHash` and `seenHashes` without invoking `OnUserMessage`. Priming completes even if the DOM is empty.
  - Echo Hashes (`AddEchoHash`): messages sent by the Telegram bot are registered with a 60-second TTL (`echoTTL`) and ref-counted in `echoHashes`. They are not cleared on `Stop()`/`Start()` cycles.
  - Seen Hashes Ring Buffer: FIFO buffer holding up to 50 hashes (`MaxSeenHashes = 50`) prevents duplicate alerts if the DOM temporarily reverts to an older message.

### 3. `CdpBridgeManager` (`internal/bridge/manager.go`)

- **State Management (`CdpBridge`)**:
  - Holds `BridgeConnectionPool`, `QuotaFetcher`, `AutoAcceptChecker`, `TelegramBotMessenger`, routing maps, and builder callbacks.
  - Protected by `sync.RWMutex` for concurrent access from multiple detectors and bot command handlers.
- **Channel Routing**:
  - `ApprovalChannelByWorkspace`: maps workspace project name to Telegram chat/thread.
  - `ApprovalChannelBySession`: maps `${projectName}::${normalizeSessionTitle(sessionTitle)}` to Telegram chat/thread.
  - `ResolveApprovalChannelForCurrentChat`: prefers session channel matching `GetCurrentChatTitle(ctx, cdp)`; falls back to workspace channel.
- **Action Custom IDs**:
  - Format: `<action_prefix>:<project_name>[:<channel_id>]`.
  - Approval: `approve_action`, `always_allow_action`, `deny_action`.
  - Planning: `planning_open_action`, `planning_proceed_action`.
  - Error Popup: `error_popup_dismiss_action`, `error_popup_copy_debug_action`, `error_popup_retry_action`.
  - Supports backward-compatible parsing without channel ID (`<action_prefix>:<project_name>`) and exact prefix matches (`<action_prefix>`).
- **Detector Orchestration (`Ensure*Detector`)**:
  - Ensures exactly one active instance per project name registered in the connection pool.
  - Approvals: executes auto-accept if enabled (`AlwaysAllowButton` then `ApproveButton`); otherwise formats notification HTML and inline buttons.
  - Planning: delegates message layout to `PlanNotificationBuilder`.
  - Error Popups: formats error notification with title, truncated body (up to 3800 chars), button list, and action buttons (`Dismiss`, `Copy Debug`, `Retry`).
  - Tracks `lastMessageId` and `lastMessageChatId` per detector with local mutex synchronization; automatically clears reply markup via `ClearMessageReplyMarkup` when `OnResolved()` fires.

## Concurrency & Synchronization

1. `CdpBridge.mu`: Read-write lock protecting shared fields (`LastActiveWorkspace`, `LastActiveChannel`, `ApprovalChannelByWorkspace`, `ApprovalChannelBySession`).
2. `UserMessageDetector.mu`: Protects `isRunning`, `isPriming`, `lastDetectedHash`, `echoHashes`, `seenHashesOrder`, and `seenHashesSet`. `OnUserMessage` callback is executed outside lock.
3. `ErrorPopupDetector.mu`: Protects polling loop state, `isRunning`, `lastDetectedKey`, `lastDetectedInfo`, and `lastNotifiedAt`. `OnErrorPopup` and `OnResolved` callbacks are executed outside lock.
4. Local Mutexes in `Ensure*Detector`: Each closure maintains a local `sync.Mutex` protecting `lastMessageID` and `lastMessageChatID` across concurrent notification and resolution callbacks.
