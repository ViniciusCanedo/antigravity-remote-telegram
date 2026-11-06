package monitor

import (
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockSender struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (m *mockSender) Send(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, content)
	return m.err
}

func (m *mockSender) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockSender) GetCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]string, len(m.calls))
	copy(copied, m.calls)
	return copied
}

func TestProgressSender_ThrottleSendCalls(t *testing.T) {
	mock := &mockSender{}
	sender := NewProgressSender(ProgressSenderOptions{
		Send:       mock.Send,
		ThrottleMs: 50,
	})

	sender.Append("chunk 1\n")
	sender.Append("chunk 2\n")
	sender.Append("chunk 3\n")

	if mock.CallCount() != 0 {
		t.Fatalf("expected 0 calls before throttle elapses, got %d", mock.CallCount())
	}

	time.Sleep(80 * time.Millisecond)

	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 call after throttle, got %d", mock.CallCount())
	}

	calls := mock.GetCalls()
	if !strings.Contains(calls[0], "chunk 1\nchunk 2\nchunk 3\n") {
		t.Fatalf("expected concatenated chunks, got %q", calls[0])
	}
}

func TestProgressSender_ForceEmit(t *testing.T) {
	mock := &mockSender{}
	sender := NewProgressSender(ProgressSenderOptions{
		Send:       mock.Send,
		ThrottleMs: 500,
	})

	sender.Append("chunk 1\n")
	sender.ForceEmit()

	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 call immediately, got %d", mock.CallCount())
	}

	calls := mock.GetCalls()
	if !strings.Contains(calls[0], "chunk 1\n") {
		t.Fatalf("expected chunk 1 in call, got %q", calls[0])
	}
}

func TestProgressSender_SplitByMaxLength(t *testing.T) {
	mock := &mockSender{}
	sender := NewProgressSender(ProgressSenderOptions{
		Send:       mock.Send,
		ThrottleMs: 50,
		MaxLength:  50,
	})

	longString := "This is a very long string that will definitely exceed the fifty character limit for this test case."
	sender.Append(longString)

	time.Sleep(80 * time.Millisecond)

	expectedChunks := int(math.Ceil(float64(len(longString)) / 50.0))
	if mock.CallCount() != expectedChunks {
		t.Fatalf("expected %d calls, got %d", expectedChunks, mock.CallCount())
	}

	calls := mock.GetCalls()
	var sentBody strings.Builder
	for _, c := range calls {
		cleaned := strings.ReplaceAll(c, "<pre>", "")
		cleaned = strings.ReplaceAll(cleaned, "</pre>", "")
		sentBody.WriteString(cleaned)
	}

	if !strings.Contains(sentBody.String(), longString) {
		t.Fatalf("expected sent body to contain full string, got %q", sentBody.String())
	}
}

func TestProgressSender_WrapInCodeBlockOptions(t *testing.T) {
	t.Run("should use send function without code block wrapping", func(t *testing.T) {
		mock := &mockSender{}
		wrapFalse := false
		sender := NewProgressSender(ProgressSenderOptions{
			Send:            mock.Send,
			ThrottleMs:      50,
			WrapInCodeBlock: &wrapFalse,
		})

		sender.Append("line 1\n")
		time.Sleep(80 * time.Millisecond)

		if mock.CallCount() != 1 {
			t.Fatalf("expected 1 call, got %d", mock.CallCount())
		}
		calls := mock.GetCalls()
		if calls[0] != "line 1\n" {
			t.Fatalf("expected 'line 1\\n', got %q", calls[0])
		}
	})

	t.Run("wraps in <pre> tags by default", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("code output")
		time.Sleep(80 * time.Millisecond)

		if mock.CallCount() != 1 {
			t.Fatalf("expected 1 call, got %d", mock.CallCount())
		}
		calls := mock.GetCalls()
		if calls[0] != "<pre>code output</pre>" {
			t.Fatalf("expected '<pre>code output</pre>', got %q", calls[0])
		}
	})
}

