## Why

The Antigravity Telegram Remote system requires high-fidelity formatting and log-buffering capabilities to present rich assistant outputs and real-time execution progress inside Telegram's restricted HTML markup mode (which only supports `<b>`, `<i>`, `<code>`, `<pre>`, `<a href="">`, `<s>`, `<u>`, and `<blockquote>`). This migration ports the DOM HTML converter, Markdown-to-Telegram HTML translator, Chrome DevTools UI chrome log separator, Telegram HTML pagination (`splitTelegramHtml`), streaming header and activity filters, task metadata footer extraction, circular deduplicating process log buffer (`ProcessLogBuffer`), and rate-limited progress sender (`ProgressSender`) from TypeScript to pure Go with zero CGO dependencies.

## What Changes

- Implement `internal/formatter/html_converter.go`:
  - `HtmlToTelegramHtml`: Transforms DOM HTML tags to Telegram HTML (`<b>`, `<i>`, `<s>`, `<u>`, `<code>`, `<pre>`, `<blockquote>`, `<a href="...">`), converts HTML tables to ASCII tables in `<pre>`, nested lists up to 5 levels, details/summary, figures, sup/sub, safe entity decoding, and unsupported tag stripping.
- Implement `internal/formatter/telegram_formatter.go`:
  - `EscapeHtml`: Escapes `&`, `<`, `>`.
  - `FormatForTelegram`: Converts Markdown inline/block styling, wraps file references in `<code>`, tables and tree lines in `<pre>`, blockquotes in `<blockquote>`, and auto-closes unclosed code blocks.
  - `SplitOutputAndLogs`: Distinguishes actual model output from UI chrome and activity log lines, with support for MCP multi-call outputs.
  - `SeparateOutputForDelivery`: Routes DOM-structured output vs legacy fallback.
  - `SanitizeActivityLines`: Cleans, filters, and deduplicates activity lines preserving order.
  - `SplitTelegramHtml`: Divides HTML text into Telegram message-safe chunks (max 4096 chars) while closing and reopening active tags across chunk boundaries.
- Implement `internal/formatter/stream_formatter.go`:
  - `BuildModeModelLines`: Produces mode and model lines, consolidating identical models.
  - `ShouldSkipActivityLog`: Filters noise and duplicate activity logs.
  - `SplitForEmbedDescription` & `FitForSingleEmbedDescription`: Embed description splitting and truncation prioritizing recent tails.
- Implement `internal/formatter/metadata.go`:
  - `ExtractMetadataFromFooter`: Parses `TaskID` and `Dir` from message footers into `TaskMetadata`.
- Implement `internal/monitor/process_log_buffer.go`:
  - `ProcessLogBuffer`: Thread-safe circular deduplicated buffer with automated emoji classification, length truncation, and entry/character quota pruning.
- Implement `internal/monitor/progress_sender.go`:
  - `ProgressSender`: Thread-safe throttled progress emitter with timer management, message splitting, and code block wrapping.
- Comprehensive unit tests matching 1:1 the original Jest test suites:
  - `internal/formatter/html_converter_test.go`
  - `internal/formatter/telegram_formatter_test.go`
  - `internal/formatter/split_telegram_html_test.go`
  - `internal/formatter/stream_formatter_test.go`
  - `internal/formatter/metadata_test.go`
  - `internal/monitor/process_log_buffer_test.go`
  - `internal/monitor/progress_sender_test.go`

## Capabilities

### New Capabilities
- `formatters-log-buffers`: Telegram HTML and Markdown formatting, log separation, metadata extraction, circular process log buffering, and throttled progress sending.

### Modified Capabilities

## Impact

- Provides formatting and log extraction engines required by Response Monitor, Chat Session, and Telegram Bot streaming modules.
- 100% pure Go standard library implementation with zero external runtime dependencies.
