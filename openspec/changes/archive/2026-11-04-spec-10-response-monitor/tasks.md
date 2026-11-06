## 1. Response Monitor Implementation

- [x] 1.1 Implement types, structs, interfaces, and state machine in `internal/monitor/response_monitor.go`
- [x] 1.2 Implement lifecycle methods `NewResponseMonitor`, `Start`, `StartPassive`, `Stop`, `GetPhase`, `GetQuotaDetected`, `IsActive`, `GetLastText`, `GetLastExtractionSource`
- [x] 1.3 Implement `ClickStopButton` via `RESPONSE_SELECTORS.CLICK_STOP_BUTTON`
- [x] 1.4 Implement polling engine with baseline suppression, quota detection, planning dialog deferral, and stop-gone confirmation
- [x] 1.5 Implement log deduplication for process logs and thinking logs
- [x] 1.6 Implement push mutation debouncing via `Runtime.bindingCalled` ("onantigravity-telegram-remoteDOMChange")

## 2. Test Suites Implementation

- [x] 2.1 Implement `internal/monitor/response_monitor_lean_test.go` matching `tests/services/responseMonitor.lean.test.ts`
- [x] 2.2 Implement `internal/monitor/response_monitor_selectors_test.go` matching `tests/services/responseMonitor.selectors.test.ts`
- [x] 2.3 Implement `internal/monitor/response_monitor_stop_button_test.go` matching `tests/services/responseMonitor.stopButtonSelector.test.ts`
- [x] 2.4 Implement `internal/monitor/response_monitor_test.go` matching `tests/services/responseMonitor.test.ts`

## 3. Verification & Quality Gates

- [x] 3.1 Run `go test -v -race ./...` and verify 100% pass rate
- [x] 3.2 Run `go vet ./...`
- [x] 3.3 Run `gofmt -w .` and check `gofmt -l .` is clean
- [x] 3.4 Run `go build ./...`
- [x] 3.5 Sync spec delta to `openspec/specs/response-monitor/spec.md`
- [x] 3.6 Archive change via `openspec archive -y spec-10-response-monitor`
- [x] 3.7 Re-verify quality gates after archiving