func TestProgressSender_Dispose(t *testing.T) {
	t.Run("clears pending timer so no emit happens", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("data")
		sender.Dispose()

		time.Sleep(80 * time.Millisecond)
		if mock.CallCount() != 0 {
			t.Fatalf("expected 0 calls after dispose, got %d", mock.CallCount())
		}
	})

	t.Run("clears the buffer", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("data")
		sender.Dispose()

		sender.ForceEmit()
		if mock.CallCount() != 0 {
			t.Fatalf("expected 0 calls after forceEmit on cleared buffer, got %d", mock.CallCount())
		}
	})

	t.Run("is safe to call multiple times", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("data")
		sender.Dispose()
		sender.Dispose()

		if mock.CallCount() != 0 {
			t.Fatalf("expected 0 calls, got %d", mock.CallCount())
		}
	})

	t.Run("can append new data after dispose", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("old")
		sender.Dispose()

		sender.Append("new")
		time.Sleep(80 * time.Millisecond)

		if mock.CallCount() != 1 {
			t.Fatalf("expected 1 call, got %d", mock.CallCount())
		}
		calls := mock.GetCalls()
		if calls[0] != "<pre>new</pre>" {
			t.Fatalf("expected '<pre>new</pre>', got %q", calls[0])
		}
	})
}

func TestProgressSender_EmitEdgeCases(t *testing.T) {
	t.Run("does not send when buffer is empty", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.ForceEmit()
		if mock.CallCount() != 0 {
			t.Fatalf("expected 0 calls for empty buffer, got %d", mock.CallCount())
		}
	})

	t.Run("swallows send errors silently", func(t *testing.T) {
		mock := &mockSender{err: errors.New("network")}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("data")
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("unexpected panic on send error: %v", r)
			}
		}()
		sender.ForceEmit()
	})

	t.Run("only creates one timer for multiple appends", func(t *testing.T) {
		mock := &mockSender{}
		sender := NewProgressSender(ProgressSenderOptions{
			Send:       mock.Send,
			ThrottleMs: 50,
		})

		sender.Append("a")
		sender.Append("b")
		sender.Append("c")

		time.Sleep(80 * time.Millisecond)

		if mock.CallCount() != 1 {
			t.Fatalf("expected 1 call for 3 appends, got %d", mock.CallCount())
		}
		calls := mock.GetCalls()
		if calls[0] != "<pre>abc</pre>" {
			t.Fatalf("expected '<pre>abc</pre>', got %q", calls[0])
		}
	})
}

func TestProgressSender_DefaultOptions(t *testing.T) {
	mock := &mockSender{}
	sender := NewProgressSender(ProgressSenderOptions{
		Send: mock.Send,
	})

	if sender.throttleMs != 3000*time.Millisecond {
		t.Errorf("expected throttleMs = 3000ms, got %v", sender.throttleMs)
	}
	if sender.maxLength != 4000 {
		t.Errorf("expected maxLength = 4000, got %d", sender.maxLength)
	}
	if !sender.wrapInCodeBlock {
		t.Errorf("expected wrapInCodeBlock = true, got %v", sender.wrapInCodeBlock)
	}

	longText := strings.Repeat("x", 8000)
	sender.Append(longText)
	sender.ForceEmit()

	if mock.CallCount() != 2 {
		t.Fatalf("expected 2 chunks for 8000 chars with default 4000 maxLength, got %d", mock.CallCount())
	}

	mock2 := &mockSender{}
	sender2 := NewProgressSender(ProgressSenderOptions{
		Send: mock2.Send,
	})
	sender2.Append("test")
	sender2.ForceEmit()

	calls2 := mock2.GetCalls()
	if len(calls2) != 1 || calls2[0] != "<pre>test</pre>" {
		t.Fatalf("expected '<pre>test</pre>', got %v", calls2)
	}
}
