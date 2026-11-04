package formatter

import (
	"strings"
	"testing"
)

func TestEscapeHtml(t *testing.T) {
	t.Run("escapes &, <, >", func(t *testing.T) {
		got := EscapeHtml("a & b < c > d")
		expected := "a &amp; b &lt; c &gt; d"
		if got != expected {
			t.Fatalf("EscapeHtml() = %q, want %q", got, expected)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		if got := EscapeHtml(""); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})

	t.Run("handles strings with no special chars", func(t *testing.T) {
		if got := EscapeHtml("hello world"); got != "hello world" {
			t.Fatalf("expected hello world, got %q", got)
		}
	})

	t.Run("escapes multiple occurrences", func(t *testing.T) {
		if got := EscapeHtml("<<>>"); got != "&lt;&lt;&gt;&gt;" {
			t.Fatalf("expected &lt;&lt;&gt;&gt;, got %q", got)
		}
	})
}

func TestFormatForTelegram(t *testing.T) {
	t.Run("wraps code blocks in <pre>", func(t *testing.T) {
		text := "```\nconst x = 1;\n```"
		result := FormatForTelegram(text)
		if !strings.Contains(result, "<pre>") || !strings.Contains(result, "</pre>") {
			t.Fatalf("expected <pre>...</pre>, got %q", result)
		}
	})

	t.Run("supports language-tagged code blocks", func(t *testing.T) {
		text := "```typescript\nconst x = 1;\n```"
		result := FormatForTelegram(text)
		if !strings.Contains(result, "language-typescript") {
			t.Fatalf("expected language-typescript, got %q", result)
		}
	})

	t.Run("escapes HTML inside code blocks", func(t *testing.T) {
		text := "```\n<div>test</div>\n```"
		result := FormatForTelegram(text)
		if !strings.Contains(result, "&lt;div&gt;") {
			t.Fatalf("expected &lt;div&gt;, got %q", result)
		}
	})

	t.Run("converts markdown headings to bold", func(t *testing.T) {
		if result := FormatForTelegram("## Heading"); !strings.Contains(result, "<b>Heading</b>") {
			t.Fatalf("expected <b>Heading</b>, got %q", result)
		}
	})

	t.Run("converts **bold** to <b>", func(t *testing.T) {
		if result := FormatForTelegram("**bold**"); !strings.Contains(result, "<b>bold</b>") {
			t.Fatalf("expected <b>bold</b>, got %q", result)
		}
	})

	t.Run("converts *italic* to <i>", func(t *testing.T) {
		if result := FormatForTelegram("*italic*"); !strings.Contains(result, "<i>italic</i>") {
			t.Fatalf("expected <i>italic</i>, got %q", result)
		}
	})

	t.Run("converts ~~strikethrough~~ to <s>", func(t *testing.T) {
		if result := FormatForTelegram("~~deleted~~"); !strings.Contains(result, "<s>deleted</s>") {
			t.Fatalf("expected <s>deleted</s>, got %q", result)
		}
	})

	t.Run("converts `inline code` to <code>", func(t *testing.T) {
		if result := FormatForTelegram("use `npm install`"); !strings.Contains(result, "<code>npm install</code>") {
			t.Fatalf("expected <code>npm install</code>, got %q", result)
		}
	})

	t.Run("converts markdown links to HTML links", func(t *testing.T) {
		result := FormatForTelegram("[click](https://example.com)")
		if !strings.Contains(result, `<a href="https://example.com">click</a>`) {
			t.Fatalf("expected <a href=...>, got %q", result)
		}
	})

	t.Run("wraps table lines in <pre>", func(t *testing.T) {
		text := "| Name | Age |\n| --- | --- |\n| Alice | 30 |"
		result := FormatForTelegram(text)
		if !strings.Contains(result, "<pre>") {
			t.Fatalf("expected table in <pre>, got %q", result)
		}
	})

	t.Run("wraps tree lines in <pre>", func(t *testing.T) {
		text := "├── src\n│   ├── index.ts\n└── package.json"
		result := FormatForTelegram(text)
		if !strings.Contains(result, "<pre>") {
			t.Fatalf("expected tree lines in <pre>, got %q", result)
		}
	})

	t.Run("handles blockquote lines (> text)", func(t *testing.T) {
		result := FormatForTelegram("> This is a quote")
		if !strings.Contains(result, "<blockquote>") || !strings.Contains(result, "This is a quote") {
			t.Fatalf("expected blockquote, got %q", result)
		}
	})

	t.Run("closes unclosed code blocks", func(t *testing.T) {
		text := "```\nconst x = 1;"
		result := FormatForTelegram(text)
		if !strings.Contains(result, "</pre>") {
			t.Fatalf("expected unclosed code block closed with </pre>, got %q", result)
		}
	})

	t.Run("handles horizontal rules", func(t *testing.T) {
		if result := FormatForTelegram("---"); !strings.Contains(result, "---") {
			t.Fatalf("expected horizontal rule ---, got %q", result)
		}
	})

	t.Run("wraps file references in <code>", func(t *testing.T) {
		result := FormatForTelegram("See src/utils/config.ts for details")
		if !strings.Contains(result, "<code>src/utils/config.ts</code>") {
			t.Fatalf("expected <code>src/utils/config.ts</code>, got %q", result)
		}
	})
}

func TestSplitOutputAndLogs(t *testing.T) {
	t.Run("returns empty output and logs for empty input", func(t *testing.T) {
		result := SplitOutputAndLogs("")
		if result.Output != "" || result.Logs != "" {
			t.Fatalf("expected empty result, got output=%q, logs=%q", result.Output, result.Logs)
		}
	})

	t.Run("classifies UI chrome lines as logs", func(t *testing.T) {
		text := "analyzed\nHello world\nreading files"
		result := SplitOutputAndLogs(text)
		if !strings.Contains(result.Output, "Hello world") {
			t.Errorf("expected output to contain 'Hello world', got %q", result.Output)
		}
		if !strings.Contains(result.Logs, "analyzed") {
			t.Errorf("expected logs to contain 'analyzed', got %q", result.Logs)
		}
		if !strings.Contains(result.Logs, "reading files") {
			t.Errorf("expected logs to contain 'reading files', got %q", result.Logs)
		}
	})

	t.Run("classifies activity log lines as logs", func(t *testing.T) {
		text := "reading src/index.ts\nHere is the result\nwriting output.txt"
		result := SplitOutputAndLogs(text)
		if !strings.Contains(result.Output, "Here is the result") {
			t.Errorf("expected output to contain 'Here is the result', got %q", result.Output)
		}
		if !strings.Contains(result.Logs, "reading src/index.ts") {
			t.Errorf("expected logs to contain 'reading src/index.ts', got %q", result.Logs)
		}
	})

	t.Run("preserves code blocks as output", func(t *testing.T) {
		text := "```\nconst x = 1;\n```"
		result := SplitOutputAndLogs(text)
		if !strings.Contains(result.Output, "const x = 1") {
			t.Errorf("expected output to contain 'const x = 1', got %q", result.Output)
		}
	})

	t.Run("handles text with only UI chrome", func(t *testing.T) {
		text := "analyzed\nthinking\nprocessing"
		result := SplitOutputAndLogs(text)
		if result.Output != "" {
			t.Errorf("expected empty output, got %q", result.Output)
		}
		if len(result.Logs) == 0 {
			t.Errorf("expected logs length > 0, got %q", result.Logs)
		}
	})

	t.Run("handles text with no chrome lines", func(t *testing.T) {
		text := "Hello world\nThis is output"
		result := SplitOutputAndLogs(text)
		if !strings.Contains(result.Output, "Hello world") {
			t.Errorf("expected output to contain 'Hello world', got %q", result.Output)
		}
		if result.Logs != "" {
			t.Errorf("expected empty logs, got %q", result.Logs)
		}
	})
}

func TestSeparateOutputForDelivery(t *testing.T) {
	t.Run("uses dom-structured source when available", func(t *testing.T) {
		domOutput := "structured output"
		result := SeparateOutputForDelivery(SeparateOutputOptions{
			RawText:          "raw text",
			DomSource:        "dom-structured",
			DomOutputText:    &domOutput,
			DomActivityLines: []string{"line1", "line2"},
		})
		if result.Source != "dom-structured" {
			t.Errorf("expected source dom-structured, got %q", result.Source)
		}
		if result.Output != "structured output" {
			t.Errorf("expected output 'structured output', got %q", result.Output)
		}
		if result.Logs != "line1\nline2" {
			t.Errorf("expected logs 'line1\\nline2', got %q", result.Logs)
		}
	})

	t.Run("falls back to legacy-fallback when domSource is not structured", func(t *testing.T) {
		result := SeparateOutputForDelivery(SeparateOutputOptions{
			RawText:   "Hello world",
			DomSource: "legacy-fallback",
		})
		if result.Source != "legacy-fallback" {
			t.Errorf("expected source legacy-fallback, got %q", result.Source)
		}
		if !strings.Contains(result.Output, "Hello world") {
			t.Errorf("expected output to contain 'Hello world', got %q", result.Output)
		}
	})

	t.Run("falls back when domOutputText is nil", func(t *testing.T) {
		result := SeparateOutputForDelivery(SeparateOutputOptions{
			RawText:       "Hello world",
			DomSource:     "dom-structured",
			DomOutputText: nil,
		})
		if result.Source != "legacy-fallback" {
			t.Errorf("expected source legacy-fallback, got %q", result.Source)
		}
	})
}

func TestSanitizeActivityLines(t *testing.T) {
	t.Run("removes UI chrome lines", func(t *testing.T) {
		result := SanitizeActivityLines("analyzed\nreal content\nthinking")
		if result != "real content" {
			t.Errorf("expected 'real content', got %q", result)
		}
	})

	t.Run("deduplicates lines", func(t *testing.T) {
		result := SanitizeActivityLines("foo\nfoo\nbar")
		if result != "foo\nbar" {
			t.Errorf("expected 'foo\\nbar', got %q", result)
		}
	})

	t.Run("trims and filters empty lines", func(t *testing.T) {
		result := SanitizeActivityLines("  hello  \n\n  world  ")
		if result != "hello\nworld" {
			t.Errorf("expected 'hello\\nworld', got %q", result)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		if result := SanitizeActivityLines(""); result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})
}
