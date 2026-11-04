package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: First poll primes existing DOM state without firing callback
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_FirstPollPrimesWithoutCallback(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(UserMessageInfo{Text: "Existing message"}), nil
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	detector.Start()
	ctx := context.Background()

	// First poll -- priming, should NOT fire callback
	detector.PollOnce(ctx)
	if callsCount != 0 {
		t.Fatalf("expected 0 calls during priming, got %d", callsCount)
	}

	// Second poll -- same message, still no callback (duplicate)
	detector.PollOnce(ctx)
	if callsCount != 0 {
		t.Fatalf("expected 0 calls for duplicate message, got %d", callsCount)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: Detects a new user message after priming
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_DetectsNewMessageAfterPriming(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(UserMessageInfo{Text: "Old message"}), nil
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "Hello from PC"}), nil
	}

	var receivedText string
	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			receivedText = info.Text
			callsCount++
		},
	})

	detector.Start()
	if !detector.IsActive() {
		t.Fatalf("expected detector to be active")
	}

	ctx := context.Background()
	// First poll -- priming
	detector.PollOnce(ctx)
	if callsCount != 0 {
		t.Fatalf("expected 0 calls during priming, got %d", callsCount)
	}

	// Second poll -- new message detected
	detector.PollOnce(ctx)
	if callsCount != 1 {
		t.Fatalf("expected 1 call after new message, got %d", callsCount)
	}
	if receivedText != "Hello from PC" {
		t.Errorf("expected text %q, got %q", "Hello from PC", receivedText)
	}

	detector.Stop()
	if detector.IsActive() {
		t.Fatalf("expected detector to be inactive after stop")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: Primes with empty DOM and detects first real message
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_PrimesWithEmptyDOMAndDetectsFirstMessage(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // empty DOM
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "First message"}), nil
	}

	var receivedText string
	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			receivedText = info.Text
			callsCount++
		},
	})

	detector.Start()
	ctx := context.Background()

	// First poll -- priming with empty DOM
	detector.PollOnce(ctx)
	if callsCount != 0 {
		t.Fatalf("expected 0 calls during priming with empty DOM, got %d", callsCount)
	}

	// Second poll -- first real message
	detector.PollOnce(ctx)
	if callsCount != 1 {
		t.Fatalf("expected 1 call on first real message, got %d", callsCount)
	}
	if receivedText != "First message" {
		t.Errorf("expected text %q, got %q", "First message", receivedText)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: Does not call onUserMessage for duplicate messages
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_DeduplicatesSameMessage(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // priming
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "Same message"}), nil
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	detector.Start()
	ctx := context.Background()

	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // first real poll -> 1 call
	detector.PollOnce(ctx) // second real poll -> duplicate, 1 call

	if callsCount != 1 {
		t.Fatalf("expected exactly 1 call, got %d", callsCount)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: Skips messages matching echo hashes
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_SkipsEchoHashedMessages(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // priming
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "Echoed message"}), nil
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	detector.AddEchoHash("Echoed message")
	detector.Start()
	ctx := context.Background()

	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // echo message, should be skipped

	if callsCount != 0 {
		t.Fatalf("expected 0 calls for echoed message, got %d", callsCount)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: Handles CDP errors gracefully
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_HandlesCDPErrorsGracefully(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return nil, errors.New("CDP timeout")
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	detector.Start()
	ctx := context.Background()
	detector.PollOnce(ctx)

	if !detector.IsActive() {
		t.Fatalf("expected detector to remain active on CDP error")
	}
	if callsCount != 0 {
		t.Fatalf("expected 0 calls, got %d", callsCount)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 7: Handles null result from CDP
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_HandlesNullCDPResult(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	detector.Start()
	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 0 {
		t.Fatalf("expected 0 calls on null result, got %d", callsCount)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 8: Detects new message after different message
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_DetectsSequentialDifferentMessages(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // priming
		}
		if callCount == 2 {
			return makeEvaluateResponse(UserMessageInfo{Text: "First message"}), nil
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "Second message"}), nil
	}

	var messages []string
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			messages = append(messages, info.Text)
		},
	})

	detector.Start()
	ctx := context.Background()

	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // first message
	detector.PollOnce(ctx) // second message

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0] != "First message" || messages[1] != "Second message" {
		t.Errorf("unexpected messages: %v", messages)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 9: Start() is idempotent
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_StartIsIdempotent(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
	})

	detector.Start()
	detector.Start() // idempotent second call

	if !detector.IsActive() {
		t.Fatalf("expected detector to be active")
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 10: seenHashes prevents re-detection of old messages after different appears
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_SeenHashesPreventsOldMessageRedetection(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // priming
		}
		if callCount == 2 {
			return makeEvaluateResponse(UserMessageInfo{Text: "Message A"}), nil
		}
		if callCount == 3 {
			return makeEvaluateResponse(UserMessageInfo{Text: "Message B"}), nil
		}
		// Poll 4: DOM reverts to Message A
		return makeEvaluateResponse(UserMessageInfo{Text: "Message A"}), nil
	}

	var messages []string
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			messages = append(messages, info.Text)
		},
	})

	detector.Start()
	ctx := context.Background()

	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // Message A
	detector.PollOnce(ctx) // Message B
	detector.PollOnce(ctx) // Message A again -> should be skipped by seenHashes

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d (%v)", len(messages), messages)
	}
	if messages[0] != "Message A" || messages[1] != "Message B" {
		t.Errorf("unexpected messages sequence: %v", messages)
	}

	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 11: seenHashes are cleared on restart
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_SeenHashesClearedOnRestart(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // priming (session 1)
		}
		if callCount == 2 {
			return makeEvaluateResponse(UserMessageInfo{Text: "Restart message"}), nil
		}
		if callCount == 3 {
			return makeEvaluateResponse(nil), nil // priming (session 2)
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "Restart message"}), nil
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	ctx := context.Background()

	// First session
	detector.Start()
	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // detect
	if callsCount != 1 {
		t.Fatalf("expected 1 call in session 1, got %d", callsCount)
	}
	detector.Stop()

	// Second session -- same message should be detected again after restart
	detector.Start()
	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // detect
	if callsCount != 2 {
		t.Fatalf("expected 2 calls total after restart, got %d", callsCount)
	}
	detector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 12: Echo hash expires after TTL
// ─────────────────────────────────────────────────────────────────────────────
func TestUserMessageDetector_EchoHashExpiresAfterTTL(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(nil), nil // priming
		}
		return makeEvaluateResponse(UserMessageInfo{Text: "Expiring echo"}), nil
	}

	callsCount := 0
	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService: mockCDP,
		EchoTTL:    20 * time.Millisecond,
		OnUserMessage: func(info UserMessageInfo) {
			callsCount++
		},
	})

	detector.AddEchoHash("Expiring echo")
	// Wait for TTL to expire
	time.Sleep(40 * time.Millisecond)

	detector.Start()
	ctx := context.Background()

	detector.PollOnce(ctx) // priming
	detector.PollOnce(ctx) // message after echo expired should be detected!

	if callsCount != 1 {
		t.Fatalf("expected 1 call after echo expired, got %d", callsCount)
	}

	detector.Stop()
}
