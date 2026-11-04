package cdp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClient_ClearPendingCalls_RejectsAllWithDisconnectError(t *testing.T) {
	zero := 0
	client := NewClient(ClientOptions{
		CallTimeout:          5 * time.Second,
		MaxReconnectAttempts: &zero,
	})

	resCh := make(chan rpcResponse, 1)
	timer := time.NewTimer(10 * time.Second)

	client.mu.Lock()
	client.pendingCalls[999] = &pendingCall{
		resultCh: resCh,
		timer:    timer,
	}
	client.mu.Unlock()

	client.clearPendingCalls(errors.New("WebSocket切断"))

	select {
	case resp := <-resCh:
		if resp.Err == nil || resp.Err.Error() != "WebSocket切断" {
			t.Fatalf("expected error 'WebSocket切断', got %v", resp.Err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for pending call reject")
	}
}

func TestClient_ClearPendingCalls_EmptiesMap(t *testing.T) {
	zero := 0
	client := NewClient(ClientOptions{
		CallTimeout:          5 * time.Second,
		MaxReconnectAttempts: &zero,
	})

	client.mu.Lock()
	client.pendingCalls[1] = &pendingCall{
		resultCh: make(chan rpcResponse, 1),
		timer:    time.NewTimer(10 * time.Second),
	}
	client.pendingCalls[2] = &pendingCall{
		resultCh: make(chan rpcResponse, 1),
		timer:    time.NewTimer(10 * time.Second),
	}
	countBefore := len(client.pendingCalls)
	client.mu.Unlock()

	if countBefore != 2 {
		t.Fatalf("expected 2 pending calls, got %d", countBefore)
	}

	client.clearPendingCalls(errors.New("切断"))

	client.mu.RLock()
	countAfter := len(client.pendingCalls)
	client.mu.RUnlock()

	if countAfter != 0 {
		t.Fatalf("expected 0 pending calls, got %d", countAfter)
	}
}

func TestClient_TryReconnect_UsesCurrentWorkspacePath(t *testing.T) {
	one := 1
	var mu sync.Mutex
	var scannedURLs []string

	client := NewClient(ClientOptions{
		CallTimeout:          1 * time.Second,
		MaxReconnectAttempts: &one,
		ReconnectDelay:       10 * time.Millisecond,
		PortsToScan:          []int{9222},
		HTTPGetJSONFunc: func(u string) ([]TargetInfo, error) {
			mu.Lock()
			scannedURLs = append(scannedURLs, u)
			mu.Unlock()
			return []TargetInfo{
				{
					ID:                   "page-1",
					Type:                 "page",
					Title:                "my-workspace — Antigravity",
					URL:                  "file:///workbench",
					WebSocketDebuggerURL: "ws://dummy-target",
				},
			}, nil
		},
		DialWebSocketFunc: func(ctx context.Context, u string) (WebSocketConn, error) {
			return newMockWSConn(), nil
		},
	})
	defer func() { _ = client.Disconnect() }()

	client.mu.Lock()
	client.currentWorkspacePath = "/tmp/my-workspace"
	client.mu.Unlock()

	reconnectedCh := make(chan struct{})
	client.On("reconnected", func(any) {
		close(reconnectedCh)
	})

	go client.tryReconnect()

	select {
	case <-reconnectedCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for reconnected event")
	}

	if client.GetCurrentWorkspaceName() != "my-workspace" {
		t.Fatalf("expected workspace name 'my-workspace', got %q", client.GetCurrentWorkspaceName())
	}
}

func TestClient_AutoReconnect_DisabledWhenMaxAttemptsZero(t *testing.T) {
	zero := 0
	client := NewClient(ClientOptions{
		CallTimeout:          1 * time.Second,
		MaxReconnectAttempts: &zero,
	})
	defer func() { _ = client.Disconnect() }()

	disconnectedCh := make(chan struct{})
	client.On("disconnected", func(any) {
		close(disconnectedCh)
	})

	client.mu.Lock()
	client.isConnectedFlag = true
	client.ws = newMockWSConn()
	client.mu.Unlock()

	// Simulate disconnect
	client.handleDisconnect(client.ws)

	select {
	case <-disconnectedCh:
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for disconnected event")
	}

	time.Sleep(100 * time.Millisecond)

	client.mu.RLock()
	reconnecting := client.isReconnecting
	reconnectCount := client.reconnectAttemptCnt
	client.mu.RUnlock()

	if reconnecting {
		t.Fatalf("expected isReconnecting to be false")
	}
	if reconnectCount != 0 {
		t.Fatalf("expected reconnectAttemptCnt to be 0, got %d", reconnectCount)
	}
}

func TestClient_TryReconnect_EmitsReconnectFailedOnExhaustion(t *testing.T) {
	two := 2
	client := NewClient(ClientOptions{
		CallTimeout:          1 * time.Second,
		MaxReconnectAttempts: &two,
		ReconnectDelay:       10 * time.Millisecond,
		PortsToScan:          []int{9222},
		HTTPGetJSONFunc: func(u string) ([]TargetInfo, error) {
			return nil, errors.New("Target not found")
		},
	})
	defer func() { _ = client.Disconnect() }()

	failedCh := make(chan error, 1)
	client.On("reconnectFailed", func(payload any) {
		if err, ok := payload.(error); ok {
			failedCh <- err
		}
	})

	go client.tryReconnect()

	select {
	case err := <-failedCh:
		if err == nil {
			t.Fatalf("expected non-nil error")
		}
		if !strings.Contains(err.Error(), "CDP reconnection failed 2 times") {
			t.Fatalf("expected error message to contain attempt count, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for reconnectFailed event")
	}
}

func TestClient_TryReconnect_DoesNotExceedMaxAttempts(t *testing.T) {
	two := 2
	var mu sync.Mutex
	connectCallCount := 0

	client := NewClient(ClientOptions{
		CallTimeout:          1 * time.Second,
		MaxReconnectAttempts: &two,
		ReconnectDelay:       10 * time.Millisecond,
		PortsToScan:          []int{9222},
		HTTPGetJSONFunc: func(u string) ([]TargetInfo, error) {
			mu.Lock()
			connectCallCount++
			mu.Unlock()
			return nil, errors.New("Connection failed")
		},
	})
	defer func() { _ = client.Disconnect() }()

	failedCh := make(chan struct{})
	client.On("reconnectFailed", func(any) {
		close(failedCh)
	})

	go client.tryReconnect()

	select {
	case <-failedCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for reconnectFailed event")
	}

	mu.Lock()
	count := connectCallCount
	mu.Unlock()

	if count > 2 {
		t.Fatalf("expected at most 2 reconnect attempts, got %d", count)
	}
}

func TestClient_Options_DefaultsAndOverrides(t *testing.T) {
	// Default client
	cDefault := NewClient(ClientOptions{})
	if cDefault.maxReconnectAttempts != 3 {
		t.Fatalf("expected default maxReconnectAttempts=3, got %d", cDefault.maxReconnectAttempts)
	}
	if cDefault.reconnectDelay != 2000*time.Millisecond {
		t.Fatalf("expected default reconnectDelay=2000ms, got %v", cDefault.reconnectDelay)
	}

	// Overridden client
	five := 5
	cCustom := NewClient(ClientOptions{
		MaxReconnectAttempts: &five,
		ReconnectDelay:       3000 * time.Millisecond,
	})
	if cCustom.maxReconnectAttempts != 5 {
		t.Fatalf("expected maxReconnectAttempts=5, got %d", cCustom.maxReconnectAttempts)
	}
	if cCustom.reconnectDelay != 3000*time.Millisecond {
		t.Fatalf("expected reconnectDelay=3000ms, got %v", cCustom.reconnectDelay)
	}

	// Zero reconnect attempts (explicit 0)
	zero := 0
	cZero := NewClient(ClientOptions{
		MaxReconnectAttempts: &zero,
	})
	if cZero.maxReconnectAttempts != 0 {
		t.Fatalf("expected maxReconnectAttempts=0, got %d", cZero.maxReconnectAttempts)
	}
}
