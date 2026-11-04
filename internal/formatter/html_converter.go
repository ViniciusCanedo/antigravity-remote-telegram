package formatter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reInvisibleStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reInvisibleScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reBr              = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHr              = regexp.MustCompile(`(?i)<hr\s*/?>`)
	reHeadings        = regexp.MustCompile(`(?is)<h[1-6][^>]*>(.*?)</h[1-6]>`)
	rePreCodeBlock    = regexp.MustCompile(`(?is)<pre[^>]*>\s*<code(?:\s+class="language-([^"]*)")?[^>]*>(.*?)</code>\s*</pre>`)
	reInlineCode      = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	reBold            = regexp.MustCompile(`(?is)<(?:strong|b)(?:\s[^>]*)?>(.*?)</(?:strong|b)>`)
	reItalic          = regexp.MustCompile(`(?is)<(?:em|i)(?:\s[^>]*)?>(.*?)</(?:em|i)>`)
	reStrike          = regexp.MustCompile(`(?is)<(?:del|strike|s)(?:\s[^>]*)?>(.*?)</(?:del|strike|s)>`)
	reUnderline       = regexp.MustCompile(`(?is)<(?:ins|u)(?:\s[^>]*)?>(.*?)</(?:ins|u)>`)
	reMark            = regexp.MustCompile(`(?is)<mark[^>]*>(.*?)</mark>`)
	reAnchorHref      = regexp.MustCompile(`(?is)<a\s+[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	reBlockquote      = regexp.MustCompile(`(?is)<blockquote[^>]*>(.*?)</blockquote>`)
	reContextScope    = regexp.MustCompile(`(?is)<span[^>]*class="[^"]*context-scope-mention[^"]*"[^>]*>(.*?)</span>`)
	reTitleAttr       = regexp.MustCompile(`(?is)<(?:div|span|a)[^>]*\btitle="([^"]*)"[^>]*>(.*?)</(?:div|span|a)>`)
	reImgWithAlt      = regexp.MustCompile(`(?is)<img[^>]*alt="([^"]*)"[^>]*\/?>`)
	reImgNoAlt        = regexp.MustCompile(`(?is)<img[^>]*\/?>`)
	reDetailsSummary  = regexp.MustCompile(`(?is)<details[^>]*>\s*<summary[^>]*>(.*?)</summary>(.*?)</details>`)
	reFigureCaption   = regexp.MustCompile(`(?is)<figure[^>]*>(.*?)<figcaption[^>]*>(.*?)</figcaption>.*?</figure>`)
	reFigureOnly      = regexp.MustCompile(`(?is)<figure[^>]*>(.*?)</figure>`)
	reTable           = regexp.MustCompile(`(?is)<table[^>]*>(.*?)</table>`)
	reTableRow        = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	reTableCell       = regexp.MustCompile(`(?is)<(?:td|th)[^>]*>(.*?)</(?:td|th)>`)
	reDefinitionList  = regexp.MustCompile(`(?is)<dl[^>]*>(.*?)</dl>`)
	reDt              = regexp.MustCompile(`(?is)<dt[^>]*>(.*?)</dt>`)
	reDd              = regexp.MustCompile(`(?is)<dd[^>]*>(.*?)</dd>`)
	reSup             = regexp.MustCompile(`(?is)<sup[^>]*>(.*?)</sup>`)
	reSub             = regexp.MustCompile(`(?is)<sub[^>]*>(.*?)</sub>`)
	reParagraph       = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	reDiv             = regexp.MustCompile(`(?is)<div[^>]*>(.*?)</div\s*>`)
	reListCheck       = regexp.MustCompile(`(?i)<(?:ul|ol)\b`)
	reListItem        = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	reFilePath        = regexp.MustCompile(`^[a-zA-Z0-9._\-/]+\.[a-zA-Z0-9]+$`)
	reAllTags         = regexp.MustCompile(`<[^>]+>`)
	reAllowedTag      = regexp.MustCompile(`(?i)^/?(b|i|u|s|code|pre|a|blockquote)(\s|>|/|$)`)
	reHexEntity       = regexp.MustCompile(`(?i)&#x([0-9a-f]+);`)
	reDecEntity       = regexp.MustCompile(`&#(\d+);`)
	reMultiNewline    = regexp.MustCompile(`\n{3,}`)
)

