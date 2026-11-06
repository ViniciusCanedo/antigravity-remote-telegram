package monitor

import (
	"strings"
	"testing"
)

func TestProcessLogBuffer_EmojiPrefixes(t *testing.T) {
	buffer := NewProcessLogBuffer(ProcessLogBufferOptions{MaxChars: 500})

	input := strings.Join([]string{
		"Initiating Project Setup",
		"",
		"Thought for 6s",
		"",
		"jina-mcp-server / search_web",
	}, "\n")

	result := buffer.Append(input)

	if !strings.Contains(result, "🚀 Initiating Project Setup") {
		t.Errorf("expected 🚀 emoji, got %q", result)
	}
	if !strings.Contains(result, "🧠 Thought for 6s") {
		t.Errorf("expected 🧠 emoji, got %q", result)
	}
	if !strings.Contains(result, "🛠️ jina-mcp-server / search_web") {
		t.Errorf("expected 🛠️ emoji, got %q", result)
	}
}

func TestProcessLogBuffer_PastTenseFileEmoji(t *testing.T) {
	buffer := NewProcessLogBuffer(ProcessLogBufferOptions{MaxChars: 1000})

	input := strings.Join([]string{
		"Analyzed package.json#L1-75",
		"",
		"Read src/index.ts",
		"",
		"Created new-file.ts",
		"",
		"Built project successfully",
	}, "\n")

	result := buffer.Append(input)

	if !strings.Contains(result, "📄 Analyzed package.json#L1-75") {
		t.Errorf("expected 📄 Analyzed, got %q", result)
	}
	if !strings.Contains(result, "📄 Read src/index.ts") {
		t.Errorf("expected 📄 Read, got %q", result)
	}
	if !strings.Contains(result, "📄 Created new-file.ts") {
		t.Errorf("expected 📄 Created, got %q", result)
	}
	if !strings.Contains(result, "📄 Built project successfully") {
		t.Errorf("expected 📄 Built, got %q", result)
	}
}

func TestProcessLogBuffer_PresentTenseActivityEmoji(t *testing.T) {
	buffer := NewProcessLogBuffer(ProcessLogBufferOptions{MaxChars: 1000})

	input := strings.Join([]string{
		"Fetching data from API",
		"",
		"Scanning directory for files",
		"",
		"Building project",
		"",
		"Creating test fixtures",
	}, "\n")

	result := buffer.Append(input)

	if !strings.Contains(result, "🔍 Fetching data from API") {
		t.Errorf("expected 🔍 Fetching, got %q", result)
	}
	if !strings.Contains(result, "🔍 Scanning directory for files") {
		t.Errorf("expected 🔍 Scanning, got %q", result)
	}
	if !strings.Contains(result, "🔍 Building project") {
		t.Errorf("expected 🔍 Building, got %q", result)
	}
	if !strings.Contains(result, "🔍 Creating test fixtures") {
		t.Errorf("expected 🔍 Creating, got %q", result)
	}
}

func TestProcessLogBuffer_DropsOldestFirst(t *testing.T) {
	buffer := NewProcessLogBuffer(ProcessLogBufferOptions{
		MaxChars:   45,
		MaxEntries: 10,
	})

	buffer.Append("Initiating Step A")
	buffer.Append("Initiating Step B")
	result := buffer.Append("Initiating Step C")

	if strings.Contains(result, "Step A") {
		t.Errorf("expected Step A to be dropped, got %q", result)
	}
	if !strings.Contains(result, "Step B") {
		t.Errorf("expected result to contain Step B, got %q", result)
	}
	if !strings.Contains(result, "Step C") {
		t.Errorf("expected result to contain Step C, got %q", result)
	}
}
