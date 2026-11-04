package ui

import (
	"encoding/json"
	"testing"
)

func TestInlineKeyboard_BuilderAndClean(t *testing.T) {
	kb := NewInlineKeyboard().
		Text("Btn 1", "data_1").
		Text("Btn 2", "data_2").
		Row().
		Text("Btn 3", "data_3").
		Row().
		Clean()

	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb.InlineKeyboard))
	}
	if len(kb.InlineKeyboard[0]) != 2 {
		t.Errorf("expected row 0 to have 2 buttons, got %d", len(kb.InlineKeyboard[0]))
	}
	if len(kb.InlineKeyboard[1]) != 1 {
		t.Errorf("expected row 1 to have 1 button, got %d", len(kb.InlineKeyboard[1]))
	}

	btn1 := kb.InlineKeyboard[0][0]
	if btn1.Text != "Btn 1" || btn1.CallbackData != "data_1" {
		t.Errorf("unexpected button 1: %+v", btn1)
	}

	// Test ToTelegramModel
	tgModel := kb.ToTelegramModel()
	if tgModel == nil {
		t.Fatal("expected non-nil tgModel")
	}
	if len(tgModel.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in tgModel, got %d", len(tgModel.InlineKeyboard))
	}
	if tgModel.InlineKeyboard[0][0].Text != "Btn 1" || tgModel.InlineKeyboard[0][0].CallbackData != "data_1" {
		t.Errorf("unexpected tgModel button 1: %+v", tgModel.InlineKeyboard[0][0])
	}

	// Test JSON marshal
	data, err := json.Marshal(kb)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}
	if string(data) == "" {
		t.Error("expected non-empty json")
	}
}

func TestInlineKeyboard_NilSafety(t *testing.T) {
	var kb *InlineKeyboardMarkup
	if kb.ToTelegramModel() != nil {
		t.Error("expected nil for nil keyboard")
	}
	if kb.Clean() != nil {
		t.Error("expected nil for nil clean")
	}
}

func TestEscapeHTML(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello & World", "Hello &amp; World"},
		{"<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{"a < b && b > c", "a &lt; b &amp;&amp; b &gt; c"},
		{"Plain text", "Plain text"},
	}

	for _, c := range cases {
		actual := EscapeHTML(c.input)
		if actual != c.expected {
			t.Errorf("EscapeHTML(%q) = %q, expected %q", c.input, actual, c.expected)
		}
	}
}
