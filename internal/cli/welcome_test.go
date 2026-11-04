package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVisibleLength(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 5},
		{"\x1b[36mhello\x1b[0m", 5},
		{"\x1b[1m\x1b[32mhello world\x1b[0m", 11},
		{"\x1b[2mTroubleshoot: \x1b[0m\x1b[32mantigravity-telegram-remote doctor\x1b[0m", 48},
		{"Xin chào", 8},
	}

	for _, tt := range tests {
		actual := VisibleLength(tt.input)
		if actual != tt.expected {
			t.Errorf("VisibleLength(%q) = %d; want %d", tt.input, actual, tt.expected)
		}
	}
}

func TestFormatWelcomeBox(t *testing.T) {
	ver := "1.2.3"
	box := FormatWelcomeBox(ver)

	lines := strings.Split(box, "\n")
	if len(lines) != 21 {
		t.Fatalf("expected 21 lines in welcome box, got %d", len(lines))
	}

	// First and last lines must be empty
	if lines[0] != "" {
		t.Errorf("expected first line to be empty, got %q", lines[0])
	}
	if lines[20] != "" {
		t.Errorf("expected last line to be empty, got %q", lines[20])
	}

	// Border top, bottom, and empty separator lines must have visual width of 52 runes (1 + 50 + 1)
	expected52Lines := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 15, 17, 18, 19}
	for _, idx := range expected52Lines {
		line := lines[idx]
		vLen := VisibleLength(line)
		if vLen != 52 {
			t.Errorf("line %d visual length = %d; want 52 (line: %q)", idx, vLen, line)
		}
	}

	// Check content keywords
	expectedKeywords := []string{
		"Antigravity Telegram Remote",
		"v1.2.3",
		"Control Antigravity from your phone",
		"Quick start:",
		"antigravity-telegram-remote setup",
		"antigravity-telegram-remote open",
		"antigravity-telegram-remote start",
		"antigravity-telegram-remote doctor",
		"github.com/optimistengineer/remoat",
	}

	for _, kw := range expectedKeywords {
		if !strings.Contains(box, kw) {
			t.Errorf("expected welcome box to contain %q", kw)
		}
	}
}

func TestPrintWelcome(t *testing.T) {
	var buf bytes.Buffer
	PrintWelcome(&buf)

	output := buf.String()
	if !strings.Contains(output, "Antigravity Telegram Remote") {
		t.Errorf("PrintWelcome output missing app title: %s", output)
	}
	if !strings.Contains(output, Version) {
		t.Errorf("PrintWelcome output missing Version (%s)", Version)
	}
}
