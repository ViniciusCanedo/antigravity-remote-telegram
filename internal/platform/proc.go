package platform

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"antigravity-telegram-remote/internal/logger"
)

// LanguageServerProcessInfo represents PID and CSRF token discovered from language_server.
type LanguageServerProcessInfo struct {
	PID       int
	CSRFToken string
}

// QuotaInfo represents the remaining fraction and reset time of a model quota.
type QuotaInfo struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

// ModelQuota represents model config and quota returned by LanguageServerService.
type ModelQuota struct {
	Label     string     `json:"label"`
	Model     string     `json:"model"`
	QuotaInfo *QuotaInfo `json:"quotaInfo,omitempty"`
}

// UserStatusData encapsulates client model configs.
type UserStatusData struct {
	ClientModelConfigs []ModelQuota `json:"clientModelConfigs,omitempty"`
}

// CommandRunner abstracts external command execution for testing.
type CommandRunner func(ctx context.Context, name string, args ...string) (stdout string, err error)

// ProcessFinder defines the cross-platform contract for process and port discovery.
type ProcessFinder interface {
	GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error)
	GetListeningPorts(ctx context.Context, pid int) ([]int, error)
}

var csrfTokenRegex = regexp.MustCompile(`--csrf_token[=\s]+([a-zA-Z0-9\-]+)`)

// ParseUnixProcessInfo extracts PID and CSRF token from pgrep output or cmdline text.
func ParseUnixProcessInfo(stdout string) *LanguageServerProcessInfo {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "--csrf_token") {
			trimmed := strings.TrimSpace(line)
			parts := strings.Fields(trimmed)
			if len(parts) == 0 {
				continue
			}
			pid, err := strconv.Atoi(parts[0])
			if err != nil || pid <= 0 {
				continue
			}
			tokenMatch := csrfTokenRegex.FindStringSubmatch(trimmed)
			if len(tokenMatch) > 1 && tokenMatch[1] != "" {
				return &LanguageServerProcessInfo{
					PID:       pid,
					CSRFToken: tokenMatch[1],
				}
			}
		}
	}
	return nil
}

// ParseLsofListeningPorts extracts deduplicated TCP LISTEN ports for pid from lsof output.
func ParseLsofListeningPorts(stdout string, pid int) []int {
	pattern := fmt.Sprintf(`(?im)^\S+\s+%d\s+.*?(?:TCP|UDP)\s+(?:\*|[\d.]+|\[[\da-f:]+\]):(\d+)\s+\(LISTEN\)`, pid)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return []int{}
	}

	matches := re.FindAllStringSubmatch(stdout, -1)
	ports := make([]int, 0, len(matches))
	seen := make(map[int]bool)

	for _, match := range matches {
		if len(match) > 1 {
			port, err := strconv.Atoi(match[1])
			if err == nil && !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	return ports
}

// ParseSSListeningPorts extracts deduplicated TCP LISTEN ports for pid from ss -tlnp output.
func ParseSSListeningPorts(stdout string, pid int) []int {
	pattern := fmt.Sprintf(`(?im)^\s*LISTEN\s+\S+\s+\S+\s+(?:\*|[\d.]+|\[[\da-f:]+\]):(\d+)\s+.*?\bpid=%d\b`, pid)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return []int{}
	}

	matches := re.FindAllStringSubmatch(stdout, -1)
	ports := make([]int, 0, len(matches))
	seen := make(map[int]bool)

	for _, match := range matches {
		if len(match) > 1 {
			port, err := strconv.Atoi(match[1])
			if err == nil && !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	return ports
}

// ParseWindowsProcessInfo extracts PID and CSRF token from PowerShell JSON or WMIC CSV output.
func ParseWindowsProcessInfo(stdout string) *LanguageServerProcessInfo {
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return nil
	}

	type winProcEntry struct {
		ProcessID   int    `json:"ProcessId"`
		CommandLine string `json:"CommandLine"`
	}

	// Try PowerShell JSON array
	var entries []winProcEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err == nil {
		for _, entry := range entries {
			if match := csrfTokenRegex.FindStringSubmatch(entry.CommandLine); len(match) > 1 {
				return &LanguageServerProcessInfo{
					PID:       entry.ProcessID,
					CSRFToken: match[1],
				}
			}
		}
	}

	// Try PowerShell JSON single object
	var single winProcEntry
	if err := json.Unmarshal([]byte(trimmed), &single); err == nil && single.ProcessID > 0 {
		if match := csrfTokenRegex.FindStringSubmatch(single.CommandLine); len(match) > 1 {
			return &LanguageServerProcessInfo{
				PID:       single.ProcessID,
				CSRFToken: match[1],
			}
		}
	}

	// Try WMIC CSV or plain text line by line
	// e.g.: Node,CommandLine,ProcessId
	// HOST,"language_server.exe --csrf_token abc-123",12345
	lines := strings.Split(trimmed, "\n")
	wmicRegex := regexp.MustCompile(`(?i)(?:^|.*,)\s*"?.*` + `--csrf_token[=\s]+([a-zA-Z0-9\-]+).*?"?\s*,\s*(\d+)\s*$`)
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if m := wmicRegex.FindStringSubmatch(l); len(m) > 2 {
			pid, err := strconv.Atoi(m[2])
			if err == nil && pid > 0 {
				return &LanguageServerProcessInfo{
					PID:       pid,
					CSRFToken: m[1],
				}
			}
		}
		// Fallback check if line contains --csrf_token and any digits
		if strings.Contains(l, "--csrf_token") {
			tokenMatch := csrfTokenRegex.FindStringSubmatch(l)
			if len(tokenMatch) > 1 {
				digitsRegex := regexp.MustCompile(`\b(\d+)\b`)
				allDigits := digitsRegex.FindAllString(l, -1)
				if len(allDigits) > 0 {
					pid, err := strconv.Atoi(allDigits[len(allDigits)-1])
					if err == nil && pid > 0 {
						return &LanguageServerProcessInfo{
							PID:       pid,
							CSRFToken: tokenMatch[1],
						}
					}
				}
			}
		}
	}

	return nil
}

