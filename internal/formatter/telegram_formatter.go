package formatter

import (
	"fmt"
	"regexp"
	"strings"
)

var uiChromeLiterals = map[string]struct{}{
	"analyzed":     {},
	"analyzing":    {},
	"reading":      {},
	"writing":      {},
	"running":      {},
	"searching":    {},
	"planning":     {},
	"thinking":     {},
	"thinking...":  {},
	"processing":   {},
	"loading":      {},
	"executing":    {},
	"testing":      {},
	"debugging":    {},
	"read":         {},
	"wrote":        {},
	"ran":          {},
	"good":         {},
	"bad":          {},
	"good bad":     {},
	"show details": {},
	"json":         {},
	"css":          {},
	"html":         {},
	"xml":          {},
	"yaml":         {},
	"toml":         {},
	"sql":          {},
	"graphql":      {},
}

var uiChromeRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^[+-]\d+$`),
	regexp.MustCompile(`(?i)^\d+\s*chars?$`),
	regexp.MustCompile(`(?i)^line\s+\d+`),
	regexp.MustCompile(`(?i)^col\s+\d+`),
	regexp.MustCompile(`(?i)^tool call:`),
	regexp.MustCompile(`(?i)^tool result:`),
	regexp.MustCompile(`(?i)^calling tool\b`),
	regexp.MustCompile(`(?i)^tool response\b`),
	regexp.MustCompile(`(?i)^mcp\b`),
	regexp.MustCompile(`(?i)^thought for\s*<?\d+`),
	regexp.MustCompile(`(?i)^show details$`),
	regexp.MustCompile(`(?i)^[a-z0-9._-]+\s*/\s*[a-z0-9._-]+$`),
	regexp.MustCompile(`(?i)^full output written to\b`),
	regexp.MustCompile(`(?i)^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$`),
	regexp.MustCompile(`^\s*\{\s*$`),
	regexp.MustCompile(`^\s*\}\s*$`),
	regexp.MustCompile(`^\s*"[^"]*"\s*:\s*`),
}

var (
	reFileRef          = regexp.MustCompile(`\b[a-zA-Z][\w.-]*(?:/[\w.-]+)+(?::\d+(?:-\d+)?)?`)
	reTableLine        = regexp.MustCompile(`^\|[\s\-:]+\|`)
	reTreeLineChars    = regexp.MustCompile(`[├└│┌┐┘┤┬┴┼]`)
	reTreeLineBranch1  = regexp.MustCompile(`^\s*[│├└]\s*──`)
	reTreeLineBranch2  = regexp.MustCompile(`^\s*\|.*──`)
	reHeading          = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	reHorizontalRule   = regexp.MustCompile(`^(?:---+|\*\*\*+|___+)\s*$`)
	reInlineCodeMd     = regexp.MustCompile("`([^`]+)`")
	reLinkMd           = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBoldAsterisks    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldUnderscores  = regexp.MustCompile(`__(.+?)__`)
	reAsteriskItalic   = regexp.MustCompile(`\*([^*]+)\*`)
	reUnderscoreItalic = regexp.MustCompile(`_([^_]+)_`)
	reStrikeMd         = regexp.MustCompile(`~~(.+?)~~`)
	reQuotePrefix      = regexp.MustCompile(`^>\s?`)
	reMcpFormatLine    = regexp.MustCompile(`(?i)^[a-z0-9._-]+\s*/\s*[a-z0-9._-]+$`)
	reActivityLogLine  = regexp.MustCompile(`(?i)^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|analyzed|read|wrote|ran)\s+.+`)
	reHtmlTag          = regexp.MustCompile(`(?i)</?(\w+)(?:\s[^>]*)?\s*/?>`)
	reTagNameOnly      = regexp.MustCompile(`(?i)^<(\w+)`)
)

// SplitOutputResult represents the return value of SplitOutputAndLogs.
type SplitOutputResult struct {
	Output string `json:"output"`
	Logs   string `json:"logs"`
}

