package cdp

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type mockPoolClient struct {
	mu              sync.Mutex
	connected       bool
	connectCalls    int
	disconnectCalls int
	listeners       map[string][]EventHandler
	id              int
	onConnectFunc   func(path string) (bool, error)
}

func newMockPoolClient(id int) *mockPoolClient {
	return &mockPoolClient{
		connected: true,
		listeners: make(map[string][]EventHandler),
		id:        id,
	}
}

func (m *mockPoolClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *mockPoolClient) DiscoverAndConnectForWorkspace(workspacePath string) (bool, error) {
	m.mu.Lock()
	m.connectCalls++
	onConnect := m.onConnectFunc
	m.mu.Unlock()

	if onConnect != nil {
		return onConnect(workspacePath)
	}
	return true, nil
}

func (m *mockPoolClient) Disconnect() error {
	m.mu.Lock()
	m.disconnectCalls++
	m.connected = false
	m.mu.Unlock()
	return nil
}

func (m *mockPoolClient) On(event string, handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listeners == nil {
		m.listeners = make(map[string][]EventHandler)
	}
	m.listeners[event] = append(m.listeners[event], handler)
}

func (m *mockPoolClient) emit(event string, payload any) {
	m.mu.Lock()
	handlers := make([]EventHandler, len(m.listeners[event]))
	copy(handlers, m.listeners[event])
	m.mu.Unlock()

	for _, h := range handlers {
		h(payload)
	}
}

func (m *mockPoolClient) Call(method string, params any) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockPoolClient) GetContexts() []Context {
	return nil
}

func (m *mockPoolClient) GetPrimaryContextID() *int {
	return nil
}

func (m *mockPoolClient) WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool {
	return true
}

type mockDetector struct {
	mu         sync.Mutex
	active     bool
	stopCalls  int
	startCalls int
}

func (d *mockDetector) IsActive() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.active
}

func (d *mockDetector) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.active = true
	d.startCalls++
}

func (d *mockDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.active = false
	d.stopCalls++
}

