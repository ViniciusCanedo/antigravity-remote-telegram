package ui

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	PlanViewBtn     = "plan_view_btn"
	PlanProceedBtn  = "plan_proceed_btn"
	PlanEditBtn     = "plan_edit_btn"
	PlanRefreshBtn  = "plan_refresh_btn"
	PlanPagePrefix  = "plan_page"
	DefaultPageSize = 3500
)

// PlanningInfo mirrors metadata extracted by PlanningDetector.
type PlanningInfo struct {
	OpenText    string
	ProceedText string
	PlanTitle   string
	PlanSummary string
	Description string
}

var (
	headingRegex    = regexp.MustCompile(`^(#{1,4})\s+(.*)`)
	boldRegex       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRegex     = regexp.MustCompile(`\*([^*\n]+?)\*`)
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	listItemRegex   = regexp.MustCompile(`^(\s*)[-*]\s+`)
)

// MarkdownToTelegramHTML converts plan markdown to Telegram HTML.
func MarkdownToTelegramHTML(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false

	for _, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmedLeft, "```") {
			if inCodeBlock {
				out = append(out, "</pre>")
				inCodeBlock = false
			} else {
				out = append(out, "<pre>")
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			out = append(out, EscapeHTML(line))
			continue
		}

		converted := EscapeHTML(line)

		// Heading: # -> bold
		if m := headingRegex.FindStringSubmatch(converted); len(m) > 2 {
			out = append(out, "<b>"+m[2]+"</b>")
			continue
		}

		// Bold: **text**
		converted = boldRegex.ReplaceAllString(converted, "<b>$1</b>")
		// Italic: *text*
		converted = italicRegex.ReplaceAllString(converted, "<i>$1</i>")
		// Inline code: `text`
		converted = inlineCodeRegex.ReplaceAllString(converted, "<code>$1</code>")
		// List items: - item or * item -> bullet
		converted = listItemRegex.ReplaceAllString(converted, "${1}• ")

		out = append(out, converted)
	}

	if inCodeBlock {
		out = append(out, "</pre>")
	}

	return strings.Join(out, "\n")
}

// BuildPlanNotificationUI builds the initial Planning Mode notification card with 4 action buttons.
func BuildPlanNotificationUI(
	info PlanningInfo,
	projectName string,
	targetChannelStr string,
) UIPayload {
	description := info.Description
	if description == "" {
		description = info.PlanSummary
	}
	if description == "" {
		description = "A plan has been generated and is awaiting your review."
	}

	planTitle := info.PlanTitle
	if planTitle == "" {
		planTitle = "Implementation Plan"
	}

	text := fmt.Sprintf(
		"📋 <b>Planning Mode</b>\n\n%s\n\n<b>Plan:</b> %s\n<b>Workspace:</b> %s",
		EscapeHTML(description),
		EscapeHTML(planTitle),
		EscapeHTML(projectName),
	)

	suffix := fmt.Sprintf("%s:%s", projectName, targetChannelStr)
	keyboard := NewInlineKeyboard().
		Text("📖 View Full Plan", fmt.Sprintf("%s:%s", PlanViewBtn, suffix)).
		Text("▶ Proceed", fmt.Sprintf("%s:%s", PlanProceedBtn, suffix)).
		Row().
		Text("✏️ Edit Plan", fmt.Sprintf("%s:%s", PlanEditBtn, suffix)).
		Text("🔄 Refresh", fmt.Sprintf("%s:%s", PlanRefreshBtn, suffix)).
		Clean()

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}
}

// PaginatePlanContent splits plan content into pages on newline boundaries up to pageSize.
// If pageSize <= 0, DefaultPageSize (3500) is used.
func PaginatePlanContent(content string, pageSize int) []string {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	if strings.TrimSpace(content) == "" {
		return []string{"(Empty plan)"}
	}
	if len(content) <= pageSize {
		return []string{content}
	}

	var pages []string
	remaining := content

	for len(remaining) > 0 {
		if len(remaining) <= pageSize {
			pages = append(pages, remaining)
			break
		}

		searchLen := pageSize + 1
		if searchLen > len(remaining) {
			searchLen = len(remaining)
		}
		splitAt := strings.LastIndexByte(remaining[:searchLen], '\n')
		if splitAt <= 0 {
			splitAt = pageSize
		}

		pages = append(pages, remaining[:splitAt])
		remaining = strings.TrimPrefix(remaining[splitAt:], "\n")
	}

	return pages
}

// BuildPlanContentUI builds a specific plan content page with pagination and action buttons.
func BuildPlanContentUI(
	pages []string,
	currentPage int,
	projectName string,
	targetChannelStr string,
) UIPayload {
	page := "(Page not found)"
	if currentPage >= 0 && currentPage < len(pages) {
		page = pages[currentPage]
	}
	totalPages := len(pages)

	text := "<b>📋 Plan Content</b>"
	if totalPages > 1 {
		text += fmt.Sprintf(" (%d/%d)", currentPage+1, totalPages)
	}
	text += "\n\n" + MarkdownToTelegramHTML(page)

	suffix := fmt.Sprintf("%s:%s", projectName, targetChannelStr)
	keyboard := NewInlineKeyboard()

	if totalPages > 1 {
		if currentPage > 0 {
			keyboard.Text("◀ Prev", fmt.Sprintf("%s:%d:%s", PlanPagePrefix, currentPage-1, suffix))
		}
		if currentPage < totalPages-1 {
			keyboard.Text("Next ▶", fmt.Sprintf("%s:%d:%s", PlanPagePrefix, currentPage+1, suffix))
		}
		keyboard.Row()
	}

	keyboard.
		Text("▶ Proceed", fmt.Sprintf("%s:%s", PlanProceedBtn, suffix)).
		Text("✏️ Edit", fmt.Sprintf("%s:%s", PlanEditBtn, suffix)).
		Row().
		Text("🔄 Refresh", fmt.Sprintf("%s:%s", PlanRefreshBtn, suffix)).
		Text("📖 View Full", fmt.Sprintf("%s:%s", PlanViewBtn, suffix)).
		Clean()

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}
}
