package formatter

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildModeModelLines(t *testing.T) {
	t.Run("consolidates into a single line when both models are the same", func(t *testing.T) {
		lines := BuildModeModelLines("Fast", "Gemini 3.1 Pro (High)", "Gemini 3.1 Pro (High)")
		expected := []string{
			"Current mode: Fast",
			"Model: Gemini 3.1 Pro (High)",
		}
		if !reflect.DeepEqual(lines, expected) {
			t.Fatalf("BuildModeModelLines() = %v, want %v", lines, expected)
		}
	})

	t.Run("shows both Fast and Plan models when they differ", func(t *testing.T) {
		lines := BuildModeModelLines("Planning", "Claude Opus 4.6", "Gemini 3.1 Pro")
		expected := []string{
			"Current mode: Planning",
			"Fast Model: Claude Opus 4.6",
			"Plan Model: Gemini 3.1 Pro",
		}
		if !reflect.DeepEqual(lines, expected) {
			t.Fatalf("BuildModeModelLines() = %v, want %v", lines, expected)
		}
	})
}

func TestShouldSkipActivityLog(t *testing.T) {
	t.Run("filters out duplicate logs that only contain mode/model names", func(t *testing.T) {
		if !ShouldSkipActivityLog("Fast", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for mode duplicate")
		}
		if !ShouldSkipActivityLog("Gemini 3.1 Pro", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for model duplicate")
		}
	})

	t.Run("filters out generic status logs like Generating", func(t *testing.T) {
		if !ShouldSkipActivityLog("Generating.", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for Generating.")
		}
		if !ShouldSkipActivityLog("Thinking...", "Planning", "Claude") {
			t.Error("expected true for Thinking...")
		}
	})

	t.Run("filters out ambiguous single-word activity logs", func(t *testing.T) {
		if !ShouldSkipActivityLog("create", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for 'create'")
		}
		if !ShouldSkipActivityLog("ready", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for 'ready'")
		}
		if !ShouldSkipActivityLog("pull.", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for 'pull.'")
		}
	})

	t.Run("filters out analyzed-type scan logs", func(t *testing.T) {
		if !ShouldSkipActivityLog("Analyzed.gemini/antigravity/global_skills", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for scan log")
		}
		if !ShouldSkipActivityLog("Analyzedantigravity_claw_development.md#L1-49", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected true for trace log")
		}
	})

	t.Run("does not filter out logs describing actual execution", func(t *testing.T) {
		if ShouldSkipActivityLog("Analyzing Name Preference", "Fast", "Gemini 3.1 Pro") {
			t.Error("expected false for actual execution log")
		}
	})
}

func TestSplitForEmbedDescription(t *testing.T) {
	t.Run("splits long text at the limit while preserving content", func(t *testing.T) {
		text := strings.Repeat("a", 1200) + "\n" + strings.Repeat("b", 1200) + "\n" + strings.Repeat("c", 1200)
		parts := SplitForEmbedDescription(text, 1000)

		if len(parts) <= 1 {
			t.Fatalf("expected more than 1 part, got %d", len(parts))
		}
		for i, part := range parts {
			if len(part) > 1000 {
				t.Fatalf("part %d length %d exceeds 1000", i, len(part))
			}
		}
		joined := strings.Join(parts, "")
		if !strings.Contains(joined, strings.Repeat("a", 500)) {
			t.Errorf("expected joined parts to contain a's")
		}
		if !strings.Contains(joined, strings.Repeat("b", 500)) {
			t.Errorf("expected joined parts to contain b's")
		}
		if !strings.Contains(joined, strings.Repeat("c", 500)) {
			t.Errorf("expected joined parts to contain c's")
		}
	})
}

func TestFitForSingleEmbedDescription(t *testing.T) {
	t.Run("returns text as-is when within the limit", func(t *testing.T) {
		text := "短いテキスト"
		if got := FitForSingleEmbedDescription(text, 50); got != text {
			t.Fatalf("FitForSingleEmbedDescription() = %q, want %q", got, text)
		}
	})

	t.Run("truncates the beginning to fit within the limit when exceeded", func(t *testing.T) {
		text := strings.Repeat("a", 200) + "\n" + strings.Repeat("b", 200)
		fitted := FitForSingleEmbedDescription(text, 120)

		if len([]rune(fitted)) > 120 {
			t.Fatalf("fitted length %d exceeds 120 runes", len([]rune(fitted)))
		}
		if !strings.HasPrefix(fitted, "...(earlier output truncated)") {
			t.Errorf("expected fitted to start with prefix, got %q", fitted)
		}
		if !strings.Contains(fitted, strings.Repeat("b", 40)) {
			t.Errorf("expected fitted to contain b's")
		}
	})
}
