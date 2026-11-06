package monitor

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProgressSenderOptions configures the ProgressSender.
// Defaults: ThrottleMs = 3000, MaxLength = 4000, WrapInCodeBlock = true.
type ProgressSenderOptions struct {
	Send            func(content string) error
	ThrottleMs      int
	MaxLength       int
	WrapInCodeBlock *bool
}

// ProgressSender accumulates streamed chunks of text in a buffer and periodically emits them with throttle.
type ProgressSender struct {
	mu              sync.Mutex
	throttleMs      time.Duration
	maxLength       int
	wrapInCodeBlock bool
	buffer          string
	timer           *time.Timer
	sendContent     func(content string) error
}

// NewProgressSender creates a new ProgressSender with defaults matching the TypeScript implementation.
func NewProgressSender(options ProgressSenderOptions) *ProgressSender {
	throttleMs := options.ThrottleMs
	if throttleMs <= 0 {
		throttleMs = 3000
	}

	maxLength := options.MaxLength
	if maxLength <= 0 {
		maxLength = 4000
	}

	wrapInCodeBlock := true
	if options.WrapInCodeBlock != nil {
		wrapInCodeBlock = *options.WrapInCodeBlock
	}

	return &ProgressSender{
		sendContent:     options.Send,
		throttleMs:      time.Duration(throttleMs) * time.Millisecond,
		maxLength:       maxLength,
		wrapInCodeBlock: wrapInCodeBlock,
	}
}

// Append concatenates text to the buffer and starts the throttle timer if not currently active.
func (p *ProgressSender) Append(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer += text
	if p.timer == nil {
		p.timer = time.AfterFunc(p.throttleMs, func() {
			p.emit()
		})
	}
}

// ForceEmit immediately sends the accumulated buffer content and cancels any pending timer.
func (p *ProgressSender) ForceEmit() {
	p.emit()
}

// Dispose cancels any pending timer and clears the buffer without emitting anything.
func (p *ProgressSender) Dispose() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	p.buffer = ""
}

func (p *ProgressSender) emit() {
	p.mu.Lock()
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}

	if p.buffer == "" {
		p.mu.Unlock()
		return
	}

	payload := p.buffer
	p.buffer = ""
	sendFn := p.sendContent
	wrap := p.wrapInCodeBlock
	maxLen := p.maxLength
	p.mu.Unlock()

	if sendFn == nil {
		return
	}

	chunks := splitByLength(payload, maxLen)
	for _, chunk := range chunks {
		escaped := strings.ReplaceAll(chunk, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		content := chunk
		if wrap {
			content = fmt.Sprintf("<pre>%s</pre>", escaped)
		}
		_ = sendFn(content)
	}
}

func splitByLength(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var result []string
	cursor := 0
	for cursor < len(text) {
		end := cursor + maxLength
		if end > len(text) {
			end = len(text)
		}
		result = append(result, text[cursor:end])
		cursor += maxLength
	}
	return result
}
