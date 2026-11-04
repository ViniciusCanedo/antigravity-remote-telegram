## 1. Error Popup Detector Implementation (`internal/bridge/error_detector.go`)

- [x] 1.1 Define constants, `ErrorPopupInfo`, and `ErrorPopupDetectorOptions` structs
- [x] 1.2 Implement JavaScript scripts: `DetectErrorPopupScript` and `ReadClipboardScript`
- [x] 1.3 Implement `NewErrorPopupDetector`, `Start`, `Stop`, `IsActive`, and `GetLastDetectedInfo`
- [x] 1.4 Implement `PollOnce` with CDP execution, key deduplication (`title::body[:100]`), 10s cooldown, and `OnResolved`
- [x] 1.5 Implement remote click actions: `ClickDismissButton`, `ClickCopyDebugInfoButton`, `ClickRetryButton`, and `ReadClipboard`

## 2. User Message Detector Implementation (`internal/bridge/user_message_detector.go`)

- [x] 2.1 Define constants, `UserMessageInfo`, and `UserMessageDetectorOptions` structs
- [x] 2.2 Implement JavaScript script `DetectUserMessageScript`
- [x] 2.3 Implement normalization and hashing: `NormalizeForHash` and `ComputeEchoHash`
- [x] 2.4 Implement `NewUserMessageDetector`, `AddEchoHash` with 60s TTL and ref-counting
- [x] 2.5 Implement `Start`, `Stop`, `IsActive`, FIFO `seenHashes` ring buffer of 50 items
- [x] 2.6 Implement `PollOnce` with priming, deduplication, echo suppression, and `onUserMessage` callback

## 3. CDP Bridge Manager Implementation (`internal/bridge/manager.go`)

- [x] 3.1 Define interfaces and structs: `TelegramChannel`, `ExecutionContextInfo`, `CdpServiceWithContexts`, `BridgeConnectionPool`, `InlineButton`, `InlineKeyboardMarkup`, `TelegramBotMessenger`, `AutoAcceptChecker`, `QuotaFetcher`, `PlanNotificationBuilder`, `CdpBridge`
- [x] 3.2 Implement `InitCdpBridge` and `GetCurrentCdp`
- [x] 3.3 Implement `GetCurrentChatTitle`
- [x] 3.4 Implement channel registration and resolution: `RegisterApprovalWorkspaceChannel`, `RegisterApprovalSessionChannel`, and `ResolveApprovalChannelForCurrentChat`
- [x] 3.5 Implement custom ID builders and parsers: `BuildApprovalCustomID`, `ParseApprovalCustomID`, `BuildPlanningCustomID`, `ParsePlanningCustomID`, `BuildErrorPopupCustomID`, `ParseErrorPopupCustomID`
- [x] 3.6 Implement detector orchestration: `EnsureApprovalDetector`, `EnsurePlanningDetector`, `EnsureErrorPopupDetector`, `EnsureUserMessageDetector` with reply markup cleanup on resolution

## 4. Error Popup Detector Unit Tests (`internal/bridge/error_detector_test.go`)

- [x] 4.1 Implement mock `CdpEvaluator` and helper functions
- [x] 4.2 Implement tests 1–5: `CallsOnErrorPopupWhenDetected`, `DoesNotCallCallbackWhenNull`, `DeduplicatesConsecutiveDetections`, `DeduplicatesByTitleAndBodyPrefix`, `SuppressesRedetectionWithin10sCooldown`
- [x] 4.3 Implement tests 6–10: `ClickDismissButtonViaCDP`, `ClickCopyDebugInfoButtonViaCDP`, `ClickRetryButtonViaCDP`, `ReadClipboardReturnsText`, `ReadClipboardReturnsNilWhenEmpty`
- [x] 4.4 Implement tests 11–16: `ReadClipboardReturnsNilOnCDPError`, `StopHaltsPolling`, `ContinuesOnCDPError`, `GetLastDetectedInfoReturnsDetected`, `GetLastDetectedInfoResetsOnDisappear`, `IsActiveLifecycle`
- [x] 4.5 Implement tests 17–21: `OmitsContextIdWhenNil`, `ClickDismissButtonReturnsFalseOnError`, `SilentlyIgnoresWebSocketDisconnect`, `CallsOnResolvedWhenPopupDisappears`, `DoesNotCallOnResolvedWithoutPriorDetection`

## 5. User Message Detector Unit Tests (`internal/bridge/user_message_detector_test.go`)

- [x] 5.1 Implement tests 1–4: `FirstPollPrimesWithoutCallback`, `DetectsNewMessageAfterPriming`, `PrimesWithEmptyDOMAndDetectsFirstMessage`, `DeduplicatesSameMessage`
- [x] 5.2 Implement tests 5–8: `SkipsEchoHashedMessages`, `HandlesCDPErrorsGracefully`, `HandlesNullCDPResult`, `DetectsSequentialDifferentMessages`
- [x] 5.3 Implement tests 9–11: `StartIsIdempotent`, `SeenHashesPreventsOldMessageRedetection`, `SeenHashesClearedOnRestart`

## 6. CDP Bridge Manager Unit Tests (`internal/bridge/manager_test.go`)

- [x] 6.1 Implement tests 1–4: `InitCdpBridgeInitialState`, `GetCurrentCdpReturnsNilWhenDisconnected`, `ApprovalCustomIDRoundTrip`, `ParseLegacyApprovalCustomIDWithoutChannel`
- [x] 6.2 Implement tests 5–10: `RoutesBySessionTitleWhenLinked`, `FallsBackToWorkspaceChannelOnMismatch`, `FallsBackToWorkspaceChannelWhenTitleNil`, `FallsBackToWorkspaceChannelWhenTitleEmpty`, `PrefersSessionChannelOverWorkspaceChannel`, `ReturnsNilWhenUnregistered`
- [x] 6.3 Implement tests 11–14: `PlanningOpenCustomIDRoundTrip`, `PlanningProceedCustomIDRoundTrip`, `ParseLegacyPlanningCustomIDWithoutChannel`, `ParsePlanningCustomIDReturnsNilForInvalid`
- [x] 6.4 Implement Error Popup custom ID tests and Ensure* orchestration tests

## 7. Verification & Quality Gates

- [x] 7.1 Run `go test -v -race ./...` and ensure 100% pass
- [x] 7.2 Run `go vet ./...`
- [x] 7.3 Run `gofmt -w .` and verify `gofmt -l .` is clean
- [x] 7.4 Run `go build ./...`
