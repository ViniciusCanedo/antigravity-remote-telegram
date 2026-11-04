## 1. CDP Operations Implementation (`internal/cdp/operations.go`)

- [x] 1.1 Define structs and constants: `InjectResult`, `ExtractedResponseImage`, `UISyncResult`, and `Selectors`
- [x] 1.2 Implement helper methods: `focusChatInput`, `clearInputField`, and `pressEnterToSend`
- [x] 1.3 Implement `InjectMessage` and `InjectMessageWithImageFiles`
- [x] 1.4 Implement `attachImageFiles` with `data-agclaw-upload-token` and `DOM.setFileInputFiles`
- [x] 1.5 Implement `ExtractLatestResponseImages` with UI filtering and fallback parsing
- [x] 1.6 Implement mode sync: `GetCurrentMode` and `SetUIMode`
- [x] 1.7 Implement model sync: `GetUIModels`, `GetCurrentModel`, and `SetUIModel`

## 2. Screenshot Service Implementation (`internal/cdp/screenshot.go`)

- [x] 2.1 Define `CDPCaller`, `ClipRegion`, `CaptureOptions`, `CaptureResult`, and `ScreenshotServiceOptions`
- [x] 2.2 Implement `NewScreenshotService`, `Capture`, and `GetBase64`

## 3. CDP Operations & Screenshot Unit Tests

- [x] 3.1 Implement `internal/cdp/operations_injection_test.go` matching `tests/services/cdpService.injection.test.ts`
- [x] 3.2 Implement `internal/cdp/operations_images_test.go` matching `tests/services/cdpService.imageTransfer.test.ts`
- [x] 3.3 Implement `internal/cdp/operations_uisync_test.go` matching `tests/services/cdpService.uiSync.test.ts`
- [x] 3.4 Implement `internal/cdp/screenshot_test.go` matching `tests/services/screenshotService.test.ts`

## 4. Verification & Quality Gates

- [x] 4.1 Run `go test -v -race ./...` and ensure 100% tests pass
- [x] 4.2 Run `go vet ./...`
- [x] 4.3 Run `gofmt -w .` and check `gofmt -l .` is clean
- [x] 4.4 Run `go build ./...`
