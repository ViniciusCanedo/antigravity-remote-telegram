package ui

import (
	"fmt"
	"strings"
)

const (
	SessionSelectID         = "session_select"
	MaxSessionSelectOptions = 25
)

// SessionListItem represents a chat session item from ChatSessionService.
type SessionListItem struct {
	Title    string
	IsActive bool
}

// IsSessionSelectID checks whether customID starts with "session_select:" or is exactly "session_select".
func IsSessionSelectID(customID string) bool {
	return customID == SessionSelectID || strings.HasPrefix(customID, SessionSelectID+":")
}

// BuildSessionPickerUI constructs the interactive session picker UI (capped at 25 options).
func BuildSessionPickerUI(sessions []SessionListItem) UIPayload {
	if len(sessions) == 0 {
		return UIPayload{
			Text:     "<b>🔗 Tham gia phiên</b>\n\nKhông tìm thấy phiên nào trong bảng điều khiển của Antigravity.",
			Keyboard: NewInlineKeyboard(),
		}
	}

	text := fmt.Sprintf(
		"<b>🔗 Tham gia phiên</b>\n\nChọn một phiên để tham gia (tìm thấy %d phiên)",
		len(sessions),
	)

	keyboard := NewInlineKeyboard()
	limit := len(sessions)
	if limit > MaxSessionSelectOptions {
		limit = MaxSessionSelectOptions
	}

	for _, session := range sessions[:limit] {
		titleRunes := []rune(session.Title)

		labelTitle := session.Title
		if len(titleRunes) > 40 {
			labelTitle = string(titleRunes[:40])
		}

		label := labelTitle
		if session.IsActive {
			label = "✅ " + labelTitle
		}

		cbTitle := session.Title
		if len(titleRunes) > 49 {
			cbTitle = string(titleRunes[:49])
		}

		keyboard.Text(label, fmt.Sprintf("%s:%s", SessionSelectID, cbTitle)).Row()
	}
	keyboard.Clean()

	return UIPayload{
		Text:     text,
		Keyboard: keyboard,
	}
}
