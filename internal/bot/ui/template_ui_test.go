package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func makeTestTemplate(id int, name, prompt string) TemplateItem {
	return TemplateItem{
		ID:        id,
		Name:      name,
		Prompt:    prompt,
		CreatedAt: "2026-01-01T00:00:00Z",
	}
}

func TestSendTemplateUI_Empty(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	var sentText string
	var sentKeyboard *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		callCount++
		sentText = text
		sentKeyboard = keyboard
		return nil
	}

	err := SendTemplateUI(ctx, sendFn, []TemplateItem{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if !strings.Contains(sentText, "No templates registered") {
		t.Errorf("expected 'No templates registered', got: %s", sentText)
	}
	if !strings.Contains(sentText, "template_add") {
		t.Errorf("expected 'template_add', got: %s", sentText)
	}
	if sentKeyboard == nil || len(sentKeyboard.InlineKeyboard) != 0 {
		t.Errorf("expected empty keyboard, got %+v", sentKeyboard)
	}
}

func TestSendTemplateUI_WithTemplates(t *testing.T) {
	ctx := context.Background()
	templates := []TemplateItem{
		makeTestTemplate(1, "daily-report", "Write a daily report"),
		makeTestTemplate(2, "code-review", "Review the latest code changes"),
	}

	var sentText string
	var sentKeyboard *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		sentText = text
		sentKeyboard = keyboard
		return nil
	}

	err := SendTemplateUI(ctx, sendFn, templates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sentText, "Template Management") {
		t.Errorf("expected 'Template Management', got: %s", sentText)
	}
	if !strings.Contains(sentText, "daily-report") {
		t.Errorf("expected 'daily-report', got: %s", sentText)
	}
	if !strings.Contains(sentText, "code-review") {
		t.Errorf("expected 'code-review', got: %s", sentText)
	}
	if !strings.Contains(sentText, "(2)") {
		t.Errorf("expected '(2)', got: %s", sentText)
	}

	if sentKeyboard == nil || len(sentKeyboard.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 keyboard rows, got %+v", sentKeyboard)
	}

	btn0 := sentKeyboard.InlineKeyboard[0][0]
	if btn0.Text != "▶ daily-report" || btn0.CallbackData != "template_btn_1" {
		t.Errorf("unexpected button 0: %+v", btn0)
	}
	btn1 := sentKeyboard.InlineKeyboard[1][0]
	if btn1.Text != "▶ code-review" || btn1.CallbackData != "template_btn_2" {
		t.Errorf("unexpected button 1: %+v", btn1)
	}
}

func TestSendTemplateUI_TruncatesLongPrompts(t *testing.T) {
	ctx := context.Background()
	longPrompt := strings.Repeat("A", 100)
	templates := []TemplateItem{
		makeTestTemplate(1, "long", longPrompt),
	}

	var sentText string
	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		sentText = text
		return nil
	}

	err := SendTemplateUI(ctx, sendFn, templates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sentText, "...") {
		t.Errorf("expected '...' in text, got: %s", sentText)
	}
	if strings.Contains(sentText, longPrompt) {
		t.Errorf("did not expect full long prompt, got: %s", sentText)
	}
}

func TestSendTemplateUI_CapsAt25Buttons(t *testing.T) {
	ctx := context.Background()
	templates := make([]TemplateItem, 30)
	for i := 0; i < 30; i++ {
		templates[i] = makeTestTemplate(i+1, fmt.Sprintf("tpl-%d", i+1), fmt.Sprintf("prompt %d", i+1))
	}

	var sentText string
	var sentKeyboard *InlineKeyboardMarkup
	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		sentText = text
		sentKeyboard = keyboard
		return nil
	}

	err := SendTemplateUI(ctx, sendFn, templates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sentText, "5 templates are hidden.") {
		t.Errorf("expected '5 templates are hidden.', got: %s", sentText)
	}
	if len(sentKeyboard.InlineKeyboard) != 25 {
		t.Errorf("expected 25 buttons, got %d", len(sentKeyboard.InlineKeyboard))
	}
}

func TestParseTemplateButtonID(t *testing.T) {
	cases := []struct {
		input    string
		expected int
		ok       bool
	}{
		{"template_btn_42", 42, true},
		{"model_btn_foo", 0, false},
		{"template_btn_abc", 0, false},
		{"", 0, false},
	}

	for _, c := range cases {
		id, ok := ParseTemplateButtonID(c.input)
		if ok != c.ok || id != c.expected {
			t.Errorf("ParseTemplateButtonID(%q) = (%d, %v), expected (%d, %v)",
				c.input, id, ok, c.expected, c.ok)
		}
	}
}
