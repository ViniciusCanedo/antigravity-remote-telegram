package formatter

import (
	"strings"
	"testing"
)

func TestHtmlToTelegramHtml_BasicConversion(t *testing.T) {
	if got := HtmlToTelegramHtml(""); got != "" {
		t.Fatalf("expected empty string for empty input, got %q", got)
	}

	if got := HtmlToTelegramHtml("Hello world"); got != "Hello world" {
		t.Fatalf("expected plain text preserved, got %q", got)
	}
}

func TestHtmlToTelegramHtml_FormattingTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<strong>bold</strong>", "<b>bold</b>"},
		{"<em>italic</em>", "<i>italic</i>"},
		{"<del>deleted</del>", "<s>deleted</s>"},
		{"<strike>struck</strike>", "<s>struck</s>"},
		{"<ins>underline</ins>", "<u>underline</u>"},
		{"<mark>highlighted</mark>", "<b>highlighted</b>"},
	}

	for _, tt := range tests {
		got := HtmlToTelegramHtml(tt.input)
		if got != tt.expected {
			t.Errorf("HtmlToTelegramHtml(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestHtmlToTelegramHtml_Headings(t *testing.T) {
	if got := HtmlToTelegramHtml("<h1>Title</h1>"); got != "<b>Title</b>" {
		t.Fatalf("expected <b>Title</b>, got %q", got)
	}

	if got := HtmlToTelegramHtml("<h3>Subtitle</h3>"); got != "<b>Subtitle</b>" {
		t.Fatalf("expected <b>Subtitle</b>, got %q", got)
	}

	if got := HtmlToTelegramHtml("<h2><span>Nested</span> heading</h2>"); got != "<b>Nested heading</b>" {
		t.Fatalf("expected <b>Nested heading</b>, got %q", got)
	}
}

func TestHtmlToTelegramHtml_CodeBlocks(t *testing.T) {
	t.Run("inline code", func(t *testing.T) {
		got := HtmlToTelegramHtml("<code>const x = 1</code>")
		if got != "<code>const x = 1</code>" {
			t.Fatalf("unexpected inline code result: %q", got)
		}
	})

	t.Run("pre/code with language", func(t *testing.T) {
		html := `<pre><code class="language-js">console.log("hi")</code></pre>`
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "<pre>") || !strings.Contains(got, "<code>") || !strings.Contains(got, `console.log("hi")`) {
			t.Fatalf("expected pre/code block preserved with lang, got: %q", got)
		}
	})

	t.Run("pre/code without language", func(t *testing.T) {
		html := "<pre><code>plain code</code></pre>"
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "<pre>") || !strings.Contains(got, "plain code") {
			t.Fatalf("expected pre block preserved without lang, got: %q", got)
		}
	})
}

func TestHtmlToTelegramHtml_Links(t *testing.T) {
	t.Run("preserves href", func(t *testing.T) {
		html := `<a href="https://example.com">click here</a>`
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, `<a href="https://example.com">click here</a>`) {
			t.Fatalf("expected link preserved, got %q", got)
		}
	})

	t.Run("strips # anchor", func(t *testing.T) {
		html := `<a href="#section">jump</a>`
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "jump") || strings.Contains(got, `href="#section"`) {
			t.Fatalf("expected jump without anchor link, got %q", got)
		}
	})

	t.Run("strips javascript: links", func(t *testing.T) {
		html := `<a href="javascript:alert(1)">xss</a>`
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "xss") || strings.Contains(got, "javascript:") {
			t.Fatalf("expected xss without javascript link, got %q", got)
		}
	})
}

func TestHtmlToTelegramHtml_LineBreaksAndSeparators(t *testing.T) {
	if got := HtmlToTelegramHtml("a<br>b"); !strings.Contains(got, "a\nb") {
		t.Fatalf("expected <br> converted to newline, got %q", got)
	}

	if got := HtmlToTelegramHtml("a<br/>b"); !strings.Contains(got, "a\nb") {
		t.Fatalf("expected <br/> converted to newline, got %q", got)
	}

	if got := HtmlToTelegramHtml("a<hr>b"); !strings.Contains(got, "---") {
		t.Fatalf("expected <hr> converted to separator, got %q", got)
	}
}

func TestHtmlToTelegramHtml_Blockquotes(t *testing.T) {
	html := "<blockquote>quoted text</blockquote>"
	got := HtmlToTelegramHtml(html)
	if !strings.Contains(got, "<blockquote>quoted text</blockquote>") {
		t.Fatalf("expected blockquote preserved, got %q", got)
	}
}