// SeparateOutputOptions represents the input parameters for SeparateOutputForDelivery.
type SeparateOutputOptions struct {
	RawText          string   `json:"rawText"`
	DomSource        string   `json:"domSource"` // "dom-structured" | "legacy-fallback"
	DomOutputText    *string  `json:"domOutputText,omitempty"`
	DomActivityLines []string `json:"domActivityLines,omitempty"`
}

// SeparateOutputResult represents the return value of SeparateOutputForDelivery.
type SeparateOutputResult struct {
	Source string `json:"source"`
	Output string `json:"output"`
	Logs   string `json:"logs"`
}

func isUiChromeLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if _, ok := uiChromeLiterals[lower]; ok {
		return true
	}
	for _, re := range uiChromeRegexes {
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// EscapeHtml escapes special HTML characters (&, <, >) for Telegram HTML parse mode.
func EscapeHtml(text string) string {
	res := strings.ReplaceAll(text, "&", "&amp;")
	res = strings.ReplaceAll(res, "<", "&lt;")
	res = strings.ReplaceAll(res, ">", "&gt;")
	return res
}

func wrapFileReferences(line string) string {
	matches := reFileRef.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return line
	}
	var sb strings.Builder
	lastIdx := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start < lastIdx {
			continue
		}
		// Lookbehind check: prev char cannot be `, /, or \
		if start > 0 {
			prev := line[start-1]
			if prev == '`' || prev == '/' || prev == '\\' {
				continue
			}
		}
		// Lookahead check
		if end < len(line) && line[end] == '`' {
			continue
		}
		matchEnd := end
		if end < len(line) && (line[end] == ' ' || line[end] == '\t') {
			if end+1 == len(line) || line[end+1] != '`' {
				matchEnd = end + 1
			}
		}
		sb.WriteString(line[lastIdx:start])
		sb.WriteString("<code>")
		sb.WriteString(line[start:end])
		sb.WriteString("</code>")
		lastIdx = matchEnd
	}
	sb.WriteString(line[lastIdx:])
	return sb.String()
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func formatMarkdownLine(line string) string {
	result := line

	// Headings: ### Title -> <b>Title</b>
	if hMatch := reHeading.FindStringSubmatch(result); len(hMatch) > 2 {
		return fmt.Sprintf("<b>%s</b>", hMatch[2])
	}

	// Horizontal rule: --- or *** or ___ -> ---
	if reHorizontalRule.MatchString(strings.TrimSpace(result)) {
		return "---"
	}

	// Inline code: `code` -> <code>code</code> (must be before bold/italic)
	result = reInlineCodeMd.ReplaceAllString(result, "<code>$1</code>")

	// Links: [text](url) -> <a href="url">text</a>
	result = reLinkMd.ReplaceAllString(result, `<a href="$2">$1</a>`)

	// Bold: **text** or __text__ -> <b>text</b>
	result = reBoldAsterisks.ReplaceAllString(result, "<b>$1</b>")
	result = reBoldUnderscores.ReplaceAllString(result, "<b>$1</b>")

	// Italic: *text* or _text_ -> <i>text</i> (not inside words)
	result = replaceItalic(result, reAsteriskItalic)
	result = replaceItalic(result, reUnderscoreItalic)

	// Strikethrough: ~~text~~ -> <s>text</s>
	result = reStrikeMd.ReplaceAllString(result, "<s>$1</s>")

	return result
}

func replaceItalic(line string, re *regexp.Regexp) string {
	matches := re.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return line
	}
	var sb strings.Builder
	lastIdx := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		gStart, gEnd := m[2], m[3]
		if start < lastIdx {
			continue
		}
		// Lookbehind: byte before start cannot be word char
		if start > 0 && isWordChar(line[start-1]) {
			continue
		}
		// Lookahead: byte at end cannot be word char
		if end < len(line) && isWordChar(line[end]) {
			continue
		}
		sb.WriteString(line[lastIdx:start])
		sb.WriteString("<i>")
		sb.WriteString(line[gStart:gEnd])
		sb.WriteString("</i>")
		lastIdx = end
	}
	sb.WriteString(line[lastIdx:])
	return sb.String()
}

