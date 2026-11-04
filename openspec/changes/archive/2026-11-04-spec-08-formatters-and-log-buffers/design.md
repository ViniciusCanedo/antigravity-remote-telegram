## Context

The TypeScript implementation of Antigravity Telegram Remote relies on regex transformations, string manipulators, and timer-driven buffers to format DOM HTML, parse Markdown, separate Chrome UI clutter, balance Telegram HTML tags, extract task metadata, deduplicate activity logs, and throttle streaming progress messages.

In Golang, these capabilities are divided into two packages:
- `internal/formatter`: Stateless pure functions for HTML conversion, Markdown parsing, Telegram HTML splitting with tag tracking, stream message formatting, and metadata extraction.
- `internal/monitor`: Stateful, thread-safe buffering and scheduling structs (`ProcessLogBuffer` and `ProgressSender`) protected by `sync.Mutex` and Go's `time.Timer`.

## Goals / Non-Goals

**Goals:**
- Provide full 1:1 behavioral equivalence with the original TypeScript implementations in `src/utils/` and `src/services/progressSender.ts`.
- Ensure all regexes compile with Go's `regexp` (RE2 engine) by replacing lookaround assertions (lookahead/lookbehind) with procedural boundary checks in Go.
- Guarantee thread safety for `ProcessLogBuffer` and `ProgressSender` under concurrent goroutine access.
- Achieve 100% test coverage matching all original Jest test cases in `tests/utils/` and `tests/services/`.

**Non-Goals:**
- Calling external Telegram APIs or CDP WebSocket connections directly (handled by downstream bot and CDP modules).
- Introducing third-party HTML parsing packages (standard library `regexp`, `strings`, `strconv` are sufficient and keep binaries small and clean).

## Decisions

- **RE2 Compatibility for Regex Lookarounds**:
  - *Context*: TypeScript uses PCRE-style lookbehinds `(?<!...)` and lookaheads `(?!...)` in `FILE_REF_REGEX` and italic markdown rules `(?<!\w)\*...\*(?!\w)`.
  - *Decision*: Go's `regexp` does not support lookarounds. We use base regular expressions to locate match offsets (`FindAllStringIndex`) and evaluate character boundaries manually in Go.
- **HTML Entity Decoding**:
  - *Context*: Telegram HTML mode strictly requires `&`, `<`, and `>` to be escaped as `&amp;`, `&lt;`, and `&gt;`, but allows other decoded entities (`&quot;`, `&#39;`, `&nbsp;`, unicode codepoints).
  - *Decision*: Implement `decodeSafeEntities` preserving existing `&amp;`, `&lt;`, `&gt;` while decoding hexadecimal, decimal, and common named entities. If a numeric entity resolves to `&`, `<`, `>`, it is re-encoded as `&amp;`, `&lt;`, `&gt;`.
- **HTML Splitter Tag Stack Management**:
  - *Context*: When dividing HTML messages into chunks of <= 4096 characters, open tags must be cleanly closed at chunk ends and reopened at next chunk starts.
  - *Decision*: Maintain an `openTags []string` stack tracking the exact raw open tags (including attributes like `<code class="language-ts">`). Void tags (`<br>`) and self-closing tags (`/>`) are ignored by the stack.
- **ProcessLogBuffer Concurrency & Deduplication**:
  - *Context*: Accessed by multiple polling and ticker goroutines.
  - *Decision*: Synchronize all reads and mutations with `sync.Mutex`. Keep a hash set `seen map[string]struct{}` with case-insensitive keys.
- **ProgressSender Lifecycle**:
  - *Context*: Emits buffered text on throttle intervals (default 3000ms), immediate force emit, or cancellation via `Dispose()`.
  - *Decision*: Synchronize with `sync.Mutex`. Safely cancel `time.Timer` on `Dispose()` or prior to `emit()`.

## Risks / Trade-offs

- [Risk] Tag nesting depth in HTML table or list parsing.
  → [Mitigation] Mirror the TS 5-pass iteration for lists and strict tag extraction for tables.
- [Risk] Long unbroken lines exceeding maximum length in `SplitTelegramHtml`.
  → [Mitigation] Implement hard-splitting with recalculation of remaining tag overhead to prevent infinite loops.
