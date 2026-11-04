package ui

import (
	"context"
	"strings"
	"testing"
)

type mockAutoAcceptService struct {
	enabled bool
}

func (m *mockAutoAcceptService) IsEnabled() bool {
	return m.enabled
}

func TestSendAutoAcceptUI_Disabled(t *testing.T) {
	ctx := context.Background()
	service := &mockAutoAcceptService{enabled: false}

	callCount := 0
	var sentText string
	var sentKeyboard *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		callCount++
		sentText = text
		sentKeyboard = keyboard
		return nil
	}

	err := SendAutoAcceptUI(ctx, sendFn, service)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected sendFn to be called 1 time, got %d", callCount)
	}
	if !strings.Contains(sentText, "Auto-accept Management") {
		t.Errorf("expected text to contain 'Auto-accept Management', got: %s", sentText)
	}
	if !strings.Contains(sentText, "⚪ OFF") {
		t.Errorf("expected text to contain '⚪ OFF', got: %s", sentText)
	}
	if sentKeyboard == nil {
		t.Fatal("expected sentKeyboard to be non-nil")
	}

	// Verify buttons
	if len(sentKeyboard.InlineKeyboard) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(sentKeyboard.InlineKeyboard))
	}
	row0 := sentKeyboard.InlineKeyboard[0]
	if len(row0) != 2 {
		t.Fatalf("expected 2 buttons in row 0, got %d", len(row0))
	}
	if row0[0].Text != "Turn ON" || row0[0].CallbackData != AutoAcceptBtnOn {
		t.Errorf("unexpected button 0: %+v", row0[0])
	}
	if row0[1].Text != "🔴 Turn OFF" || row0[1].CallbackData != AutoAcceptBtnOff {
		t.Errorf("unexpected button 1: %+v", row0[1])
	}

	row1 := sentKeyboard.InlineKeyboard[1]
	if len(row1) != 1 {
		t.Fatalf("expected 1 button in row 1, got %d", len(row1))
	}
	if row1[0].Text != "🔄 Refresh" || row1[0].CallbackData != AutoAcceptBtnRefresh {
		t.Errorf("unexpected refresh button: %+v", row1[0])
	}
}

func TestSendAutoAcceptUI_Enabled(t *testing.T) {
	ctx := context.Background()
	service := &mockAutoAcceptService{enabled: true}

	var sentText string
	var sentKeyboard *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		sentText = text
		sentKeyboard = keyboard
		return nil
	}

	err := SendAutoAcceptUI(ctx, sendFn, service)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sentText, "🟢 ON") {
		t.Errorf("expected text to contain '🟢 ON', got: %s", sentText)
	}
	if sentKeyboard == nil {
		t.Fatal("expected sentKeyboard to be non-nil")
	}

	row0 := sentKeyboard.InlineKeyboard[0]
	if row0[0].Text != "✅ Turn ON" {
		t.Errorf("expected '✅ Turn ON', got %q", row0[0].Text)
	}
	if row0[1].Text != "Turn OFF" {
		t.Errorf("expected 'Turn OFF', got %q", row0[1].Text)
	}
}
