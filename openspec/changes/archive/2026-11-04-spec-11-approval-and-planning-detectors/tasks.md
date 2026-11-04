## 1. Approval Detector Implementation (`internal/bridge/approval_detector.go`)

- [x] 1.1 Define constants, `CdpEvaluator` interface, and helper data types (`runtimeEvaluateResponse`, `clickScriptResult`, `ApprovalInfo`, `ApprovalDetectorOptions`)
- [x] 1.2 Implement JavaScript scripts: `DetectApprovalScript`, `ExpandAlwaysAllowMenuScript`, and `BuildClickScript`
- [x] 1.3 Implement `NewApprovalDetector`, `Start`, `Stop`, `IsActive`, and `GetLastDetectedInfo`
- [x] 1.4 Implement `PollOnce` with CDP execution, key deduplication (`approveText::description`), and `onResolved`
- [x] 1.5 Implement remote click actions: `ApproveButton`, `DenyButton`, `AlwaysAllowButton` with dropdown expansion and retry loop

## 2. Planning Detector Implementation (`internal/bridge/planning_detector.go`)

- [x] 2.1 Define constants, `PlanningInfo`, and `PlanningDetectorOptions`
- [x] 2.2 Implement JavaScript scripts: `DetectPlanningScript` and `ExtractPlanContentScript`
- [x] 2.3 Implement `NewPlanningDetector`, `Start`, `Stop`, `IsActive`, and `GetLastDetectedInfo`
- [x] 2.4 Implement `PollOnce` with button pair deduplication (`openText::proceedText`), 5s cooldown tracking (`PlanningCooldown`), and `onResolved`
- [x] 2.5 Implement remote actions: `ClickOpenButton`, `ClickProceedButton`, and `ExtractPlanContent`

## 3. Approval Detector Unit Tests (`internal/bridge/approval_detector_test.go`)

- [x] 3.1 Implement mock `CdpEvaluator`
- [x] 3.2 Implement test cases 1–5: `CallsOnApprovalRequiredWhenDetected`, `DoesNotCallCallbackWhenNull`, `DeduplicatesConsecutiveDetections`, `ApproveButtonClicksViaCDP`, `DenyButtonClicksViaCDP`
- [x] 3.3 Implement test cases 6–10: `AlwaysAllowButtonDirectClick`, `AlwaysAllowButtonExpandsDropdownThenClicks`, `StopHaltsPolling`, `ContinuesOnCDPError`, `GetLastDetectedInfoReturnsDetected`
- [x] 3.4 Implement test cases 11–16: `GetLastDetectedInfoResetsOnDisappear`, `ApproveButtonUsesDetectedText`, `DenyButtonUsesDetectedText`, `OmitsContextIdWhenNil`, `CallsOnResolvedWhenButtonsDisappear`, `DoesNotCallOnResolvedWithoutPriorDetection`

## 4. Planning Detector Unit Tests (`internal/bridge/planning_detector_test.go`)

- [x] 4.1 Implement test cases 1–5: `CallsOnPlanningRequiredWhenDetected`, `DoesNotCallCallbackWhenNull`, `DeduplicatesConsecutiveDetections`, `DeduplicatesByButtonTextPair`, `SuppressesRedetectionWithin5sCooldown`
- [x] 4.2 Implement test cases 6–10: `ClickOpenButtonViaCDP`, `ClickProceedButtonViaCDP`, `ExtractPlanContentReturnsMarkdown`, `ExtractPlanContentReturnsNilWhenNull`, `StopHaltsPolling`
- [x] 4.3 Implement test cases 11–16: `ContinuesOnCDPError`, `GetLastDetectedInfoReturnsDetected`, `GetLastDetectedInfoResetsOnDisappear`, `ClickOpenButtonUsesDetectedText`, `ClickProceedButtonUsesDetectedText`, `OmitsContextIdWhenNil`
- [x] 4.4 Implement test cases 17–22: `IsActiveLifecycle`, `DetectScriptContainsSkipTags`, `ExtractScriptContainsHtmlToMd`, `ExtractPlanContentReturnsNilOnCDPError`, `CallsOnResolvedWhenButtonsDisappear`, `DoesNotCallOnResolvedWithoutPriorDetection`

## 5. Verification & Quality Gates

- [x] 5.1 Run `go test -v -race ./...` and ensure 100% tests pass
- [x] 5.2 Run `go vet ./...`
- [x] 5.3 Run `gofmt -w .` and check `gofmt -l .` is clean
- [x] 5.4 Run `go build ./...`