// FormatForTelegram converts Markdown syntax to Telegram HTML, wraps table/tree lines in <pre>,
// and file references in <code>.
func FormatForTelegram(text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	inSpecialBlock := false
	inCodeBlock := false
	codeBlockHasLang := false
	inBlockquote := false
	var blockquoteLines []string

	flushBlockquote := func() {
		if len(blockquoteLines) > 0 {
			result = append(result, fmt.Sprintf("<blockquote>%s</blockquote>", strings.Join(blockquoteLines, "\n")))
			blockquoteLines = nil
		}
		inBlockquote = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if inBlockquote {
				flushBlockquote()
			}
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				lang := strings.TrimSpace(trimmed[3:])
				codeBlockHasLang = (lang != "")
				if codeBlockHasLang {
					result = append(result, fmt.Sprintf("<pre><code class=\"language-%s\">", EscapeHtml(lang)))
				} else {
					result = append(result, "<pre>")
				}
			} else {
				if codeBlockHasLang {
					result = append(result, "</code></pre>")
				} else {
					result = append(result, "</pre>")
				}
				codeBlockHasLang = false
			}
			continue
		}

		if inCodeBlock {
			result = append(result, EscapeHtml(line))
			continue
		}

		isTableLine := (strings.HasPrefix(trimmed, "|") && len(trimmed) > 2) || reTableLine.MatchString(trimmed)
		isTreeLine := reTreeLineChars.MatchString(line) || reTreeLineBranch1.MatchString(line) || reTreeLineBranch2.MatchString(line)
		isSpecialLine := isTableLine || isTreeLine

		// Blockquote lines: > text
		if strings.HasPrefix(trimmed, ">") && !isSpecialLine {
			if inSpecialBlock {
				result = append(result, "</pre>")
				inSpecialBlock = false
			}
			inBlockquote = true
			quoteContent := reQuotePrefix.ReplaceAllString(trimmed, "")
			blockquoteLines = append(blockquoteLines, formatMarkdownLine(EscapeHtml(quoteContent)))
			continue
		} else if inBlockquote {
			flushBlockquote()
		}

		if isSpecialLine && !inSpecialBlock {
			result = append(result, "<pre>")
			inSpecialBlock = true
			result = append(result, EscapeHtml(line))
		} else if isSpecialLine && inSpecialBlock {
			result = append(result, EscapeHtml(line))
		} else if !isSpecialLine && inSpecialBlock {
			result = append(result, "</pre>")
			inSpecialBlock = false
			result = append(result, formatMarkdownLine(wrapFileReferences(EscapeHtml(line))))
		} else {
			result = append(result, formatMarkdownLine(wrapFileReferences(EscapeHtml(line))))
		}
	}

	if inBlockquote {
		flushBlockquote()
	}
	if inSpecialBlock {
		result = append(result, "</pre>")
	}
	if inCodeBlock {
		if codeBlockHasLang {
			result = append(result, "</code></pre>")
		} else {
			result = append(result, "</pre>")
		}
	}

	return strings.Join(result, "\n")
}

func isMcpFormatLine(line string) bool {
	return reMcpFormatLine.MatchString(line)
}

func isActivityLogLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	return reActivityLogLine.MatchString(trimmed) && len(trimmed) <= 220
}

func collapseBlankLines(text string) string {
	res := reMultiNewline.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(res)
}

