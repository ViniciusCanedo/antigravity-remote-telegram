package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"antigravity-telegram-remote/internal/logger"
)

// CDPPorts contains the exact list of CDP ports scanned for Antigravity connections.
var CDPPorts = []int{9222, 9223, 9333, 9444, 9555, 9666}

// HTTPGetFunc abstracts the HTTP GET call to allow mocks in unit tests.
type HTTPGetFunc func(ctx context.Context, url string) (statusCode int, body []byte, err error)

func defaultHTTPGet(ctx context.Context, url string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	client := &http.Client{
		Timeout: 2000 * time.Millisecond,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// CheckPort checks if the endpoint http://127.0.0.1:<port>/json/list responds within 2000ms
// with a valid JSON array (`[...]`).
func CheckPort(port int, httpGet ...HTTPGetFunc) bool {
	getFn := defaultHTTPGet
	if len(httpGet) > 0 && httpGet[0] != nil {
		getFn = httpGet[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	statusCode, body, err := getFn(ctx, url)
	if err != nil || statusCode != http.StatusOK {
		return false
	}

	var parsed []any
	if err := json.Unmarshal(body, &parsed); err != nil || parsed == nil {
		return false
	}
	return true
}

// EnsureAntigravityRunning checks all ports in CDPPorts in parallel.
// If at least one responds with a valid JSON array, logs at DEBUG level and returns.
// If all fail, emits the complete 70-column warning banner via logger.Warn.
func EnsureAntigravityRunning(httpGet ...HTTPGetFunc) {
	logger.Debug("[AntigravityLauncher] Checking CDP ports...")

	results := make([]bool, len(CDPPorts))
	var wg sync.WaitGroup
	wg.Add(len(CDPPorts))

	for i, port := range CDPPorts {
		go func(idx int, p int) {
			defer wg.Done()
			results[idx] = CheckPort(p, httpGet...)
		}(i, port)
	}

	wg.Wait()

	foundIndex := -1
	for i, ok := range results {
		if ok {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		logger.Debug(fmt.Sprintf("[AntigravityLauncher] OK — Port %d responding", CDPPorts[foundIndex]))
		return
	}

	banner := strings.Repeat("=", 70)
	logger.Warn("")
	logger.Warn(banner)
	logger.Warn("  Antigravity CDP ports are not responding")
	logger.Warn("")
	logger.Warn("  Launch Antigravity with CDP enabled:")
	logger.Warn("    antigravity-telegram-remote open")
	logger.Warn("")
	logger.Warn("  Or manually:")
	logger.Warn("    " + GetAntigravityCdpHint(9222))
	logger.Warn("")
	logger.Warn("  The bot will auto-connect once Antigravity is running.")
	logger.Warn(banner)
	logger.Warn("")
}
