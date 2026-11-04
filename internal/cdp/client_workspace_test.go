package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

type mockWSConn struct {
	mu     sync.Mutex
	readCh chan []byte
	closed chan struct{}
}

func newMockWSConn() *mockWSConn {
	return &mockWSConn{
		readCh: make(chan []byte, 10),
		closed: make(chan struct{}),
	}
}

func (m *mockWSConn) WriteMessage(data []byte) error {
	var req struct {
		ID     int64  `json:"id"`
		Method string `json:"method"`
	}
	if err := json.Unmarshal(data, &req); err == nil && req.ID != 0 {
		resp := fmt.Sprintf(`{"id":%d,"result":{}}`, req.ID)
		select {
		case m.readCh <- []byte(resp):
		default:
		}
	}
	return nil
}

func (m *mockWSConn) ReadMessage() ([]byte, error) {
	select {
	case <-m.closed:
		return nil, io.EOF
	case msg := <-m.readCh:
		return msg, nil
	}
}

func (m *mockWSConn) Ping() error { return nil }
func (m *mockWSConn) SetPongHandler(h func(appData string) error) {
}
func (m *mockWSConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.closed:
	default:
		close(m.closed)
	}
	return nil
}
func (m *mockWSConn) Terminate() error { return m.Close() }

func setupWorkspaceClient(opts ClientOptions, targetTitle ...string) (*Client, *[][]string) {
	var mu sync.Mutex
	var runCalls [][]string

	title := "MyProject"
	if len(targetTitle) > 0 && targetTitle[0] != "" {
		title = targetTitle[0]
	}

	zeroAttempts := 0
	opts.PortsToScan = []int{9999}
	opts.MaxReconnectAttempts = &zeroAttempts
	opts.LaunchPollInterval = 10 * time.Millisecond

	if opts.FindAvailablePortFunc == nil {
		opts.FindAvailablePortFunc = func() (int, bool) {
			return 9999, true
		}
	}

	if opts.DialWebSocketFunc == nil {
		opts.DialWebSocketFunc = func(ctx context.Context, u string) (WebSocketConn, error) {
			return newMockWSConn(), nil
		}
	}

	origRunCmd := opts.RunCommandFunc
	opts.RunCommandFunc = func(command string, args []string) error {
		mu.Lock()
		record := append([]string{command}, args...)
		runCalls = append(runCalls, record)
		mu.Unlock()

		if origRunCmd != nil {
			return origRunCmd(command, args)
		}
		return nil
	}

	callCount := 0
	opts.HTTPGetJSONFunc = func(url string) ([]TargetInfo, error) {
		mu.Lock()
		callCount++
		c := callCount
		mu.Unlock()

		if c == 1 {
			return nil, errors.New("initial pre-launch port scan fails")
		}
		return []TargetInfo{
			{
				ID:                   "new-id",
				Type:                 "page",
				Title:                title,
				URL:                  "file:///workbench",
				WebSocketDebuggerURL: "ws://debug",
			},
		}, nil
	}

	client := NewClient(opts)
	return client, &runCalls
}