// SplitOutputAndLogs separates model output and UI chrome / activity log lines.
func SplitOutputAndLogs(rawText string) SplitOutputResult {
	normalized := strings.ReplaceAll(rawText, "\r", "")
	if strings.TrimSpace(normalized) == "" {
		return SplitOutputResult{Output: "", Logs: ""}
	}

	lines := strings.Split(normalized, "\n")
	classes := make([]string, len(lines))
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			classes[i] = "code"
			continue
		}
		if inCodeBlock {
			classes[i] = "code"
			continue
		}
		if trimmed == "" {
			classes[i] = "blank"
			continue
		}
		if isUiChromeLine(trimmed) || isActivityLogLine(trimmed) {
			classes[i] = "chrome"
		} else {
			classes[i] = "output"
		}
	}

	hasMcpCalls := false
	for i, l := range lines {
		if classes[i] == "chrome" && isMcpFormatLine(strings.TrimSpace(l)) {
			hasMcpCalls = true
			break
		}
	}

	if !hasMcpCalls {
		var outputLines []string
		var logLines []string
		for i := 0; i < len(lines); i++ {
			if classes[i] == "chrome" {
				logLines = append(logLines, strings.TrimSpace(lines[i]))
			} else {
				outputLines = append(outputLines, lines[i])
			}
		}
		return SplitOutputResult{
			Output: collapseBlankLines(strings.Join(outputLines, "\n")),
			Logs:   collapseBlankLines(strings.Join(logLines, "\n")),
		}
	}

	lastOutputEnd := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if classes[i] == "output" || classes[i] == "code" {
			lastOutputEnd = i
			break
		}
	}

	if lastOutputEnd == -1 {
		var logLines []string
		for i, l := range lines {
			if classes[i] == "chrome" {
				logLines = append(logLines, strings.TrimSpace(l))
			}
		}
		return SplitOutputResult{Output: "", Logs: collapseBlankLines(strings.Join(logLines, "\n"))}
	}

	lastOutputStart := lastOutputEnd
	for i := lastOutputEnd - 1; i >= 0; i-- {
		if classes[i] == "blank" || classes[i] == "chrome" {
			break
		}
		if classes[i] == "output" || classes[i] == "code" {
			lastOutputStart = i
		}
	}

	var outputLines []string
	var logLines []string

	for i := 0; i < len(lines); i++ {
		if i >= lastOutputStart && i <= lastOutputEnd {
			if classes[i] != "chrome" {
				outputLines = append(outputLines, lines[i])
			} else {
				logLines = append(logLines, strings.TrimSpace(lines[i]))
			}
		} else if classes[i] == "chrome" || classes[i] == "output" {
			logLines = append(logLines, strings.TrimSpace(lines[i]))
		}
	}

	return SplitOutputResult{
		Output: collapseBlankLines(strings.Join(outputLines, "\n")),
		Logs:   collapseBlankLines(strings.Join(logLines, "\n")),
	}
}

// SeparateOutputForDelivery selects between DOM-structured output and legacy-fallback.
func SeparateOutputForDelivery(options SeparateOutputOptions) SeparateOutputResult {
	if options.DomSource == "dom-structured" && options.DomOutputText != nil {
		return SeparateOutputResult{
			Source: "dom-structured",
			Output: *options.DomOutputText,
			Logs:   strings.Join(options.DomActivityLines, "\n"),
		}
	}

	separated := SplitOutputAndLogs(options.RawText)
	return SeparateOutputResult{
		Source: "legacy-fallback",
		Output: separated.Output,
		Logs:   separated.Logs,
	}
}

// SanitizeActivityLines removes UI chrome lines, filters empty lines, and deduplicates keeping order.
func SanitizeActivityLines(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r", "")
	lines := strings.Split(normalized, "\n")
	seen := make(map[string]bool)
	var kept []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !isUiChromeLine(trimmed) {
			if !seen[trimmed] {
				seen[trimmed] = true
				kept = append(kept, trimmed)
			}
		}
	}

	return strings.Join(kept, "\n")
}

func closingForTag(tag string) string {
	sub := reTagNameOnly.FindStringSubmatch(tag)
	if len(sub) > 1 {
		return fmt.Sprintf("</%s>", sub[1])
	}
	return ""
}