// HtmlToTelegramHtml converts common HTML tags extracted from DOM to Telegram-compatible HTML formatting.
// Supported Telegram HTML tags: <b>, <i>, <code>, <pre>, <a href="">, <s>, <u>, <blockquote>.
func HtmlToTelegramHtml(html string) string {
	if html == "" {
		return ""
	}

	result := html

	// Remove non-visible elements
	result = reInvisibleStyle.ReplaceAllString(result, "")
	result = reInvisibleScript.ReplaceAllString(result, "")

	// Line breaks and separators
	result = reBr.ReplaceAllString(result, "\n")
	result = reHr.ReplaceAllString(result, "\n---\n")

	// Headings (all levels -> bold)
	result = reHeadings.ReplaceAllStringFunc(result, func(m string) string {
		sub := reHeadings.FindStringSubmatch(m)
		if len(sub) > 1 {
			return fmt.Sprintf("\n<b>%s</b>\n", strings.TrimSpace(stripTags(sub[1])))
		}
		return m
	})

	// Code blocks: <pre><code> (must come before inline code)
	result = rePreCodeBlock.ReplaceAllStringFunc(result, func(m string) string {
		sub := rePreCodeBlock.FindStringSubmatch(m)
		if len(sub) > 2 {
			lang := sub[1]
			content := sub[2]
			if lang != "" {
				return fmt.Sprintf("\n<pre><code class=\"language-%s\">%s</code></pre>\n", lang, content)
			}
			return fmt.Sprintf("\n<pre>%s</pre>\n", content)
		}
		return m
	})

	// Inline code
	result = reInlineCode.ReplaceAllString(result, "<code>$1</code>")

	// Bold
	result = reBold.ReplaceAllString(result, "<b>$1</b>")

	// Italic
	result = reItalic.ReplaceAllString(result, "<i>$1</i>")

	// Strikethrough: <del>, <strike>, <s> -> <s>
	result = reStrike.ReplaceAllString(result, "<s>$1</s>")

	// Underline: <ins>, <u> -> <u>
	result = reUnderline.ReplaceAllString(result, "<u>$1</u>")

	// Highlight/mark -> bold fallback
	result = reMark.ReplaceAllString(result, "<b>$1</b>")

	// Links: preserve <a href="...">text</a>
	result = reAnchorHref.ReplaceAllStringFunc(result, func(m string) string {
		sub := reAnchorHref.FindStringSubmatch(m)
		if len(sub) > 2 {
			href := sub[1]
			text := sub[2]
			linkText := strings.TrimSpace(stripTags(text))
			if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
				return linkText
			}
			return fmt.Sprintf("<a href=\"%s\">%s</a>", href, linkText)
		}
		return m
	})

	// Blockquote -> Telegram <blockquote>
	result = reBlockquote.ReplaceAllStringFunc(result, func(m string) string {
		sub := reBlockquote.FindStringSubmatch(m)
		if len(sub) > 1 {
			return fmt.Sprintf("\n<blockquote>%s</blockquote>\n", strings.TrimSpace(stripTags(sub[1])))
		}
		return m
	})

	// Context-scope mentions (Antigravity-specific)
	result = reContextScope.ReplaceAllStringFunc(result, func(m string) string {
		sub := reContextScope.FindStringSubmatch(m)
		if len(sub) > 1 {
			return fmt.Sprintf("<code>%s</code>", strings.TrimSpace(stripTags(sub[1])))
		}
		return m
	})

	// Elements with title attributes (file paths)
	result = reTitleAttr.ReplaceAllStringFunc(result, func(m string) string {
		sub := reTitleAttr.FindStringSubmatch(m)
		if len(sub) > 2 {
			title := sub[1]
			text := sub[2]
			if looksLikeFilePath(title) {
				return fmt.Sprintf("%s%s", title, strings.TrimSpace(stripTags(text)))
			}
			return stripTags(text)
		}
		return m
	})

	// Images -> show alt text
	result = reImgWithAlt.ReplaceAllStringFunc(result, func(m string) string {
		sub := reImgWithAlt.FindStringSubmatch(m)
		if len(sub) > 1 && sub[1] != "" {
			return fmt.Sprintf("[%s]", sub[1])
		}
		return ""
	})
	result = reImgNoAlt.ReplaceAllString(result, "")

	// Details/summary -> show as expandable content
	result = reDetailsSummary.ReplaceAllStringFunc(result, func(m string) string {
		sub := reDetailsSummary.FindStringSubmatch(m)
		if len(sub) > 2 {
			summaryText := strings.TrimSpace(stripTags(sub[1]))
			contentText := strings.TrimSpace(stripTags(sub[2]))
			if contentText != "" {
				return fmt.Sprintf("\n<b>%s</b>\n%s\n", summaryText, contentText)
			}
			return fmt.Sprintf("\n<b>%s</b>\n", summaryText)
		}
		return m
	})

	// Figure/figcaption -> show caption
	result = reFigureCaption.ReplaceAllStringFunc(result, func(m string) string {
		sub := reFigureCaption.FindStringSubmatch(m)
		if len(sub) > 2 {
			return fmt.Sprintf("%s\n<i>%s</i>\n", strings.TrimSpace(stripTags(sub[1])), strings.TrimSpace(stripTags(sub[2])))
		}
		return m
	})
	result = reFigureOnly.ReplaceAllString(result, "$1")

	// Convert HTML tables to monospace ASCII tables inside <pre>
	result = reTable.ReplaceAllStringFunc(result, func(m string) string {
		sub := reTable.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		tableContent := sub[1]
		var rows [][]string
		rowMatches := reTableRow.FindAllStringSubmatch(tableContent, -1)
		for _, rowMatch := range rowMatches {
			if len(rowMatch) > 1 {
				var cells []string
				cellMatches := reTableCell.FindAllStringSubmatch(rowMatch[1], -1)
				for _, cellMatch := range cellMatches {
					if len(cellMatch) > 1 {
						cellRaw := strings.TrimSpace(stripTags(cellMatch[1]))
						// Telegram HTML requires escaping < > & even inside <pre>
						escaped := strings.ReplaceAll(cellRaw, "&", "&amp;")
						escaped = strings.ReplaceAll(escaped, "<", "&lt;")
						escaped = strings.ReplaceAll(escaped, ">", "&gt;")
						cells = append(cells, escaped)
					}
				}
				if len(cells) > 0 {
					rows = append(rows, cells)
				}
			}
		}

		if len(rows) == 0 {
			return ""
		}

		colCount := 0
		for _, r := range rows {
			if len(r) > colCount {
				colCount = len(r)
			}
		}

		colWidths := make([]int, colCount)
		for c := 0; c < colCount; c++ {
			maxW := 1
			for _, r := range rows {
				if c < len(r) && len(r[c]) > maxW {
					maxW = len(r[c])
				}
			}
			colWidths[c] = maxW
		}

		var lines []string
		for ri, row := range rows {
			var paddedCells []string
			for ci, cell := range row {
				w := colWidths[ci]
				padded := cell
				if len(cell) < w {
					padded += strings.Repeat(" ", w-len(cell))
				}
				paddedCells = append(paddedCells, padded)
			}
			lines = append(lines, "| "+strings.Join(paddedCells, " | ")+" |")
			if ri == 0 {
				var separators []string
				for _, w := range colWidths {
					separators = append(separators, strings.Repeat("-", w+2))
				}
				lines = append(lines, "|"+strings.Join(separators, "|")+"|")
			}
		}

		return fmt.Sprintf("\n<pre>%s</pre>\n", strings.Join(lines, "\n"))
	})

	// Definition lists: <dl>, <dt>, <dd>
	result = reDefinitionList.ReplaceAllStringFunc(result, func(m string) string {
		sub := reDefinitionList.FindStringSubmatch(m)
		if len(sub) > 1 {
			content := sub[1]
			output := reDt.ReplaceAllStringFunc(content, func(dtMatch string) string {
				dtSub := reDt.FindStringSubmatch(dtMatch)
				if len(dtSub) > 1 {
					return fmt.Sprintf("<b>%s</b>\n", strings.TrimSpace(stripTags(dtSub[1])))
				}
				return dtMatch
			})
			output = reDd.ReplaceAllStringFunc(output, func(ddMatch string) string {
				ddSub := reDd.FindStringSubmatch(ddMatch)
				if len(ddSub) > 1 {
					return fmt.Sprintf("  %s\n", strings.TrimSpace(stripTags(ddSub[1])))
				}
				return ddMatch
			})
			return fmt.Sprintf("\n\n%s", output)
		}
		return m
	})

	// Superscript/subscript -> inline text
	result = reSup.ReplaceAllStringFunc(result, func(m string) string {
		sub := reSup.FindStringSubmatch(m)
		if len(sub) > 1 {
			return fmt.Sprintf("^%s", strings.TrimSpace(stripTags(sub[1])))
		}
		return m
	})
	result = reSub.ReplaceAllStringFunc(result, func(m string) string {
		sub := reSub.FindStringSubmatch(m)
		if len(sub) > 1 {
			return fmt.Sprintf("_%s", strings.TrimSpace(stripTags(sub[1])))
		}
		return m
	})

	// Block-level elements
	result = reParagraph.ReplaceAllString(result, "$1\n\n")
	result = reDiv.ReplaceAllString(result, "$1\n")

	// Lists (iterative for nested lists up to 5 iterations)
	for iteration := 0; iteration < 5; iteration++ {
		if !reListCheck.MatchString(result) {
			break
		}
		newResult, changed := replaceInnermostLists(result)
		if !changed {
			break
		}
		result = newResult
	}

	// Strip remaining HTML tags except the Telegram-supported ones
	result = stripUnsupportedTags(result)

	result = decodeSafeEntities(result)
	result = reMultiNewline.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return result
}

