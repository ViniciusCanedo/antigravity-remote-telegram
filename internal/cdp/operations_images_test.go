package cdp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type imageTransferServer struct {
	server           *httptest.Server
	wsURL            string
	port             int
	mu               sync.Mutex
	receivedMessages []recordedMessage
	evalCalls        int
}

func newImageTransferServer(t *testing.T) *imageTransferServer {
	t.Helper()
	its := &imageTransferServer{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []TargetInfo{
				{
					Type:                 "page",
					Title:                "Antigravity Workspace",
					URL:                  "file:///workbench/index.html",
					WebSocketDebuggerURL: its.wsURL,
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

				var req recordedMessage
				if err := json.Unmarshal(data, &req); err != nil {
					continue
				}

				its.mu.Lock()
				its.receivedMessages = append(its.receivedMessages, req)
				its.mu.Unlock()

				switch req.Method {
				case "Runtime.enable":
					evt := map[string]any{
						"method": "Runtime.executionContextCreated",
						"params": map[string]any{
							"context": Context{
								ID:   2,
								Name: "cascade",
								URL:  "file:///workbench/cascade-panel.html",
							},
						},
					}
					evtBytes, _ := json.Marshal(evt)
					_ = conn.Write(ctx, websocket.MessageText, evtBytes)

					resp := map[string]any{"id": req.ID, "result": map[string]any{}}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "Network.enable", "Runtime.addBinding", "DOM.enable", "DOM.setFileInputFiles", "Input.insertText", "Input.dispatchKeyEvent":
					resp := map[string]any{"id": req.ID, "result": map[string]any{}}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "DOM.getDocument":
					resp := map[string]any{
						"id": req.ID,
						"result": map[string]any{
							"root": map[string]any{"nodeId": 1},
						},
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "DOM.querySelector":
					resp := map[string]any{
						"id": req.ID,
						"result": map[string]any{
							"nodeId": 101,
						},
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "Runtime.evaluate":
					its.mu.Lock()
					its.evalCalls++
					callCount := its.evalCalls
					its.mu.Unlock()

					var val any
					if callCount == 1 {
						val = map[string]any{"ok": true}
					} else if callCount == 2 {
						val = map[string]any{"ok": true, "token": "agclaw-token"}
					} else if callCount == 3 {
						val = map[string]any{"ok": true}
					} else {
						val = []map[string]any{
							{
								"name":       "generated-1.png",
								"mimeType":   "image/png",
								"base64Data": base64.StdEncoding.EncodeToString([]byte("image-data")),
							},
						}
					}

					resp := map[string]any{
						"id": req.ID,
						"result": map[string]any{
							"result": map[string]any{
								"value": val,
							},
						},
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)
				}
			}
		}

		http.NotFound(w, r)
	})

	server := httptest.NewServer(handler)
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	its.server = server
	its.port = port
	its.wsURL = "ws://" + u.Host + "/devtools/page/test-id"

	return its
}

func (its *imageTransferServer) close() {
	its.server.Close()
}

func (its *imageTransferServer) getReceivedMessages() []recordedMessage {
	its.mu.Lock()
	defer its.mu.Unlock()
	copied := make([]recordedMessage, len(its.receivedMessages))
	copy(copied, its.receivedMessages)
	return copied
}

func (its *imageTransferServer) setEvalCalls(calls int) {
	its.mu.Lock()
	defer its.mu.Unlock()
	its.evalCalls = calls
}

func TestClient_InjectMessageWithImageFiles_AttachesAndSends(t *testing.T) {
	srv := newImageTransferServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	result, err := client.InjectMessageWithImageFiles(
		"画像付きプロンプト",
		[]string{"/tmp/agclaw-a.png", "/tmp/agclaw-b.jpg"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK == true, got false, error: %s", result.Error)
	}
	if result.ContextID == nil || *result.ContextID != 2 {
		t.Errorf("expected ContextID == 2, got %v", result.ContextID)
	}

	messages := srv.getReceivedMessages()
	var domSetFilesCall *recordedMessage
	for _, m := range messages {
		if m.Method == "DOM.setFileInputFiles" {
			domSetFilesCall = &m
			break
		}
	}
	if domSetFilesCall == nil {
		t.Fatalf("expected DOM.setFileInputFiles to be called")
	}

	filesRaw, ok := domSetFilesCall.Params["files"].([]any)
	if !ok {
		t.Fatalf("files param missing or wrong type")
	}
	var files []string
	for _, f := range filesRaw {
		files = append(files, f.(string))
	}
	if len(files) != 2 || files[0] != "/tmp/agclaw-a.png" || files[1] != "/tmp/agclaw-b.jpg" {
		t.Errorf("files param mismatch, got %v", files)
	}
	if nid, ok := domSetFilesCall.Params["nodeId"].(float64); !ok || int(nid) != 101 {
		t.Errorf("expected nodeId == 101, got %v", domSetFilesCall.Params["nodeId"])
	}
}

func TestClient_InjectMessageWithImageFiles_NoDuplicateDispatchEvent(t *testing.T) {
	srv := newImageTransferServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	_, err := client.InjectMessageWithImageFiles(
		"test prompt",
		[]string{"/tmp/test-image.png"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := srv.getReceivedMessages()
	var notifyScriptCalls []recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			expr, _ := m.Params["expression"].(string)
			if strings.Contains(expr, "data-agclaw-upload-token") {
				notifyScriptCalls = append(notifyScriptCalls, m)
			}
		}
	}

	if len(notifyScriptCalls) == 0 {
		t.Fatalf("expected at least one notifyScript call")
	}

	for _, call := range notifyScriptCalls {
		expr, _ := call.Params["expression"].(string)
		if strings.Contains(expr, "dispatchEvent") {
			t.Errorf("notifyScript should NOT contain 'dispatchEvent', found: %s", expr)
		}
	}
}

func TestClient_ExtractLatestResponseImages_ReturnsExtractedImages(t *testing.T) {
	srv := newImageTransferServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)
	srv.setEvalCalls(3)

	images := client.ExtractLatestResponseImages(4)
	if len(images) != 1 {
		t.Fatalf("expected 1 extracted image, got %d", len(images))
	}

	img := images[0]
	if img.Name != "generated-1.png" {
		t.Errorf("expected Name == 'generated-1.png', got %q", img.Name)
	}
	if img.MimeType != "image/png" {
		t.Errorf("expected MimeType == 'image/png', got %q", img.MimeType)
	}
	if img.Base64Data == "" {
		t.Errorf("expected non-empty Base64Data")
	}
}
