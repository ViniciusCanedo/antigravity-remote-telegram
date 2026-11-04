## Why

The Antigravity Telegram Remote system requires real-time detection and handling of error popups, desktop user chat messages, and central coordination of background detectors across multi-workspace CDP connections. Migrating `ErrorPopupDetector` (`src/services/errorPopupDetector.ts`), `UserMessageDetector` (`src/services/userMessageDetector.ts`), and `CdpBridgeManager` (`src/services/cdpBridgeManager.ts`) to Go (`internal/bridge`) completes the bridge layer with thread-safe lifecycle management, automatic deduplication, anti-echo windows, session-aware Telegram routing, and remote button dispatch.

## What Changes

- Add `ErrorPopupDetector` in `internal/bridge/error_detector.go`:
  - `ErrorPopupInfo` and `ErrorPopupDetectorOptions` structs.
  - `DetectErrorPopupScript`: DOM evaluation looking for dialogs, modals, and fixed/absolute error overlays (`[role="dialog"]`, `[role="alertdialog"]`, `.modal`, `.dialog`) matching error patterns.
  - `ReadClipboardScript`: Asynchronous evaluation of `navigator.clipboard.readText()`.
  - Polling lifecycle (`Start`, `Stop`, `IsActive`, `GetLastDetectedInfo`, `PollOnce`) with 10s cooldown and `${title}::${body[:100]}` deduplication.
  - Remote click actions: `ClickDismissButton`, `ClickCopyDebugInfoButton`, `ClickRetryButton`, and `ReadClipboard`.
- Add `UserMessageDetector` in `internal/bridge/user_message_detector.go`:
  - `UserMessageInfo` and `UserMessageDetectorOptions` structs.
  - `DetectUserMessageScript`: Extraction of latest user bubble from `.antigravity-agent-side-panel`.
  - Hashing and normalization: `NormalizeForHash` and `ComputeEchoHash` (16-character SHA-256 hex slice).
  - Anti-echo mechanism (`AddEchoHash`) with 60-second TTL and ref-counting.
  - Priming on start without callback invocation, and FIFO `seenHashes` ring buffer of up to 50 entries.
- Add `CdpBridgeManager` in `internal/bridge/manager.go`:
  - `TelegramChannel`, `ExecutionContextInfo`, `CdpServiceWithContexts`, and `BridgeConnectionPool` interfaces.
  - `TelegramBotMessenger`, `AutoAcceptChecker`, `AutoAcceptResult`, `QuotaFetcher`, and `PlanNotificationBuilder`.
  - `CdpBridge` central state struct with thread-safe mutex protection.
  - Channel registration and resolution: `RegisterApprovalWorkspaceChannel`, `RegisterApprovalSessionChannel`, and `ResolveApprovalChannelForCurrentChat`.
  - Chat title extraction from DOM via `GetCurrentChatTitle`.
  - Workspace resolution via `GetCurrentCdp`.
  - Action custom ID generators and parsers: `BuildApprovalCustomID`, `ParseApprovalCustomID`, `BuildPlanningCustomID`, `ParsePlanningCustomID`, `BuildErrorPopupCustomID`, `ParseErrorPopupCustomID`.
  - Detector orchestration: `EnsureApprovalDetector`, `EnsurePlanningDetector`, `EnsureErrorPopupDetector`, `EnsureUserMessageDetector` with message reply markup cleanup on resolution.
- Complete unit test coverage matching 1:1 original Jest suites:
  - `internal/bridge/error_detector_test.go`: 21 test cases mirroring `tests/services/errorPopupDetector.test.ts`.
  - `internal/bridge/user_message_detector_test.go`: 11 test cases mirroring `tests/services/userMessageDetector.test.ts`.
  - `internal/bridge/manager_test.go`: 14 test cases mirroring `tests/services/cdpBridgeManager.test.ts`, plus orchestration tests.

## Capabilities

### New Capabilities
- `bridge-manager`: Error popup detection, clipboard extraction, PC user message monitoring with echo hash suppression, and centralized bridge orchestration linking CDP connection pools, detectors, auto-accept logic, and Telegram channels.

### Modified Capabilities

## Impact

- Provides end-to-end integration between CDP connections, DOM detectors, and Telegram notification delivery.
- Thread-safe, non-blocking polling and channel resolution across multiple goroutines.
- Zero external dependencies beyond Go standard library packages.