func TestClient_LaunchWorkspace_DarwinPrimaryCLI(t *testing.T) {
	client, runCalls := setupWorkspaceClient(ClientOptions{
		GOOS: "darwin",
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := "/Users/test/Documents/MyProject"
	ok, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}

	expected := []string{
		"/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity",
		"--remote-debugging-port=9999",
		"--new-window",
		workspacePath,
	}

	if len(*runCalls) == 0 {
		t.Fatalf("expected runCommand to be called")
	}
	if !reflect.DeepEqual((*runCalls)[0], expected) {
		t.Fatalf("expected %v, got %v", expected, (*runCalls)[0])
	}
}

func TestClient_LaunchWorkspace_DarwinFallbackToOpenApp(t *testing.T) {
	runCount := 0
	client, runCalls := setupWorkspaceClient(ClientOptions{
		GOOS: "darwin",
		RunCommandFunc: func(command string, args []string) error {
			runCount++
			if runCount == 1 {
				return errors.New("Command not found")
			}
			return nil
		},
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := "/Users/test/Documents/MyProject"
	ok, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}

	if len(*runCalls) != 2 {
		t.Fatalf("expected 2 runCommand calls, got %d", len(*runCalls))
	}

	expected1 := []string{
		"/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity",
		"--remote-debugging-port=9999",
		"--new-window",
		workspacePath,
	}
	expected2 := []string{
		"open",
		"-a",
		"Antigravity",
		"--args",
		"--remote-debugging-port=9999",
		workspacePath,
	}

	if !reflect.DeepEqual((*runCalls)[0], expected1) {
		t.Fatalf("expected call 1 %v, got %v", expected1, (*runCalls)[0])
	}
	if !reflect.DeepEqual((*runCalls)[1], expected2) {
		t.Fatalf("expected call 2 %v, got %v", expected2, (*runCalls)[1])
	}
}

func TestClient_LaunchWorkspace_WindowsWithLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\TestUser\AppData\Local`)
	client, runCalls := setupWorkspaceClient(ClientOptions{
		GOOS: "windows",
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := `C:\Source\MyProject`
	ok, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}

	expected := []string{
		`C:\Users\TestUser\AppData\Local\Programs\Antigravity\Antigravity.exe`,
		"--remote-debugging-port=9999",
		"--new-window",
		workspacePath,
	}

	if len(*runCalls) == 0 {
		t.Fatalf("expected runCommand to be called")
	}
	if !reflect.DeepEqual((*runCalls)[0], expected) {
		t.Fatalf("expected %v, got %v", expected, (*runCalls)[0])
	}
}

func TestClient_LaunchWorkspace_WindowsWithoutLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	client, runCalls := setupWorkspaceClient(ClientOptions{
		GOOS: "windows",
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := `C:\Source\MyProject`
	ok, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}

	expected := []string{
		"Antigravity.exe",
		"--remote-debugging-port=9999",
		"--new-window",
		workspacePath,
	}

	if len(*runCalls) == 0 {
		t.Fatalf("expected runCommand to be called")
	}
	if !reflect.DeepEqual((*runCalls)[0], expected) {
		t.Fatalf("expected %v, got %v", expected, (*runCalls)[0])
	}
}

func TestClient_LaunchWorkspace_LinuxDefaultCommand(t *testing.T) {
	t.Setenv("ANTIGRAVITY_PATH", "")
	client, runCalls := setupWorkspaceClient(ClientOptions{
		GOOS: "linux",
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := "/home/user/MyProject"
	ok, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}

	expected := []string{
		"antigravity",
		"--remote-debugging-port=9999",
		"--new-window",
		workspacePath,
	}

	if len(*runCalls) == 0 {
		t.Fatalf("expected runCommand to be called")
	}
	if !reflect.DeepEqual((*runCalls)[0], expected) {
		t.Fatalf("expected %v, got %v", expected, (*runCalls)[0])
	}
}

func TestClient_LaunchWorkspace_LinuxCustomEnvPath(t *testing.T) {
	customPath := "/opt/custom/antigravity.AppImage"
	client, runCalls := setupWorkspaceClient(ClientOptions{
		GOOS: "linux",
		GetAntigravityCliFunc: func() (string, error) {
			return customPath, nil
		},
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := "/home/user/MyProject"
	ok, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}

	expected := []string{
		customPath,
		"--remote-debugging-port=9999",
		"--new-window",
		workspacePath,
	}

	if len(*runCalls) == 0 {
		t.Fatalf("expected runCommand to be called")
	}
	if !reflect.DeepEqual((*runCalls)[0], expected) {
		t.Fatalf("expected %v, got %v", expected, (*runCalls)[0])
	}
}

func TestClient_ProjectNameExtraction_WindowsBackslashes(t *testing.T) {
	client, _ := setupWorkspaceClient(ClientOptions{
		GOOS: "windows",
	})
	defer func() { _ = client.Disconnect() }()

	workspacePath := `C:\Source\MyProject`
	_, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.GetCurrentWorkspaceName() != "MyProject" {
		t.Fatalf("expected workspace name 'MyProject', got %q", client.GetCurrentWorkspaceName())
	}
}

func TestClient_ProjectNameExtraction_PosixSlashes(t *testing.T) {
	client, _ := setupWorkspaceClient(ClientOptions{
		GOOS: "darwin",
	}, "my-cool-project")
	defer func() { _ = client.Disconnect() }()

	workspacePath := "/Users/test/Documents/my-cool-project"
	_, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.GetCurrentWorkspaceName() != "my-cool-project" {
		t.Fatalf("expected workspace name 'my-cool-project', got %q", client.GetCurrentWorkspaceName())
	}
}

func TestClient_ProjectNameExtraction_TrailingSlashes(t *testing.T) {
	client, _ := setupWorkspaceClient(ClientOptions{
		GOOS: "linux",
	}, "trailing-slash-proj")
	defer func() { _ = client.Disconnect() }()

	workspacePath := "/home/user/trailing-slash-proj/"
	_, err := client.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.GetCurrentWorkspaceName() != "trailing-slash-proj" {
		t.Fatalf("expected workspace name 'trailing-slash-proj', got %q", client.GetCurrentWorkspaceName())
	}
}