// SplitTelegramHtml splits an HTML-formatted string into multiple Telegram-safe chunks (max 4096 by default).
// Respects HTML tag boundaries by closing unclosed tags at chunk end and reopening them at the start of next chunk.
func SplitTelegramHtml(html string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 4096
	}

	if len(html) <= maxLen {
		return []string{html}
	}

	lines := strings.Split(html, "\n")
	var chunks []string
	current := ""
	var openTags []string

	flush := func() {
		if current == "" {
			return
		}
		var suffix strings.Builder
		for i := len(openTags) - 1; i >= 0; i-- {
			suffix.WriteString(closingForTag(openTags[i]))
		}
		chunks = append(chunks, current+suffix.String())
		current = strings.Join(openTags, "")
	}

	trackTags := func(line string) {
		matches := reHtmlTag.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			full := m[0]
			if strings.HasSuffix(full, "/>") {
				continue
			}
			tagName := strings.ToLower(m[1])
			if tagName == "br" {
				continue
			}
			if strings.HasPrefix(full, "</") {
				// Pop matching open tag from back
				for i := len(openTags) - 1; i >= 0; i-- {
					openSub := reTagNameOnly.FindStringSubmatch(openTags[i])
					if len(openSub) > 1 && strings.ToLower(openSub[1]) == tagName {
						openTags = append(openTags[:i], openTags[i+1:]...)
						break
					}
				}
			} else {
				openTags = append(openTags, full)
			}
		}
	}

	for _, line := range lines {
		candidate := line
		if current != "" {
			candidate = current + "\n" + line
		}

		// Account for closing tags that would be needed after adding this line
		tempTags := make([]string, len(openTags))
		copy(tempTags, openTags)

		tempMatches := reHtmlTag.FindAllStringSubmatch(line, -1)
		for _, m := range tempMatches {
			full := m[0]
			if strings.HasSuffix(full, "/>") {
				continue
			}
			tagName := strings.ToLower(m[1])
			if tagName == "br" {
				continue
			}
			if strings.HasPrefix(full, "</") {
				for j := len(tempTags) - 1; j >= 0; j-- {
					openSub := reTagNameOnly.FindStringSubmatch(tempTags[j])
					if len(openSub) > 1 && strings.ToLower(openSub[1]) == tagName {
						tempTags = append(tempTags[:j], tempTags[j+1:]...)
						break
					}
				}
			} else {
				tempTags = append(tempTags, full)
			}
		}

		newClosingLen := 0
		for i := len(tempTags) - 1; i >= 0; i-- {
			newClosingLen += len(closingForTag(tempTags[i]))
		}

		if len(candidate)+newClosingLen <= maxLen {
			current = candidate
			trackTags(line)
			continue
		}

		// Current chunk is full — flush it
		if current != "" {
			flush()
		}

		// If this single line fits in a fresh chunk
		reopenLen := len(strings.Join(openTags, ""))
		if reopenLen+len(line)+newClosingLen <= maxLen {
			current = strings.Join(openTags, "") + line
			trackTags(line)
			continue
		}

		// Hard-split a very long line
		available := maxLen - reopenLen - newClosingLen
		if available <= 0 {
			current = strings.Join(openTags, "") + line
			trackTags(line)
			flush()
			continue
		}

		cursor := 0
		for cursor < len(line) {
			sliceEnd := cursor + available
			if sliceEnd > len(line) {
				sliceEnd = len(line)
			}
			slice := line[cursor:sliceEnd]
			current = strings.Join(openTags, "") + slice
			trackTags(slice)
			cursor += available
			if cursor < len(line) {
				flush()
				updatedClosingLen := 0
				for i := len(openTags) - 1; i >= 0; i-- {
					updatedClosingLen += len(closingForTag(openTags[i]))
				}
				available = maxLen - len(strings.Join(openTags, "")) - updatedClosingLen
				if available <= 0 {
					available = 1
				}
			}
		}
	}

	if current != "" {
		var suffix strings.Builder
		for i := len(openTags) - 1; i >= 0; i-- {
			suffix.WriteString(closingForTag(openTags[i]))
		}
		chunks = append(chunks, current+suffix.String())
	}

	if len(chunks) > 0 {
		return chunks
	}
	return []string{html}
}
