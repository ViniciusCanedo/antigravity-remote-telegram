package formatter

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestSplitTelegramHtml_UnderAndExactLimit(t *testing.T) {
	t.Run("returns single chunk when text is under limit", func(t *testing.T) {
		text := "Hello world"
		chunks := SplitTelegramHtml(text, 100)
		if len(chunks) != 1 || chunks[0] != text {
			t.Fatalf("expected single chunk [%q], got %v", text, chunks)
		}
	})

	t.Run("returns original text in array when exactly at limit", func(t *testing.T) {
		text := strings.Repeat("a", 100)
		chunks := SplitTelegramHtml(text, 100)
		if len(chunks) != 1 || chunks[0] != text {
			t.Fatalf("expected single chunk of length 100, got %v", chunks)
		}
	})
}

func TestSplitTelegramHtml_LineBoundaries(t *testing.T) {
	line1 := strings.Repeat("a", 40)
	line2 := strings.Repeat("b", 40)
	line3 := strings.Repeat("c", 40)
	text := line1 + "\n" + line2 + "\n" + line3
	chunks := SplitTelegramHtml(text, 85)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != line1+"\n"+line2 {
		t.Errorf("chunk 0 mismatch: got %q, want %q", chunks[0], line1+"\n"+line2)
	}
	if chunks[1] != line3 {
		t.Errorf("chunk 1 mismatch: got %q, want %q", chunks[1], line3)
	}
}

func TestSplitTelegramHtml_ReopensBoldTags(t *testing.T) {
	inner := strings.Repeat("x", 50)
	text := fmt.Sprintf("<b>%s\n%s</b>", inner, inner)
	chunks := SplitTelegramHtml(text, 60)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "</b>") {
		t.Errorf("expected chunk[0] to contain </b>, got %q", chunks[0])
	}
	reOpenBold := regexp.MustCompile(`^<b>`)
	if !reOpenBold.MatchString(chunks[1]) {
		t.Errorf("expected chunk[1] to start with <b>, got %q", chunks[1])
	}
	if !strings.Contains(chunks[1], "</b>") {
		t.Errorf("expected chunk[1] to contain </b>, got %q", chunks[1])
	}
}

func TestSplitTelegramHtml_ReopensPreCodeTags(t *testing.T) {
	codeLine1 := "const a = 1;"
	codeLine2 := "const b = 2;"
	codeLine3 := "const c = 3;"
	text := fmt.Sprintf("<pre><code>%s\n%s\n%s</code></pre>", codeLine1, codeLine2, codeLine3)
	chunks := SplitTelegramHtml(text, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	rePreOpen := regexp.MustCompile(`<pre>`)
	rePreClose := regexp.MustCompile(`</pre>`)
	for i, chunk := range chunks {
		opens := len(rePreOpen.FindAllString(chunk, -1))
		closes := len(rePreClose.FindAllString(chunk, -1))
		if opens != closes {
			t.Errorf("chunk %d has mismatched <pre> tags (opens=%d, closes=%d): %q", i, opens, closes, chunk)
		}
	}
}

func TestSplitTelegramHtml_NestedPreCodeWithClass(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	text := fmt.Sprintf("<pre><code class=\"language-ts\">%s</code></pre>", strings.Join(lines, "\n"))
	chunks := SplitTelegramHtml(text, 60)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "<pre>") {
		t.Errorf("expected chunk[0] to contain <pre>")
	}
	for i, chunk := range chunks {
		if strings.Contains(chunk, "<pre>") && !strings.Contains(chunk, "</pre>") {
			t.Errorf("chunk %d has unclosed <pre>", i)
		}
		if strings.Contains(chunk, "<code") && !strings.Contains(chunk, "</code>") {
			t.Errorf("chunk %d has unclosed <code>", i)
		}
	}
}

func TestSplitTelegramHtml_HardSplitLongLine(t *testing.T) {
	text := strings.Repeat("x", 200)
	chunks := SplitTelegramHtml(text, 80)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 200 chars at limit 80, got %d", len(chunks))
	}
	if strings.Join(chunks, "") != text {
		t.Fatalf("expected chunks joined to equal original text")
	}
}

func TestSplitTelegramHtml_EmptyAndItalicAndMaxLen(t *testing.T) {
	t.Run("handles empty string", func(t *testing.T) {
		chunks := SplitTelegramHtml("", 100)
		if len(chunks) != 1 || chunks[0] != "" {
			t.Fatalf("expected single empty string chunk, got %v", chunks)
		}
	})

	t.Run("handles text with <i> tags", func(t *testing.T) {
		text := fmt.Sprintf("<i>%s\n%s</i>", strings.Repeat("a", 30), strings.Repeat("b", 30))
		chunks := SplitTelegramHtml(text, 40)
		if len(chunks) < 2 {
			t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
		}
		reIOpen := regexp.MustCompile(`<i>`)
		reIClose := regexp.MustCompile(`</i>`)
		for i, chunk := range chunks {
			opens := len(reIOpen.FindAllString(chunk, -1))
			closes := len(reIClose.FindAllString(chunk, -1))
			if opens != closes {
				t.Errorf("chunk %d has mismatched <i> tags: %q", i, chunk)
			}
		}
	})

	t.Run("preserves content across all chunks", func(t *testing.T) {
		var lines []string
		for i := 0; i < 20; i++ {
			lines = append(lines, fmt.Sprintf("Line %d: %s", i, strings.Repeat("x", 30)))
		}
		text := strings.Join(lines, "\n")
		chunks := SplitTelegramHtml(text, 200)
		combined := strings.Join(chunks, "\n")
		for _, line := range lines {
			if !strings.Contains(combined, line) {
				t.Errorf("expected combined chunks to contain %q", line)
			}
		}
	})

	t.Run("does not produce chunks exceeding maxLen", func(t *testing.T) {
		var lines []string
		for i := 0; i < 50; i++ {
			lines = append(lines, fmt.Sprintf("Line %d: %s", i, strings.Repeat("data", 10)))
		}
		text := strings.Join(lines, "\n")
		maxLen := 300
		chunks := SplitTelegramHtml(text, maxLen)
		for i, chunk := range chunks {
			if len(chunk) > maxLen {
				t.Errorf("chunk %d length %d exceeds maxLen %d", i, len(chunk), maxLen)
			}
		}
	})

	t.Run("handles mixed tags and plain text", func(t *testing.T) {
		text := "Hello\n<b>bold text</b>\n<pre>code here</pre>\nplain text\n<i>italic</i>"
		chunks := SplitTelegramHtml(text, 40)
		if len(chunks) < 2 {
			t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
		}
		combined := strings.Join(chunks, "\n")
		if !strings.Contains(combined, "Hello") {
			t.Errorf("expected combined chunks to contain Hello")
		}
		if !strings.Contains(combined, "plain text") {
			t.Errorf("expected combined chunks to contain plain text")
		}
	})
}
