## 1. Streaming Pipeline & Utilities

- [x] 1.1 Implement `internal/bot/streaming.go` with constants, `SerialTaskQueue`, and HTML formatting utilities (`StripHTMLForFile`, `BalanceHTMLTags`, `StripHTML`, `IsJunkEntry`, `FormatActivityLine`)
- [x] 1.2 Implement `SendPromptToAntigravity` in `internal/bot/streaming.go` with progress tracking, versioning, 5s ticker, thinking blocks, chunking, topic renaming, and image transmission
- [x] 1.3 Implement `StartPassiveMirrorForUserMessage` in `internal/bot/streaming.go`
- [x] 1.4 Implement unit tests in `internal/bot/streaming_test.go` (`TestBalanceHTMLTags`, `TestStripHTMLForFile`, `TestIsJunkEntryAndFormatActivityLine`, `TestSendMsg_ThreeTierFallback`, `TestSendChunkedResponse`)

## 2. Callback Query Router

- [x] 2.1 Implement `internal/bot/callbacks.go` with `CallbackContext` interface and `HandleCallbackQuery` routing all 11 callback button families
- [x] 2.2 Implement unit tests in `internal/bot/callbacks_test.go` covering each callback family

## 3. Message Handlers & Passive Mirroring

- [x] 3.1 Implement `internal/bot/handlers_messages.go` with `HandleTextMessage`, `HandlePhotoMessage`, and `HandleVoiceMessage`
- [x] 3.2 Wire handlers and callback query dispatcher into `internal/bot/bot.go`
- [x] 3.3 Implement unit tests in `internal/bot/handlers_messages_test.go`

## 4. Quality Gate & Archive

- [x] 4.1 Run full quality gate (`go test -v -race ./...`, `go vet ./...`, `gofmt -w .`, `go build ./...`)
- [x] 4.2 Sync spec deltas to `openspec/specs/bot-streaming/spec.md`
- [x] 4.3 Archive change with `openspec archive -y spec-16-telegram-bot-streaming-messages-and-callbacks`
- [x] 4.4 Re-verify quality gate post-archive
