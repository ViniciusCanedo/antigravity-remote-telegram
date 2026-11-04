package cdp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestClient_DiscoverTarget_SkipsLaunchpadAndSelectsWorkbench(t *testing.T) {
	var wsURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []TargetInfo{
				{
					Type:                 "page",
					Title:                "Launchpad",
					URL:                  "file:///some/path/launchpad.html",
					WebSocketDebuggerURL: "ws://" + r.Host + "/devtools/page/launchpad-id",
				},
				{
					Type:                 "page",
					Title:                "Antigravity Workspace",
					URL:                  "file:///some/path/workbench.html",
					WebSocketDebuggerURL: wsURL,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	wsURL = "ws://" + u.Host + "/devtools/page/test-id"

	zeroAttempts := 0
	service := NewClient(ClientOptions{
		PortsToScan:          []int{port},
		MaxReconnectAttempts: &zeroAttempts,
	})
	defer func() { _ = service.Disconnect() }()

	targetURL, err := service.DiscoverTarget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if targetURL != wsURL {
		t.Fatalf("expected target URL %q, got %q", wsURL, targetURL)
	}
}

func TestClient_Connect_RecognizesCascadePanelContext(t *testing.T) {
	var wsURL string
	var server *httptest.Server

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []TargetInfo{
				{
					Type:                 "page",
					Title:                "Launchpad",
					URL:                  "file:///some/path/launchpad.html",
					WebSocketDebuggerURL: "ws://" + r.Host + "/devtools/page/launchpad-id",
				},
				{
					Type:                 "page",
					Title:                "Antigravity Workspace",
					URL:                  "file:///some/path/workbench.html",
					WebSocketDebuggerURL: wsURL,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
			return
		}

		if r.URL.Path == "/devtools/page/test-id" {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.CloseNow()

			ctx := r.Context()
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}

				var req struct {
					ID     int64  `json:"id"`
					Method string `json:"method"`
				}
				if err := json.Unmarshal(data, &req); err != nil {
					continue
				}

				if req.Method == "Runtime.enable" {
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"method":"Runtime.executionContextCreated","params":{"context":{"id":1,"name":"top","url":"file:///some/path/workbench.html"}}}`))
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"method":"Runtime.executionContextCreated","params":{"context":{"id":2,"name":"cascade-panel","url":"file:///some/path/cascade-panel.html"}}}`))
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"id":`+strconv.FormatInt(req.ID, 10)+`,"result":{}}`))
				} else if req.Method == "Network.enable" || req.Method == "Runtime.addBinding" {
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"id":`+strconv.FormatInt(req.ID, 10)+`,"result":{}}`))
				}
			}
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	wsURL = "ws://" + u.Host + "/devtools/page/test-id"

	zeroAttempts := 0
	service := NewClient(ClientOptions{
		PortsToScan:          []int{port},
		MaxReconnectAttempts: &zeroAttempts,
	})
	defer func() { _ = service.Disconnect() }()

	if err := service.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !service.IsConnected() {
		t.Fatalf("expected service to be connected")
	}

	time.Sleep(100 * time.Millisecond)

	contexts := service.GetContexts()
	if len(contexts) < 1 {
		t.Fatalf("expected at least 1 context, got %d", len(contexts))
	}

	primaryID := service.GetPrimaryContextID()
	if primaryID == nil || *primaryID != 2 {
		t.Fatalf("expected primary context id to be 2, got %v", primaryID)
	}
}

func TestClient_DisconnectEvent_EmittedOnServerClose(t *testing.T) {
	var wsURL string
	var serverConns []*websocket.Conn
	var connMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []TargetInfo{
				{
					Type:                 "page",
					Title:                "Antigravity Workspace",
					URL:                  "file:///some/path/workbench.html",
					WebSocketDebuggerURL: wsURL,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
			return
		}

		if r.URL.Path == "/devtools/page/test-id" {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			connMu.Lock()
			serverConns = append(serverConns, conn)
			connMu.Unlock()

			ctx := r.Context()
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}
				var req struct {
					ID     int64  `json:"id"`
					Method string `json:"method"`
				}
				if json.Unmarshal(data, &req) == nil {
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"id":`+strconv.FormatInt(req.ID, 10)+`,"result":{}}`))
				}
			}
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	wsURL = "ws://" + u.Host + "/devtools/page/test-id"

	zeroAttempts := 0
	service := NewClient(ClientOptions{
		PortsToScan:          []int{port},
		MaxReconnectAttempts: &zeroAttempts,
	})
	defer func() { _ = service.Disconnect() }()

	if err := service.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !service.IsConnected() {
		t.Fatalf("expected service to be connected")
	}

	disconnectCh := make(chan struct{})
	service.On("disconnected", func(any) {
		close(disconnectCh)
	})

	// Terminate server connection
	connMu.Lock()
	for _, conn := range serverConns {
		_ = conn.CloseNow()
	}
	connMu.Unlock()

	select {
	case <-disconnectCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for disconnected event")
	}

	if service.IsConnected() {
		t.Fatalf("expected service to be disconnected")
	}
}
