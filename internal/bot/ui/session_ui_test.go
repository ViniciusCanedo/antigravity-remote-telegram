package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestIsSessionSelectID(t *testing.T) {
	if !IsSessionSelectID(SessionSelectID + ":some_session") {
		t.Errorf("expected true for session_select:some_session")
	}
	if !IsSessionSelectID(SessionSelectID) {
		t.Errorf("expected true for exact session_select")
	}

	if IsSessionSelectID("project_select") {
		t.Errorf("expected false for project_select")
	}
	if IsSessionSelectID("mode_select") {
		t.Errorf("expected false for mode_select")
	}
	if IsSessionSelectID("") {
		t.Errorf("expected false for empty string")
	}
}

func TestBuildSessionPickerUI_MultipleSessions(t *testing.T) {
	sessions := []SessionListItem{
		{Title: "Fix login bug", IsActive: true},
		{Title: "Refactor auth", IsActive: false},
		{Title: "Add tests", IsActive: false},
	}

	res := BuildSessionPickerUI(sessions)

	if !strings.Contains(res.Text, "3") {
		t.Errorf("expected text to contain '3', got: %s", res.Text)
	}
	if res.Keyboard == nil || len(res.Keyboard.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 keyboard rows, got %+v", res.Keyboard)
	}

	btn0 := res.Keyboard.InlineKeyboard[0][0]
	if !strings.HasPrefix(btn0.Text, "✅ ") {
		t.Errorf("expected active session to have '✅ ', got %q", btn0.Text)
	}
	if btn0.CallbackData != "session_select:Fix login bug" {
		t.Errorf("unexpected callback data: %s", btn0.CallbackData)
	}

	btn1 := res.Keyboard.InlineKeyboard[1][0]
	if strings.HasPrefix(btn1.Text, "✅ ") {
		t.Errorf("inactive session should not have '✅ ', got %q", btn1.Text)
	}
}

func TestBuildSessionPickerUI_ZeroSessions(t *testing.T) {
	res := BuildSessionPickerUI([]SessionListItem{})

	if !strings.Contains(res.Text, "No sessions found") {
		t.Errorf("expected 'No sessions found', got: %s", res.Text)
	}
	if res.Keyboard == nil || len(res.Keyboard.InlineKeyboard) != 0 {
		t.Errorf("expected empty keyboard, got %+v", res.Keyboard)
	}
}

func TestBuildSessionPickerUI_TruncatesTo25Items(t *testing.T) {
	sessions := make([]SessionListItem, 30)
	for i := 0; i < 30; i++ {
		sessions[i] = SessionListItem{
			Title:    fmt.Sprintf("Session %d", i+1),
			IsActive: i == 0,
		}
	}

	res := BuildSessionPickerUI(sessions)

	if len(res.Keyboard.InlineKeyboard) != 25 {
		t.Fatalf("expected 25 buttons, got %d", len(res.Keyboard.InlineKeyboard))
	}
}