// replaceInnermostLists replaces the innermost <ul> and <ol> blocks in the string.
func replaceInnermostLists(html string) (string, bool) {
	lower := strings.ToLower(html)
	changed := false
	result := html

	for {
		lower = strings.ToLower(result)
		// Find first closing </ul> or </ol>
		cUl := strings.Index(lower, "</ul>")
		cOl := strings.Index(lower, "</ol>")
		if cUl == -1 && cOl == -1 {
			break
		}

		closeIdx := -1
		isUl := false
		if cUl != -1 && (cOl == -1 || cUl < cOl) {
			closeIdx = cUl
			isUl = true
		} else {
			closeIdx = cOl
			isUl = false
		}

		// Find the last <ul or <ol before closeIdx
		beforeClose := lower[:closeIdx]
		lastUl := strings.LastIndex(beforeClose, "<ul")
		lastOl := strings.LastIndex(beforeClose, "<ol")

		openIdx := lastUl
		if isUl {
			openIdx = lastUl
		} else {
			openIdx = lastOl
		}

		// If for some reason the closest opening tag is of the other type, adjust
		if lastUl > openIdx && isUl {
			openIdx = lastUl
		} else if lastOl > openIdx && !isUl {
			openIdx = lastOl
		}

		if openIdx == -1 {
			// No matching opening tag before close tag; remove broken close tag to prevent loop
			closeTagLen := 5
			result = result[:closeIdx] + result[closeIdx+closeTagLen:]
			changed = true
			continue
		}

		// Find end of opening tag '>'
		openTagEnd := strings.Index(result[openIdx:], ">")
		if openTagEnd == -1 {
			break
		}
		contentStart := openIdx + openTagEnd + 1
		content := result[contentStart:closeIdx]
		closeTagLen := 5

		var itemsOutput string
		if isUl {
			itemsOutput = reListItem.ReplaceAllStringFunc(content, func(m string) string {
				sub := reListItem.FindStringSubmatch(m)
				if len(sub) > 1 {
					return fmt.Sprintf("- %s\n", strings.TrimSpace(stripTags(sub[1])))
				}
				return m
			})
		} else {
			counter := 0
			itemsOutput = reListItem.ReplaceAllStringFunc(content, func(m string) string {
				sub := reListItem.FindStringSubmatch(m)
				if len(sub) > 1 {
					counter++
					return fmt.Sprintf("%d. %s\n", counter, strings.TrimSpace(stripTags(sub[1])))
				}
				return m
			})
		}

		replacement := "\n" + itemsOutput
		result = result[:openIdx] + replacement + result[closeIdx+closeTagLen:]
		changed = true
	}

	return result, changed
}

