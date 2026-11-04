package ui

import (
	"context"
	"strings"
	"testing"
)

type mockModeService struct {
	mode string
}

func (m *mockModeService) GetCurrentMode() string {
	if m.mode == "" {
		return "fast"
	}
	return m.mode
}

func (m *mockModeService) SetMode(mode string) {
	m.mode = mode
}

type mockModeCdp struct {
	called bool
	mode   string
	err    error
}

func (m *mockModeCdp) GetCurrentMode(ctx context.Context) (string, error) {
	m.called = true
	return m.mode, m.err
}

func TestSendModeUI_DefaultFast(t *testing.T) {
	ctx := context.Background()
	modeService := &mockModeService{mode: "fast"}

	callCount := 0
	var sentText string
	var sentKeyboard *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		callCount++
		sentText = text
		sentKeyboard = keyboard
		return nil
	}

	err := SendModeUI(ctx, sendFn, modeService, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if !strings.Contains(sentText, "Mode Management") {
		t.Errorf("expected 'Mode Management', got %s", sentText)
	}
	if !strings.Contains(sentText, "Current Mode:</b> Fast") {
		t.Errorf("expected Current Mode Fast in text, got: %s", sentText)
	}
	if sentKeyboard == nil || len(sentKeyboard.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 keyboard rows, got %+v", sentKeyboard)
	}

	btnFast := sentKeyboard.InlineKeyboard[0][0]
	if btnFast.Text != "✅ Fast" || btnFast.CallbackData != "mode_select:fast" {
		t.Errorf("unexpected btnFast: %+v", btnFast)
	}

	btnPlan := sentKeyboard.InlineKeyboard[1][0]
	if btnPlan.Text != "Planning" || btnPlan.CallbackData != "mode_select:plan" {
		t.Errorf("unexpected btnPlan: %+v", btnPlan)
	}
}

func TestSendModeUI_SyncsFromCdp(t *testing.T) {
	ctx := context.Background()
	modeService := &mockModeService{mode: "fast"}
	cdp := &mockModeCdp{mode: "plan"}

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		return nil
	}

	deps := &ModeUIDeps{
		GetCurrentCdp: func() ModeCdpProvider {
			return cdp
		},
	}

	err := SendModeUI(ctx, sendFn, modeService, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cdp.called {
		t.Error("expected CDP GetCurrentMode to be called")
	}
	if modeService.GetCurrentMode() != "plan" {
		t.Errorf("expected modeService to be 'plan', got %q", modeService.GetCurrentMode())
	}
}

func TestSendModeUI_DoesNotSyncWhenCdpReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	modeService := &mockModeService{mode: "fast"}
	cdp := &mockModeCdp{mode: ""}

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		return nil
	}

	deps := &ModeUIDeps{
		GetCurrentCdp: func() ModeCdpProvider {
			return cdp
		},
	}

	err := SendModeUI(ctx, sendFn, modeService, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cdp.called {
		t.Error("expected CDP GetCurrentMode to be called")
	}
	if modeService.GetCurrentMode() != "fast" {
		t.Errorf("expected modeService to remain 'fast', got %q", modeService.GetCurrentMode())
	}
}

func TestSendModeUI_NilDeps(t *testing.T) {
	ctx := context.Background()
	modeService := &mockModeService{mode: "fast"}

	callCount := 0
	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		callCount++
		return nil
	}

	err := SendModeUI(ctx, sendFn, modeService, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestSendModeUI_NilCdpInstance(t *testing.T) {
	ctx := context.Background()
	modeService := &mockModeService{mode: "fast"}

	callCount := 0
	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		callCount++
		return nil
	}

	deps := &ModeUIDeps{
		GetCurrentCdp: func() ModeCdpProvider {
			return nil
		},
	}

	err := SendModeUI(ctx, sendFn, modeService, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if modeService.GetCurrentMode() != "fast" {
		t.Errorf("expected mode to remain 'fast', got %q", modeService.GetCurrentMode())
	}
}
