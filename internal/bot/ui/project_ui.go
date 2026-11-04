package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	ProjectSelectID     = "project_select"
	WorkspaceSelectID   = "workspace_select"
	ProjectPagePrefix   = "project_page"
	ProjectItemsPerPage = 100
)

// ParseProjectPageID extracts the numeric page index from "project_page:<page>".
// Returns (page, true) if valid, or (0, false) if invalid (equivalent to NaN in TypeScript).
func ParseProjectPageID(customID string) (int, bool) {
	prefix := ProjectPagePrefix + ":"
	if !strings.HasPrefix(customID, prefix) {
		return 0, false
	}
	pageStr := strings.TrimPrefix(customID, prefix)
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return 0, false
	}
	return page, true
}

// IsProjectSelectID checks whether customID matches project_select, workspace_select, or project_select:<path>.
func IsProjectSelectID(customID string) bool {
	return customID == ProjectSelectID ||
		customID == WorkspaceSelectID ||
		strings.HasPrefix(customID, ProjectSelectID+":")
}

// BuildProjectListUI builds the HTML message and paginated inline keyboard for workspace selection.
func BuildProjectListUI(workspaces []string, page int) UIPayload {
	if len(workspaces) == 0 {
		return UIPayload{
			Text:     "<b>📁 Dự án</b>\n\nKhông tìm thấy dự án nào.",
			Keyboard: NewInlineKeyboard(),
		}
	}

	totalPages := int(math.Ceil(float64(len(workspaces)) / float64(ProjectItemsPerPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	safePage := page
	if safePage < 0 {
		safePage = 0
	}
	if safePage > totalPages-1 {
		safePage = totalPages - 1
	}

	start := safePage * ProjectItemsPerPage
	end := start + ProjectItemsPerPage
	if end > len(workspaces) {
		end = len(workspaces)
	}
	pageItems := workspaces[start:end]

	lines := make([]string, len(pageItems))
	for i, ws := range pageItems {
		lines[i] = fmt.Sprintf("%d. %s", start+i+1, EscapeHTML(ws))
	}

	text := "<b>📁 Dự án</b>\n\n" +
		"Chọn một dự án để tự động tạo topic và phiên làm việc\n\n" +
		strings.Join(lines, "\n")

	if totalPages > 1 {
		text += fmt.Sprintf("\n\n<i>Trang %d / %d (%d tổng số dự án)</i>", safePage+1, totalPages, len(workspaces))
	}

	keyboard := NewInlineKeyboard()
	for _, ws := range pageItems {
		label := ws
		if len(label) > 40 {
			label = label[:37] + "..."
		}
		keyboard.Text(label, fmt.Sprintf("%s:%s", ProjectSelectID, ws)).Row()
	}

	if totalPages > 1 {
		if safePage > 0 {
			keyboard.Text("◀ Trước", fmt.Sprintf("%s:%d", ProjectPagePrefix, safePage-1))
		}
		if safePage < totalPages-1 {
			keyboard.Text("Tiếp ▶", fmt.Sprintf("%s:%d", ProjectPagePrefix, safePage+1))
		}
		keyboard.Row()
	}
	keyboard.Clean()

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}
}
