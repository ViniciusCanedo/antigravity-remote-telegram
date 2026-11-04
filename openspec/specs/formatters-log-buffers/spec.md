# formatters-log-buffers Specification

## Purpose
TBD - created by archiving change spec-08-formatters-and-log-buffers. Update Purpose after archive.
## Requirements
### Requirement: DOM HTML to Telegram HTML Conversion
The `formatter` package SHALL provide `HtmlToTelegramHtml(html string) string` converting DOM HTML formatting into Telegram-supported HTML elements (`<b>`, `<i>`, `<s>`, `<u>`, `<code>`, `<pre>`, `<a href="">`, `<blockquote>`), converting HTML tables to formatted monospace ASCII tables, nested lists to indented lists, and stripping unsupported HTML tags.

#### Scenario: Basic conversions and formatting
- **WHEN** empty or nil string is provided
- **THEN** an empty string is returned
- **WHEN** text with `<strong>`, `<em>`, `<del>`, `<strike>`, `<ins>`, `<mark>` is provided
- **THEN** it converts to `<b>`, `<i>`, `<s>`, `<u>`, `<b>` respectively

#### Scenario: Code blocks and headings
- **WHEN** `<pre><code class="language-js">console.log("hi")</code></pre>` is provided
- **THEN** it is preserved with `<pre>`, `<code>`, and language class intact
- **WHEN** `<h1>Title</h1>` is provided
- **THEN** it is converted to `\n<b>Title</b>\n`

#### Scenario: HTML tables to ASCII tables
- **WHEN** an HTML `<table>` with rows and cells is converted
- **THEN** it produces an ASCII formatted table inside `<pre>...</pre>` with column padding and pipe separators
- **WHEN** an empty table `<table></table>` is provided
- **THEN** it returns an empty string

#### Scenario: Entity decoding
- **WHEN** HTML containing `&quot;`, `&#39;`, `&nbsp;`, `&#65;`, `&#x41;` is converted
- **THEN** they are decoded to `"`, `'`, `' '`, `A`, `A` while preserving `&amp;`, `&lt;`, and `&gt;`

### Requirement: Telegram Markdown Formatting and Log Separation
The `formatter` package SHALL provide `EscapeHtml`, `FormatForTelegram`, `SplitOutputAndLogs`, `SeparateOutputForDelivery`, and `SanitizeActivityLines`.

#### Scenario: Escape HTML characters
- **WHEN** `EscapeHtml("a & b < c > d")` is called
- **THEN** it returns `"a &amp; b &lt; c &gt; d"`

#### Scenario: Markdown and special block formatting
- **WHEN** markdown with headings, bold, italic, code blocks, blockquotes, file paths, and ASCII trees is formatted with `FormatForTelegram`
- **THEN** headings become `<b>`, bold becomes `<b>`, italic becomes `<i>`, trees/tables are enclosed in `<pre>`, blockquotes are enclosed in `<blockquote>`, and file references are enclosed in `<code>`

#### Scenario: Split output and UI chrome logs
- **WHEN** text containing UI chrome literals or activity lines mixed with actual content is processed by `SplitOutputAndLogs`
- **THEN** actual content is segregated into `Output` and UI chrome / activity lines into `Logs`

#### Scenario: Safe Telegram HTML splitting
- **WHEN** `SplitTelegramHtml` splits HTML exceeding `maxLen`
- **THEN** chunks are split at line boundaries or hard limits, with open tags closed before chunk breaks and reopened in the subsequent chunk

### Requirement: Streaming Message Formatting
The `formatter` package SHALL provide `BuildModeModelLines`, `ShouldSkipActivityLog`, `SplitForEmbedDescription`, and `FitForSingleEmbedDescription`.

#### Scenario: Build mode and model lines
- **WHEN** fast and plan models are identical (case-insensitive)
- **THEN** a single consolidated model line is generated
- **WHEN** models differ
- **THEN** separate lines for Fast and Plan models are generated

#### Scenario: Activity log noise filtering
- **WHEN** activity logs contain generic status words, single-word tokens, or analyzed trace prefixes
- **THEN** `ShouldSkipActivityLog` returns `true`

### Requirement: Task Metadata Extraction
The `formatter` package SHALL provide `ExtractMetadataFromFooter(footerText string) TaskMetadata` extracting `TaskID` and `Dir` properties from message footers.

#### Scenario: Extract metadata
- **WHEN** footer contains `TaskID: abc-123 | Dir: /workspace/projectA`
- **THEN** `TaskID` is `"abc-123"` and `Directory` is `"/workspace/projectA"`

### Requirement: Process Log Buffer
The `monitor` package SHALL provide `ProcessLogBuffer` managing a circular, deduplicated, emoji-tagged buffer of process activities.

#### Scenario: Emoji tagging and deduplication
- **WHEN** log entries with operation keywords are appended
- **THEN** appropriate emojis (🚀, 🧠, 🛠️, 📄, 🔍, 🔎, 📦) are prepended and duplicate entries are ignored

#### Scenario: Capacity and character truncation
- **WHEN** entries exceed `MaxChars` or `MaxEntries`
- **THEN** oldest entries are dropped to fit within limits

### Requirement: Progress Sender
The `monitor` package SHALL provide `ProgressSender` aggregating streaming text with time-based throttling and optional code block encapsulation.

#### Scenario: Throttled emission and force emit
- **WHEN** `Append` is called repeatedly within `throttleMs`
- **THEN** text is accumulated and sent in a single emission after the throttle duration
- **WHEN** `ForceEmit` is called
- **THEN** pending text is sent immediately and any active timer is cleared

#### Scenario: Disposal
- **WHEN** `Dispose` is called
- **THEN** active timers are cancelled and buffered text is discarded without emission

