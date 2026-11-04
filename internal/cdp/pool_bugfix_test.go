package cdp

import (
	"errors"
	"testing"
)

func TestConnectionPool_Bugfix_DisconnectsStaleOnRevalidationError(t *testing.T) {
	mockCdp := newMockPoolClient(1)
	freshCdp := newMockPoolClient(2)

	firstCall := true
	mockCdp.onConnectFunc = func(path string) (bool, error) {
		if firstCall {
			firstCall = false
			return true, nil
		}
		// Re-validation fails
		return false, errors.New("tab closed")
	}

	callCount := 0
	factory := func(opts ClientOptions) PoolClient {
		callCount++
		if callCount == 1 {
			return mockCdp
		}
		return freshCdp
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	cdp1, err := pool.GetOrConnect("/path/to/Project")
	if err != nil {
		t.Fatalf("first GetOrConnect failed: %v", err)
	}
	if cdp1 != mockCdp {
		t.Fatalf("expected first connection to be mockCdp")
	}

	cdp2, err := pool.GetOrConnect("/path/to/Project")
	if err != nil {
		t.Fatalf("second GetOrConnect failed: %v", err)
	}

	mockCdp.mu.Lock()
	disconnectCalls := mockCdp.disconnectCalls
	mockCdp.mu.Unlock()

	if disconnectCalls == 0 {
		t.Fatalf("expected stale mockCdp to have Disconnect called")
	}

	if cdp2 != freshCdp {
		t.Fatalf("expected second connection to be freshCdp, got %v", cdp2)
	}
}

func TestConnectionPool_Bugfix_CleansUpDisconnectedEntryWithoutCallingDisconnect(t *testing.T) {
	mockCdp := newMockPoolClient(1)
	freshCdp := newMockPoolClient(2)

	callCount := 0
	factory := func(opts ClientOptions) PoolClient {
		callCount++
		if callCount == 1 {
			return mockCdp
		}
		return freshCdp
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	// Initial connection
	cdp1, err := pool.GetOrConnect("/path/to/Project")
	if err != nil {
		t.Fatalf("first GetOrConnect failed: %v", err)
	}
	if cdp1 != mockCdp {
		t.Fatalf("expected first connection to be mockCdp")
	}

	// Mark mockCdp as disconnected
	mockCdp.mu.Lock()
	mockCdp.connected = false
	mockCdp.mu.Unlock()

	// Second call
	cdp2, err := pool.GetOrConnect("/path/to/Project")
	if err != nil {
		t.Fatalf("second GetOrConnect failed: %v", err)
	}

	if cdp2 != freshCdp {
		t.Fatalf("expected fresh connection to replace disconnected entry")
	}

	mockCdp.mu.Lock()
	disconnectCalls := mockCdp.disconnectCalls
	mockCdp.mu.Unlock()

	if disconnectCalls != 0 {
		t.Fatalf("expected Disconnect NOT to be called on already-disconnected entry, got %d calls", disconnectCalls)
	}
}

func TestConnectionPool_Bugfix_ReconnectFailedCleansUpWorkspaceAndDetectors(t *testing.T) {
	mockCdp := newMockPoolClient(1)

	factory := func(opts ClientOptions) PoolClient {
		return mockCdp
	}

	pool := NewConnectionPool(ClientOptions{}, factory)

	_, err := pool.GetOrConnect("/path/to/Project")
	if err != nil {
		t.Fatalf("GetOrConnect failed: %v", err)
	}

	detector := &mockDetector{active: true}
	pool.RegisterApprovalDetector("Project", detector)

	// Verify listener registered for reconnectFailed
	mockCdp.mu.Lock()
	handlers := mockCdp.listeners["reconnectFailed"]
	mockCdp.mu.Unlock()

	if len(handlers) == 0 {
		t.Fatalf("expected listener for reconnectFailed to be registered")
	}

	// Trigger reconnectFailed
	mockCdp.emit("reconnectFailed", errors.New("exhausted"))

	mockCdp.mu.Lock()
	disconnectCalls := mockCdp.disconnectCalls
	mockCdp.mu.Unlock()

	if disconnectCalls == 0 {
		t.Fatalf("expected Disconnect to be called on mockCdp")
	}

	if detector.IsActive() {
		t.Fatalf("expected detector to be stopped on reconnectFailed")
	}

	if pool.GetConnected("Project") != nil {
		t.Fatalf("expected Project to be removed from pool")
	}

	if pool.GetApprovalDetector("Project") != nil {
		t.Fatalf("expected approval detector to be removed from pool")
	}
}

func TestConnectionPool_Bugfix_RegistersLifecycleEventHandlers(t *testing.T) {
	mockCdp := newMockPoolClient(1)

	factory := func(opts ClientOptions) PoolClient {
		return mockCdp
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	_, err := pool.GetOrConnect("/path/to/Project")
	if err != nil {
		t.Fatalf("GetOrConnect failed: %v", err)
	}

	mockCdp.mu.Lock()
	disconnectedLen := len(mockCdp.listeners["disconnected"])
	reconnectedLen := len(mockCdp.listeners["reconnected"])
	reconnectFailedLen := len(mockCdp.listeners["reconnectFailed"])
	mockCdp.mu.Unlock()

	if disconnectedLen == 0 {
		t.Fatalf("expected 'disconnected' listener to be registered")
	}
	if reconnectedLen == 0 {
		t.Fatalf("expected 'reconnected' listener to be registered")
	}
	if reconnectFailedLen == 0 {
		t.Fatalf("expected 'reconnectFailed' listener to be registered")
	}
}