func TestHtmlToTelegramHtml_Lists(t *testing.T) {
	t.Run("unordered list", func(t *testing.T) {
		html := "<ul><li>item 1</li><li>item 2</li></ul>"
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "- item 1") || !strings.Contains(got, "- item 2") {
			t.Fatalf("expected items with dashes, got %q", got)
		}
	})

	t.Run("ordered list", func(t *testing.T) {
		html := "<ol><li>first</li><li>second</li></ol>"
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "1. first") || !strings.Contains(got, "2. second") {
			t.Fatalf("expected numbered items, got %q", got)
		}
	})
}

func TestHtmlToTelegramHtml_Images(t *testing.T) {
	t.Run("shows alt text", func(t *testing.T) {
		html := `<img alt="diagram" src="img.png">`
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "[diagram]") {
			t.Fatalf("expected [diagram], got %q", got)
		}
	})

	t.Run("removes images without alt", func(t *testing.T) {
		html := `before<img src="img.png">after`
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "beforeafter") {
			t.Fatalf("expected beforeafter, got %q", got)
		}
	})
}

func TestHtmlToTelegramHtml_Tables(t *testing.T) {
	t.Run("ASCII table", func(t *testing.T) {
		html := "<table><tr><th>Name</th><th>Value</th></tr><tr><td>foo</td><td>42</td></tr></table>"
		got := HtmlToTelegramHtml(html)
		if !strings.Contains(got, "Name") || !strings.Contains(got, "foo") || !strings.Contains(got, "|") {
			t.Fatalf("expected table converted to ASCII table, got %q", got)
		}
	})

	t.Run("empty table", func(t *testing.T) {
		html := "<table></table>"
		got := HtmlToTelegramHtml(html)
		if got != "" {
			t.Fatalf("expected empty result for empty table, got %q", got)
		}
	})
}

func TestHtmlToTelegramHtml_EntityDecoding(t *testing.T) {
	if got := HtmlToTelegramHtml("A &amp; B"); !strings.Contains(got, "A &amp; B") {
		t.Fatalf("expected &amp; preserved, got %q", got)
	}

	if got := HtmlToTelegramHtml("&lt;tag&gt;"); !strings.Contains(got, "&lt;tag&gt;") {
		t.Fatalf("expected &lt;tag&gt; preserved, got %q", got)
	}

	if got := HtmlToTelegramHtml("a&nbsp;b"); !strings.Contains(got, "a b") {
		t.Fatalf("expected &nbsp; decoded to space, got %q", got)
	}

	if got := HtmlToTelegramHtml("&#65;"); !strings.Contains(got, "A") {
		t.Fatalf("expected &#65; decoded to A, got %q", got)
	}

	if got := HtmlToTelegramHtml("&#x41;"); !strings.Contains(got, "A") {
		t.Fatalf("expected &#x41; decoded to A, got %q", got)
	}

	if got := HtmlToTelegramHtml("&quot;hello&quot;"); !strings.Contains(got, `"hello"`) {
		t.Fatalf("expected &quot; decoded to quote, got %q", got)
	}

	if got := HtmlToTelegramHtml("it&#39;s"); !strings.Contains(got, "it's") {
		t.Fatalf("expected &#39; decoded to apostrophe, got %q", got)
	}
}

func TestHtmlToTelegramHtml_TagStripping(t *testing.T) {
	if got := HtmlToTelegramHtml("<span>text</span>"); got != "text" {
		t.Fatalf("expected span stripped, got %q", got)
	}

	if got := HtmlToTelegramHtml("<style>.cls { color: red; }</style>Hello"); got != "Hello" {
		t.Fatalf("expected style removed, got %q", got)
	}

	if got := HtmlToTelegramHtml("<script>alert(1)</script>Hello"); got != "Hello" {
		t.Fatalf("expected script removed, got %q", got)
	}

	if got := HtmlToTelegramHtml("<div>content</div>"); got != "content" {
		t.Fatalf("expected div stripped, got %q", got)
	}

	if got := HtmlToTelegramHtml("<p>paragraph</p>"); got != "paragraph" {
		t.Fatalf("expected p stripped, got %q", got)
	}
}

func TestHtmlToTelegramHtml_DetailsSummary(t *testing.T) {
	html := "<details><summary>Click me</summary>Hidden content</details>"
	got := HtmlToTelegramHtml(html)
	if !strings.Contains(got, "<b>Click me</b>") || !strings.Contains(got, "Hidden content") {
		t.Fatalf("expected summary bold with content, got %q", got)
	}
}

func TestHtmlToTelegramHtml_SupSub(t *testing.T) {
	if got := HtmlToTelegramHtml("x<sup>2</sup>"); !strings.Contains(got, "x^2") {
		t.Fatalf("expected x^2, got %q", got)
	}

	if got := HtmlToTelegramHtml("H<sub>2</sub>O"); !strings.Contains(got, "H_2O") {
		t.Fatalf("expected H_2O, got %q", got)
	}
}
