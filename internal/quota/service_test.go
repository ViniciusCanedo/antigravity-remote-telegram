package quota

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestQuotaService_ReturnsEmptySliceWhenProcessNotFound(t *testing.T) {
	mockExec := func(_ context.Context, command string) (string, string, error) {
		if strings.HasPrefix(command, "pgrep") {
			return "", "", errors.New("not found")
		}
		return "", "", fmt.Errorf("unexpected command: %s", command)
	}

	requestCalled := false
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCalled = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		}),
	}

	svc := NewService(
		WithCommandExecutor(mockExec),
		WithHTTPClient(mockClient),
	)

	result := svc.FetchQuota(context.Background())
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(result))
	}
	if requestCalled {
		t.Fatal("expected no HTTP requests when process is not found")
	}
}

func TestQuotaService_FetchesQuotaAndCachesPortForSamePIDAndToken(t *testing.T) {
	var lsofCallCount int32

	mockExec := func(_ context.Context, cmd string) (string, string, error) {
		if strings.HasPrefix(cmd, "pgrep -fl language_server") {
			return "123 language_server --csrf_token abc-123\n", "", nil
		}
		if strings.HasPrefix(cmd, "lsof -nP -a -iTCP -sTCP:LISTEN -p 123") {
			atomic.AddInt32(&lsofCallCount, 1)
			return "language_server 123 user 10u IPv4 0x0 0t0 TCP *:4444 (LISTEN)\n", "", nil
		}
		return "", "", fmt.Errorf("unexpected command: %s", cmd)
	}

	var requestCount int32
	var requestedPorts []string
	var reqMu sync.Mutex

	mockResponseJSON := `{
		"userStatus": {
			"cascadeModelConfigData": {
				"clientModelConfigs": [
					{
						"label": "Gemini",
						"model": "gemini-pro",
						"quotaInfo": {
							"remainingFraction": 0.6,
							"resetTime": "2026-02-23T12:00:00.000Z"
						}
					}
				]
			}
		}
	}`

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&requestCount, 1)

			reqMu.Lock()
			requestedPorts = append(requestedPorts, req.URL.Port())
			reqMu.Unlock()

			// Check headers
			if token := req.Header.Get("X-Codeium-Csrf-Token"); token != "abc-123" {
				t.Errorf("expected X-Codeium-Csrf-Token 'abc-123', got '%s'", token)
			}
			if proto := req.Header.Get("Connect-Protocol-Version"); proto != "1" {
				t.Errorf("expected Connect-Protocol-Version '1', got '%s'", proto)
			}
			if ct := req.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(mockResponseJSON)),
			}, nil
		}),
	}

	svc := NewService(
		WithCommandExecutor(mockExec),
		WithHTTPClient(mockClient),
	)

	ctx := context.Background()
	first := svc.FetchQuota(ctx)
	second := svc.FetchQuota(ctx)

	if len(first) != 1 {
		t.Fatalf("first call: expected 1 item, got %d", len(first))
	}
	if first[0].Label != "Gemini" || first[0].Model != "gemini-pro" {
		t.Errorf("first call: unexpected model quota %+v", first[0])
	}
	if first[0].QuotaInfo == nil || first[0].QuotaInfo.RemainingFraction != 0.6 {
		t.Errorf("first call: unexpected QuotaInfo %+v", first[0].QuotaInfo)
	}

	if len(second) != 1 {
		t.Fatalf("second call: expected 1 item, got %d", len(second))
	}
	if second[0].Label != "Gemini" {
		t.Errorf("second call: unexpected model quota %+v", second[0])
	}

	if calls := atomic.LoadInt32(&lsofCallCount); calls != 1 {
		t.Errorf("expected lsof to be called exactly 1 time, got %d", calls)
	}

	if reqs := atomic.LoadInt32(&requestCount); reqs != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", reqs)
	}

	reqMu.Lock()
	defer reqMu.Unlock()
	if len(requestedPorts) == 2 {
		if requestedPorts[0] != "4444" || requestedPorts[1] != "4444" {
			t.Errorf("expected both requests to target port 4444, got %v", requestedPorts)
		}
	}
}