// ParseNetstatListeningPorts extracts deduplicated TCP LISTENING ports for pid from netstat -ano output.
func ParseNetstatListeningPorts(stdout string, pid int) []int {
	pattern := fmt.Sprintf(`(?im)^\s*TCP\s+(?:[\d.]+|\[[\da-f:]+\]):(\d+)\s+\S+\s+LISTENING\s+%d\s*$`, pid)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return []int{}
	}

	matches := re.FindAllStringSubmatch(stdout, -1)
	ports := make([]int, 0, len(matches))
	seen := make(map[int]bool)

	for _, match := range matches {
		if len(match) > 1 {
			port, err := strconv.Atoi(match[1])
			if err == nil && !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	return ports
}

// GetLanguageServerProcessInfo discovers PID and CSRF token using the default platform finder.
func GetLanguageServerProcessInfo(ctx context.Context, runner ...CommandRunner) (*LanguageServerProcessInfo, error) {
	finder := newDefaultPlatformProcessFinder(runner...)
	return finder.GetProcessInfo(ctx)
}

// GetListeningPorts discovers listening ports using the default platform finder.
func GetListeningPorts(ctx context.Context, pid int, runner ...CommandRunner) ([]int, error) {
	finder := newDefaultPlatformProcessFinder(runner...)
	return finder.GetListeningPorts(ctx, pid)
}

// QuotaService queries local HTTPS endpoint of language_server with PID/Token/Port cache and 1 retry.
type QuotaService struct {
	mu              sync.Mutex
	cachedPort      *int
	cachedCsrfToken string
	cachedPid       int
	finder          ProcessFinder
	requestAPI      func(ctx context.Context, port int, csrfToken string) (*UserStatusData, error)
}

// defaultRequestAPI performs the loopback HTTPS POST to LanguageServerService/GetUserStatus.
func defaultRequestAPI(ctx context.Context, port int, csrfToken string) (*UserStatusData, error) {
	payload := `{"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}`
	url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/GetUserStatus", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("X-Codeium-Csrf-Token", csrfToken)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Safe for loopback 127.0.0.1
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   2000 * time.Millisecond,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseUserStatusData(body)
}

func parseUserStatusData(body []byte) (*UserStatusData, error) {
	var raw struct {
		UserStatus struct {
			CascadeModelConfigData struct {
				ClientModelConfigs []map[string]any `json:"clientModelConfigs"`
			} `json:"cascadeModelConfigData"`
		} `json:"userStatus"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	rawConfigs := raw.UserStatus.CascadeModelConfigData.ClientModelConfigs
	configs := make([]ModelQuota, 0, len(rawConfigs))

	for _, c := range rawConfigs {
		label := getStringValue(c, "label", "displayName", "modelName")

		model := ""
		if moa, ok := c["modelOrAlias"].(map[string]any); ok {
			if m, ok := moa["model"].(string); ok && m != "" {
				model = m
			}
		}
		if model == "" {
			model = getStringValue(c, "model", "modelId")
		}

		var quotaInfo *QuotaInfo
		var qiMap map[string]any
		for _, key := range []string{"quotaInfo", "quota", "usageInfo"} {
			if val, ok := c[key].(map[string]any); ok {
				qiMap = val
				break
			}
		}

		if qiMap != nil {
			var frac float64
			for _, fKey := range []string{"remainingFraction", "remaining"} {
				if fVal, ok := qiMap[fKey]; ok && fVal != nil {
					switch v := fVal.(type) {
					case float64:
						frac = v
					case float32:
						frac = float64(v)
					case int:
						frac = float64(v)
					case string:
						if f, err := strconv.ParseFloat(v, 64); err == nil {
							frac = f
						}
					}
					break
				}
			}

			resetTime := ""
			for _, rKey := range []string{"resetTime", "resetAt"} {
				if rVal, ok := qiMap[rKey]; ok && rVal != nil {
					if s, ok := rVal.(string); ok {
						resetTime = s
						break
					}
				}
			}

			quotaInfo = &QuotaInfo{
				RemainingFraction: frac,
				ResetTime:         resetTime,
			}
		}

		configs = append(configs, ModelQuota{
			Label:     label,
			Model:     model,
			QuotaInfo: quotaInfo,
		})
	}

	return &UserStatusData{ClientModelConfigs: configs}, nil
}

func getStringValue(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if val, ok := m[k]; ok && val != nil {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// NewQuotaService creates a new QuotaService with optional custom ProcessFinder.
func NewQuotaService(finder ...ProcessFinder) *QuotaService {
	var f ProcessFinder
	if len(finder) > 0 && finder[0] != nil {
		f = finder[0]
	} else {
		f = newDefaultPlatformProcessFinder()
	}

	return &QuotaService{
		finder:     f,
		requestAPI: defaultRequestAPI,
	}
}

// FetchQuota retrieves model quotas from the language_server, using caching and 1 retry.
func (s *QuotaService) FetchQuota(ctx context.Context) []ModelQuota {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchQuotaLocked(ctx, 0)
}

func (s *QuotaService) fetchQuotaLocked(ctx context.Context, retryCount int) []ModelQuota {
	procInfo, err := s.finder.GetProcessInfo(ctx)
	if err != nil || procInfo == nil {
		logger.Error("No language_server process found.")
		return []ModelQuota{}
	}

	pid := procInfo.PID
	csrfToken := procInfo.CSRFToken

	// If PID or Token changed, invalidate cached port
	if s.cachedPid != pid || s.cachedCsrfToken != csrfToken {
		s.cachedPort = nil
		s.cachedPid = pid
		s.cachedCsrfToken = csrfToken
	}

	targetPort := s.cachedPort

	if targetPort == nil {
		ports, err := s.finder.GetListeningPorts(ctx, pid)
		if err != nil || len(ports) == 0 {
			return []ModelQuota{}
		}

		for _, port := range ports {
			data, err := s.requestAPI(ctx, port, csrfToken)
			if err == nil && data != nil {
				p := port
				s.cachedPort = &p
				if data.ClientModelConfigs == nil {
					return []ModelQuota{}
				}
				return data.ClientModelConfigs
			}
		}
		return []ModelQuota{}
	}

	// Try cached port
	data, err := s.requestAPI(ctx, *targetPort, csrfToken)
	if err == nil && data != nil {
		if data.ClientModelConfigs == nil {
			return []ModelQuota{}
		}
		return data.ClientModelConfigs
	}

	// Cache might be invalid — retry once with fresh port scan
	s.cachedPort = nil
	if retryCount < 1 {
		return s.fetchQuotaLocked(ctx, retryCount+1)
	}

	logger.Error(fmt.Sprintf("fetchQuota failed after retry: %v", err))
	return []ModelQuota{}
}
