## 1. Formatter Package Implementation

- [x] 1.1 Implement `internal/formatter/html_converter.go` (`HtmlToTelegramHtml`, `stripTags`, `stripUnsupportedTags`, `decodeSafeEntities`, `looksLikeFilePath`)
- [x] 1.2 Implement `internal/formatter/html_converter_test.go` matching `tests/utils/htmlToTelegramMarkdown.test.ts`
- [x] 1.3 Implement `internal/formatter/telegram_formatter.go` (`EscapeHtml`, `FormatForTelegram`, `SplitOutputAndLogs`, `SeparateOutputForDelivery`, `SplitTelegramHtml`, `SanitizeActivityLines`)
- [x] 1.4 Implement `internal/formatter/telegram_formatter_test.go` matching `tests/utils/telegramFormatter.test.ts`
- [x] 1.5 Implement `internal/formatter/split_telegram_html_test.go` matching `tests/utils/splitTelegramHtml.test.ts`
- [x] 1.6 Implement `internal/formatter/stream_formatter.go` (`BuildModeModelLines`, `ShouldSkipActivityLog`, `SplitForEmbedDescription`, `FitForSingleEmbedDescription`)
- [x] 1.7 Implement `internal/formatter/stream_formatter_test.go` matching `tests/utils/streamMessageFormatter.test.ts`
- [x] 1.8 Implement `internal/formatter/metadata.go` (`ExtractMetadataFromFooter`, `TaskMetadata`)
- [x] 1.9 Implement `internal/formatter/metadata_test.go` matching `tests/utils/metadataExtractor.test.ts`

## 2. Monitor Package Implementation

- [x] 2.1 Implement `internal/monitor/process_log_buffer.go` (`ProcessLogBuffer`, `ProcessLogBufferOptions`, `pickEmoji`)
- [x] 2.2 Implement `internal/monitor/process_log_buffer_test.go` matching `tests/utils/processLogBuffer.test.ts`
- [x] 2.3 Implement `internal/monitor/progress_sender.go` (`ProgressSender`, `ProgressSenderOptions`)
- [x] 2.4 Implement `internal/monitor/progress_sender_test.go` matching `tests/services/progressSender.test.ts` and `tests/services/progressSender.bugfix.test.ts`

## 3. Verification & Quality Gates

- [x] 3.1 Run `go test -v ./...` and verify 100% pass rate
- [x] 3.2 Run `go vet ./...`
- [x] 3.3 Run `gofmt -w .` and check `gofmt -l .` is clean
- [x] 3.4 Run `go build ./...`
- [x] 3.5 Sync spec delta to `openspec/specs/formatters-log-buffers/spec.md`
- [x] 3.6 Archive change via `openspec archive -y spec-08-formatters-and-log-buffers`
- [x] 3.7 Re-verify quality gates after archiving