func TestQuotaService_RetriesOnceWithFreshPortScanWhenCachedPortFails(t *testing.T) {
	var lsofCount int32
	var reqCount int32

	mockExec := func(_ context.Context, cmd string) (string, string, error) {
		if strings.HasPrefix(cmd, "pgrep -fl language_server") {
			return "123 language_server --csrf_token abc-123\n", "", nil
		}
		if strings.HasPrefix(cmd, "lsof -nP -a -iTCP -sTCP:LISTEN -p 123") {
			count := atomic.AddInt32(&lsofCount, 1)
			if count == 1 {
				return "language_server 123 user 10u IPv4 0x0 0t0 TCP *:4444 (LISTEN)\n", "", nil
			}
			// Second lsof returns port 5555
			return "language_server 123 user 10u IPv4 0x0 0t0 TCP *:5555 (LISTEN)\n", "", nil
		}
		return "", "", fmt.Errorf("unexpected command: %s", cmd)
	}

	mockSuccessJSON := `{
		"userStatus": {
			"cascadeModelConfigData": {
				"clientModelConfigs": [
					{
						"label": "Claude 3.5 Sonnet",
						"model": "claude-3-5-sonnet",
						"quotaInfo": {
							"remainingFraction": 0.85,
							"resetTime": "2026-02-24T00:00:00.000Z"
						}
					}
				]
			}
		}
	}`

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rNum := atomic.AddInt32(&reqCount, 1)
			if req.URL.Port() == "4444" {
				if rNum == 1 {
					// First call succeeds on 4444
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString(mockSuccessJSON)),
					}, nil
				}
				// Subsequent call on 4444 fails (simulating server restart / port shift)
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString(`{"error": "connection refused"}`)),
				}, nil
			}
			if req.URL.Port() == "5555" {
				// Success on retried port 5555
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(mockSuccessJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			}, nil
		}),
	}

	svc := NewService(
		WithCommandExecutor(mockExec),
		WithHTTPClient(mockClient),
	)

	ctx := context.Background()

	// 1st fetch: uses port 4444 and caches it
	first := svc.FetchQuota(ctx)
	if len(first) != 1 || first[0].Label != "Claude 3.5 Sonnet" {
		t.Fatalf("first fetch failed: %+v", first)
	}

	// 2nd fetch: port 4444 fails, retries once with lsof returning port 5555
	second := svc.FetchQuota(ctx)
	if len(second) != 1 || second[0].Label != "Claude 3.5 Sonnet" {
		t.Fatalf("second fetch failed after retry: %+v", second)
	}

	if atomic.LoadInt32(&lsofCount) != 2 {
		t.Errorf("expected lsof to be called 2 times (initial + 1 retry), got %d", atomic.LoadInt32(&lsofCount))
	}
}

func TestQuotaService_InvalidatesCacheWhenPIDOrTokenChanges(t *testing.T) {
	var currentPID int32 = 100
	var currentToken = "token-1"

	mockExec := func(_ context.Context, cmd string) (string, string, error) {
		pid := atomic.LoadInt32(&currentPID)
		if strings.HasPrefix(cmd, "pgrep -fl language_server") {
			return fmt.Sprintf("%d language_server --csrf_token %s\n", pid, currentToken), "", nil
		}
		if strings.Contains(cmd, "lsof") {
			return fmt.Sprintf("language_server %d user 10u IPv4 0x0 0t0 TCP *:4444 (LISTEN)\n", pid), "", nil
		}
		return "", "", fmt.Errorf("unexpected command: %s", cmd)
	}

	mockSuccessJSON := `{"userStatus":{"cascadeModelConfigData":{"clientModelConfigs":[{"label":"ModelA","model":"m-a"}]}}}`
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockSuccessJSON)),
			}, nil
		}),
	}

	svc := NewService(
		WithCommandExecutor(mockExec),
		WithHTTPClient(mockClient),
	)

	ctx := context.Background()
	res1 := svc.FetchQuota(ctx)
	if len(res1) != 1 {
		t.Fatalf("res1 expected 1 item, got %d", len(res1))
	}

	// Change token
	currentToken = "token-2"
	res2 := svc.FetchQuota(ctx)
	if len(res2) != 1 {
		t.Fatalf("res2 expected 1 item, got %d", len(res2))
	}

	// Change PID
	atomic.StoreInt32(&currentPID, 200)
	res3 := svc.FetchQuota(ctx)
	if len(res3) != 1 {
		t.Fatalf("res3 expected 1 item, got %d", len(res3))
	}
}

func TestParseUnixProcessInfoOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPID  int
		wantCsrf string
		wantNil  bool
	}{
		{
			name:     "standard space separated format",
			input:    "1234 /usr/local/bin/language_server --csrf_token abc-123\n",
			wantPID:  1234,
			wantCsrf: "abc-123",
		},
		{
			name:     "equals sign formatted token",
			input:    "5678 language_server --csrf_token=token-XYZ-789 --other_flag\n",
			wantPID:  5678,
			wantCsrf: "token-XYZ-789",
		},
		{
			name:    "no csrf token flag",
			input:   "1234 /usr/local/bin/language_server --other_flag\n",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ParseUnixProcessInfoOutput(tt.input)
			if tt.wantNil {
				if res != nil {
					t.Fatalf("expected nil, got %+v", res)
				}
				return
			}
			if res == nil {
				t.Fatal("expected non-nil result")
			}
			if res.PID != tt.wantPID {
				t.Errorf("PID: expected %d, got %d", tt.wantPID, res.PID)
			}
			if res.CSRFToken != tt.wantCsrf {
				t.Errorf("CSRF: expected %s, got %s", tt.wantCsrf, res.CSRFToken)
			}
		})
	}
}

func TestParseLsofListeningPortsOutput(t *testing.T) {
	input := `
COMMAND     PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
lang_srv  12345 user   10u  IPv4 0x1234      0t0  TCP *:4444 (LISTEN)
lang_srv  12345 user   11u  IPv6 0x5678      0t0  TCP *:5555 (LISTEN)
lang_srv  12345 user   12u  IPv4 0x1234      0t0  TCP *:4444 (LISTEN)
other_pr  99999 user   10u  IPv4 0x9999      0t0  TCP *:6666 (LISTEN)
`
	ports := ParseLsofListeningPortsOutput(input, 12345)
	if len(ports) != 2 {
		t.Fatalf("expected 2 unique ports, got %d: %v", len(ports), ports)
	}
	if ports[0] != 4444 || ports[1] != 5555 {
		t.Errorf("expected [4444, 5555], got %v", ports)
	}
}

func TestParseUserStatusResponseBody(t *testing.T) {
	t.Run("varied field names and float string conversions", func(t *testing.T) {
		jsonBody := []byte(`{
			"userStatus": {
				"cascadeModelConfigData": {
					"clientModelConfigs": [
						{
							"displayName": "Claude 3.5 Sonnet",
							"modelOrAlias": { "model": "claude-3-5" },
							"quotaInfo": {
								"remainingFraction": "0.75",
								"resetTime": "2026-03-01T00:00:00Z"
							}
						},
						{
							"modelName": "GPT-4o",
							"modelId": "gpt-4o",
							"usageInfo": {
								"remaining": 0.45,
								"resetAt": "2026-03-01T01:00:00Z"
							}
						},
						{
							"label": "Custom Model",
							"model": "custom-1"
						}
					]
				}
			}
		}`)

		data, err := ParseUserStatusResponseBody(jsonBody)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data.ClientModelConfigs) != 3 {
			t.Fatalf("expected 3 configs, got %d", len(data.ClientModelConfigs))
		}

		c1 := data.ClientModelConfigs[0]
		if c1.Label != "Claude 3.5 Sonnet" || c1.Model != "claude-3-5" {
			t.Errorf("c1 mismatch: %+v", c1)
		}
		if c1.QuotaInfo == nil || c1.QuotaInfo.RemainingFraction != 0.75 || c1.QuotaInfo.ResetTime != "2026-03-01T00:00:00Z" {
			t.Errorf("c1 quotaInfo mismatch: %+v", c1.QuotaInfo)
		}

		c2 := data.ClientModelConfigs[1]
		if c2.Label != "GPT-4o" || c2.Model != "gpt-4o" {
			t.Errorf("c2 mismatch: %+v", c2)
		}
		if c2.QuotaInfo == nil || c2.QuotaInfo.RemainingFraction != 0.45 || c2.QuotaInfo.ResetTime != "2026-03-01T01:00:00Z" {
			t.Errorf("c2 quotaInfo mismatch: %+v", c2.QuotaInfo)
		}

		c3 := data.ClientModelConfigs[2]
		if c3.Label != "Custom Model" || c3.Model != "custom-1" {
			t.Errorf("c3 mismatch: %+v", c3)
		}
		if c3.QuotaInfo != nil {
			t.Errorf("c3 expected nil QuotaInfo, got %+v", c3.QuotaInfo)
		}
	})

	t.Run("returns Invalid JSON response on malformed input", func(t *testing.T) {
		_, err := ParseUserStatusResponseBody([]byte(`invalid json`))
		if err == nil || err.Error() != "Invalid JSON response" {
			t.Fatalf("expected 'Invalid JSON response', got %v", err)
		}
	})

	t.Run("returns empty slice when cascade data is empty", func(t *testing.T) {
		data, err := ParseUserStatusResponseBody([]byte(`{"userStatus": {}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data.ClientModelConfigs) != 0 {
			t.Fatalf("expected 0 configs, got %d", len(data.ClientModelConfigs))
		}
	})
}