func TestConnectionPool_ExtractProjectName(t *testing.T) {
	pool := NewConnectionPool(ClientOptions{})

	cases := []struct {
		input    string
		expected string
	}{
		{"/home/user/Code/MyProject", "MyProject"},
		{"/home/user/Code/MyProject/", "MyProject"},
		{"MyProject", "MyProject"},
		{`D:\Code\MyProject`, "MyProject"},
		{`D:\categorizer`, "categorizer"},
		{`C:\Users\test/Code/MyProject`, "MyProject"},
	}

	for _, tc := range cases {
		actual := pool.ExtractProjectName(tc.input)
		if actual != tc.expected {
			t.Errorf("ExtractProjectName(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestConnectionPool_GetOrConnect_CreatesAndConnects(t *testing.T) {
	var created *mockPoolClient
	factory := func(opts ClientOptions) PoolClient {
		created = newMockPoolClient(1)
		return created
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	cdp, err := pool.GetOrConnect("/path/to/ProjectA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cdp != created {
		t.Fatalf("expected returned cdp to match created client")
	}

	created.mu.Lock()
	calls := created.connectCalls
	created.mu.Unlock()

	if calls != 1 {
		t.Fatalf("expected 1 connect call, got %d", calls)
	}

	active := pool.GetActiveWorkspaceNames()
	found := false
	for _, name := range active {
		if name == "ProjectA" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ProjectA to be in active workspace names, got %v", active)
	}
}

func TestConnectionPool_GetOrConnect_ReusesAndRevalidatesCachedInstance(t *testing.T) {
	var created *mockPoolClient
	factory := func(opts ClientOptions) PoolClient {
		created = newMockPoolClient(1)
		return created
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	cdp1, err := pool.GetOrConnect("/path/to/ProjectA")
	if err != nil {
		t.Fatalf("first GetOrConnect failed: %v", err)
	}

	cdp2, err := pool.GetOrConnect("/path/to/ProjectA")
	if err != nil {
		t.Fatalf("second GetOrConnect failed: %v", err)
	}

	if cdp1 != cdp2 {
		t.Fatalf("expected cached instance to be returned")
	}

	created.mu.Lock()
	calls := created.connectCalls
	created.mu.Unlock()

	if calls != 2 {
		t.Fatalf("expected 2 connect calls (initial + revalidate), got %d", calls)
	}
}

func TestConnectionPool_GetOrConnect_SeparateInstancesPerWorkspace(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	factory := func(opts ClientOptions) PoolClient {
		mu.Lock()
		callCount++
		id := callCount
		mu.Unlock()
		return newMockPoolClient(id)
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	cdpA, err := pool.GetOrConnect("/path/to/ProjectA")
	if err != nil {
		t.Fatalf("GetOrConnect ProjectA failed: %v", err)
	}

	cdpB, err := pool.GetOrConnect("/path/to/ProjectB")
	if err != nil {
		t.Fatalf("GetOrConnect ProjectB failed: %v", err)
	}

	if cdpA == cdpB {
		t.Fatalf("expected separate instances for different workspaces")
	}

	active := pool.GetActiveWorkspaceNames()
	hasA, hasB := false, false
	for _, name := range active {
		if name == "ProjectA" {
			hasA = true
		}
		if name == "ProjectB" {
			hasB = true
		}
	}

	if !hasA || !hasB {
		t.Fatalf("expected active to contain both ProjectA and ProjectB, got %v", active)
	}
}

func TestConnectionPool_GetOrConnect_PreventsConcurrentConnections(t *testing.T) {
	var factoryCalls int
	var mu sync.Mutex

	connectGate := make(chan struct{})

	factory := func(opts ClientOptions) PoolClient {
		mu.Lock()
		factoryCalls++
		mu.Unlock()

		client := newMockPoolClient(1)
		client.onConnectFunc = func(path string) (bool, error) {
			<-connectGate
			return true, nil
		}
		return client
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	var wg sync.WaitGroup
	var cdp1, cdp2 PoolClient
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		cdp1, err1 = pool.GetOrConnect("/path/to/ProjectX")
	}()

	go func() {
		defer wg.Done()
		cdp2, err2 = pool.GetOrConnect("/path/to/ProjectX")
	}()

	// Brief pause to ensure both goroutines have hit the gate
	time.Sleep(50 * time.Millisecond)

	close(connectGate)
	wg.Wait()

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected error: err1=%v, err2=%v", err1, err2)
	}

	if cdp1 != cdp2 {
		t.Fatalf("expected cdp1 and cdp2 to be the exact same instance")
	}

	mu.Lock()
	calls := factoryCalls
	mu.Unlock()

	if calls != 1 {
		t.Fatalf("expected factory to be called exactly once, got %d", calls)
	}
}

func TestConnectionPool_GetConnected(t *testing.T) {
	factory := func(opts ClientOptions) PoolClient {
		return newMockPoolClient(1)
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	cdp, err := pool.GetOrConnect("/path/to/ProjectA")
	if err != nil {
		t.Fatalf("GetOrConnect failed: %v", err)
	}

	connected := pool.GetConnected("ProjectA")
	if connected != cdp {
		t.Fatalf("expected connected client, got %v", connected)
	}

	nonExistent := pool.GetConnected("NonExistent")
	if nonExistent != nil {
		t.Fatalf("expected nil for non-existent workspace, got %v", nonExistent)
	}
}

func TestConnectionPool_DisconnectWorkspace_StopsClientAndDetectors(t *testing.T) {
	var mockClient *mockPoolClient
	factory := func(opts ClientOptions) PoolClient {
		mockClient = newMockPoolClient(1)
		return mockClient
	}

	pool := NewConnectionPool(ClientOptions{}, factory)

	_, err := pool.GetOrConnect("/path/to/ProjectA")
	if err != nil {
		t.Fatalf("GetOrConnect failed: %v", err)
	}

	detector := &mockDetector{active: true}
	pool.RegisterApprovalDetector("ProjectA", detector)

	pool.DisconnectWorkspace("ProjectA")

	mockClient.mu.Lock()
	disconnectCalls := mockClient.disconnectCalls
	mockClient.mu.Unlock()

	if disconnectCalls == 0 {
		t.Fatalf("expected client Disconnect to be called")
	}

	if detector.IsActive() {
		t.Fatalf("expected detector to be stopped")
	}

	if pool.GetConnected("ProjectA") != nil {
		t.Fatalf("expected ProjectA to be removed from pool")
	}

	if pool.GetApprovalDetector("ProjectA") != nil {
		t.Fatalf("expected approval detector to be removed from pool")
	}
}

func TestConnectionPool_DisconnectAll(t *testing.T) {
	var clients []*mockPoolClient
	var mu sync.Mutex

	factory := func(opts ClientOptions) PoolClient {
		mu.Lock()
		c := newMockPoolClient(len(clients) + 1)
		clients = append(clients, c)
		mu.Unlock()
		return c
	}

	pool := NewConnectionPool(ClientOptions{}, factory)

	_, _ = pool.GetOrConnect("/path/to/A")
	_, _ = pool.GetOrConnect("/path/to/B")

	pool.DisconnectAll()

	mu.Lock()
	for i, c := range clients {
		c.mu.Lock()
		calls := c.disconnectCalls
		c.mu.Unlock()
		if calls == 0 {
			t.Errorf("expected client %d to have Disconnect called", i)
		}
	}
	mu.Unlock()

	active := pool.GetActiveWorkspaceNames()
	if len(active) != 0 {
		t.Fatalf("expected 0 active workspaces, got %d", len(active))
	}
}

func TestConnectionPool_DetectorRegistrationAndReplacement(t *testing.T) {
	pool := NewConnectionPool(ClientOptions{})

	detector1 := &mockDetector{active: true}
	detector2 := &mockDetector{active: false}

	pool.RegisterApprovalDetector("ProjectA", detector1)
	if pool.GetApprovalDetector("ProjectA") != detector1 {
		t.Fatalf("expected detector1 to be registered")
	}

	pool.RegisterApprovalDetector("ProjectA", detector2)

	if detector1.IsActive() {
		t.Fatalf("expected detector1 to have been stopped upon replacement")
	}

	if pool.GetApprovalDetector("ProjectA") != detector2 {
		t.Fatalf("expected detector2 to be registered")
	}
}

func TestConnectionPool_GetActiveWorkspaceNames_FiltersDisconnected(t *testing.T) {
	callCount := 0
	factory := func(opts ClientOptions) PoolClient {
		callCount++
		c := newMockPoolClient(callCount)
		if callCount == 2 {
			c.connected = false
		}
		return c
	}

	pool := NewConnectionPool(ClientOptions{}, factory)
	defer pool.DisconnectAll()

	_, _ = pool.GetOrConnect("/path/to/A")
	_, _ = pool.GetOrConnect("/path/to/B")

	active := pool.GetActiveWorkspaceNames()
	hasA, hasB := false, false
	for _, name := range active {
		if name == "A" {
			hasA = true
		}
		if name == "B" {
			hasB = true
		}
	}

	if !hasA {
		t.Fatalf("expected active to contain 'A'")
	}
	if hasB {
		t.Fatalf("expected active NOT to contain 'B'")
	}
}