func looksLikeFilePath(value string) bool {
	if value == "" {
		return false
	}
	return strings.Contains(value, "/") && reFilePath.MatchString(value)
}

func stripTags(html string) string {
	return reAllTags.ReplaceAllString(html, "")
}

// stripUnsupportedTags strips HTML tags except the ones Telegram supports.
func stripUnsupportedTags(html string) string {
	return reAllTags.ReplaceAllStringFunc(html, func(tag string) string {
		inner := tag
		if strings.HasPrefix(inner, "</") {
			inner = inner[2:]
		} else if strings.HasPrefix(inner, "<") {
			inner = inner[1:]
		}
		if strings.HasSuffix(inner, ">") {
			inner = inner[:len(inner)-1]
		}
		if reAllowedTag.MatchString(inner) {
			return tag
		}
		return ""
	})
}

// decodeSafeEntities decodes HTML entities that are safe for Telegram HTML mode.
// Preserves &amp; &lt; &gt; as-is since Telegram requires them escaped.
func decodeSafeEntities(text string) string {
	res := strings.ReplaceAll(text, "&quot;", "\"")
	res = strings.ReplaceAll(res, "&#39;", "'")
	res = strings.ReplaceAll(res, "&#x27;", "'")
	res = strings.ReplaceAll(res, "&nbsp;", " ")

	res = reHexEntity.ReplaceAllStringFunc(res, func(m string) string {
		sub := reHexEntity.FindStringSubmatch(m)
		if len(sub) > 1 {
			cp, err := strconv.ParseInt(sub[1], 16, 32)
			if err == nil {
				ch := string(rune(cp))
				switch ch {
				case "&":
					return "&amp;"
				case "<":
					return "&lt;"
				case ">":
					return "&gt;"
				default:
					return ch
				}
			}
		}
		return m
	})

	res = reDecEntity.ReplaceAllStringFunc(res, func(m string) string {
		sub := reDecEntity.FindStringSubmatch(m)
		if len(sub) > 1 {
			cp, err := strconv.ParseInt(sub[1], 10, 32)
			if err == nil {
				ch := string(rune(cp))
				switch ch {
				case "&":
					return "&amp;"
				case "<":
					return "&lt;"
				case ">":
					return "&gt;"
				default:
					return ch
				}
			}
		}
		return m
	})

	return res
}
