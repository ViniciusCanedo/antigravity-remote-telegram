package monitor

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const (
	DefaultMaxChars       = 3500
	DefaultMaxEntries     = 120
	DefaultMaxEntryLength = 260
)

var (
	reWhitespace     = regexp.MustCompile(`\s+`)
	reDoubleNewline  = regexp.MustCompile(`\n{2,}`)
	reThought        = regexp.MustCompile(`(?i)^(?:thought for\b|thinking\b)`)
	reInitiating     = regexp.MustCompile(`(?i)^(?:initiating\b|starting\b)`)
	reMcpFormat      = regexp.MustCompile(`(?i)^[a-z0-9._-]+\s*/\s*[a-z0-9._-]+$`)
	rePastTense      = regexp.MustCompile(`(?i)^(?:analy[sz]ed|read|wrote|created|updated|deleted|built|compiled|installed|resolved|downloaded|connected|fetched)\b`)
	rePresentTense   = regexp.MustCompile(`(?i)^(?:analy[sz]ing|reading|writing|running|searching|fetching|checking|scanning|creating|updating|deleting|building|compiling|deploying|parsing|resolving|downloading|uploading|connecting|installing|executing|testing|debugging|processing|loading)\b`)
	reTitlePrefix    = regexp.MustCompile(`^title:\s`)
	reUrlInLine      = regexp.MustCompile(`\surl:\s`)
	reCodeIdentifier = regexp.MustCompile(`(?i)^(?:json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log)$`)
)

// ProcessLogBufferOptions defines the configuration options for ProcessLogBuffer.
type ProcessLogBufferOptions struct {
	MaxChars       int
	MaxEntries     int
	MaxEntryLength int
}

// ProcessLogBuffer maintains a deduplicated, circular buffer of process activity logs
// formatted with descriptive emoji prefixes.
type ProcessLogBuffer struct {
	mu             sync.Mutex
	maxChars       int
	maxEntries     int
	maxEntryLength int
	entries        []string
	seen           map[string]struct{}
}

// NewProcessLogBuffer creates a new ProcessLogBuffer applying defaults for non-positive values.
func NewProcessLogBuffer(options ProcessLogBufferOptions) *ProcessLogBuffer {
	maxChars := options.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	maxEntries := options.MaxEntries
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	maxEntryLength := options.MaxEntryLength
	if maxEntryLength <= 0 {
		maxEntryLength = DefaultMaxEntryLength
	}

	return &ProcessLogBuffer{
		maxChars:       maxChars,
		maxEntries:     maxEntries,
		maxEntryLength: maxEntryLength,
		seen:           make(map[string]struct{}),
	}
}

func collapseWhitespace(text string) string {
	res := strings.ReplaceAll(text, "\r", "")
	res = reWhitespace.ReplaceAllString(res, " ")
	return strings.TrimSpace(res)
}

func parseBlocks(raw string) []string {
	normalized := strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
	if normalized == "" {
		return nil
	}

	chunks := reDoubleNewline.Split(normalized, -1)
	var blocks []string
	for _, chunk := range chunks {
		c := collapseWhitespace(chunk)
		if len(c) > 0 {
			blocks = append(blocks, c)
		}
	}

	if len(blocks) > 0 {
		return blocks
	}

	lines := strings.Split(normalized, "\n")
	for _, line := range lines {
		c := collapseWhitespace(line)
		if len(c) > 0 {
			blocks = append(blocks, c)
		}
	}
	return blocks
}

func pickEmoji(entry string) string {
	lower := strings.ToLower(entry)
	if reThought.MatchString(lower) {
		return "🧠"
	}
	if reInitiating.MatchString(lower) {
		return "🚀"
	}
	if reMcpFormat.MatchString(entry) {
		return "🛠️"
	}
	if rePastTense.MatchString(entry) {
		return "📄"
	}
	if rePresentTense.MatchString(entry) {
		return "🔍"
	}
	if reTitlePrefix.MatchString(lower) && reUrlInLine.MatchString(lower) {
		return "🔎"
	}
	if reCodeIdentifier.MatchString(entry) {
		return "📦"
	}
	return "•"
}

func toDisplayEntry(rawEntry string, maxEntryLength int) string {
	trimmed := collapseWhitespace(rawEntry)
	if trimmed == "" {
		return ""
	}
	clipped := trimmed
	if len(trimmed) > maxEntryLength {
		cut := maxEntryLength - 3
		if cut < 0 {
			cut = 0
		}
		clipped = fmt.Sprintf("%s...", trimmed[:cut])
	}
	return fmt.Sprintf("%s %s", pickEmoji(clipped), clipped)
}

// Append parses blocks from raw, formats them with emoji prefixes, deduplicates, trims to capacity, and returns the current snapshot.
func (b *ProcessLogBuffer) Append(raw string) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	blocks := parseBlocks(raw)
	for _, block := range blocks {
		display := toDisplayEntry(block, b.maxEntryLength)
		if display == "" {
			continue
		}
		key := strings.ToLower(display)
		if _, exists := b.seen[key]; exists {
			continue
		}
		b.entries = append(b.entries, display)
		b.seen[key] = struct{}{}
	}

	b.trim()
	return strings.Join(b.entries, "\n")
}

// Snapshot returns all current entries joined by "\n".
func (b *ProcessLogBuffer) Snapshot() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.entries, "\n")
}

func (b *ProcessLogBuffer) trim() {
	for len(b.entries) > b.maxEntries {
		b.dropOldest()
	}

	totalLen := 0
	for _, e := range b.entries {
		totalLen += len(e)
	}
	if len(b.entries) > 1 {
		totalLen += len(b.entries) - 1
	}

	for len(b.entries) > 1 && totalLen > b.maxChars {
		removed := b.entries[0]
		b.dropOldest()
		totalLen -= (len(removed) + 1)
	}

	if len(b.entries) == 1 && len(b.entries[0]) > b.maxChars {
		only := b.entries[0]
		cut := b.maxChars - 3
		if cut < 0 {
			cut = 0
		}
		b.entries[0] = fmt.Sprintf("%s...", only[:cut])
		b.seen = map[string]struct{}{
			strings.ToLower(b.entries[0]): {},
		}
	}
}

func (b *ProcessLogBuffer) dropOldest() {
	if len(b.entries) == 0 {
		return
	}
	removed := b.entries[0]
	b.entries = b.entries[1:]
	delete(b.seen, strings.ToLower(removed))
}
