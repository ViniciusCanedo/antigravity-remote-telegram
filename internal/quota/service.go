package quota

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// PgrepLanguageServerCmd is the default command to locate language_server on Unix systems.
	PgrepLanguageServerCmd = "pgrep -fl language_server"

	// LsofListeningPortsFmt is the format string to list listening TCP ports for a PID.
	LsofListeningPortsFmt = "lsof -nP -a -iTCP -sTCP:LISTEN -p %d"

	// Endpoint and HTTPS configuration for local Antigravity language_server.
	QuotaAPIHost                   = "127.0.0.1"
	QuotaAPIPath                   = "/exa.language_server_pb.LanguageServerService/GetUserStatus"
	QuotaAPIMethod                 = "POST"
	QuotaAPITimeout                = 2000 * time.Millisecond
	QuotaHeaderContentType         = "application/json"
	QuotaHeaderConnectProtocolVer  = "1"
	QuotaHeaderCodeiumCsrfTokenKey = "X-Codeium-Csrf-Token"
	QuotaHeaderConnectProtocolKey  = "Connect-Protocol-Version"

	// QuotaRequestBodyJSON is the literal JSON payload required by GetUserStatus.
	QuotaRequestBodyJSON = `{"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}`

	// MaxQuotaCachedPortRetries limits retries when a cached port fails.
	MaxQuotaCachedPortRetries = 1
)

// CsrfTokenRegex extracts the CSRF token from the language_server process arguments.
var CsrfTokenRegex = regexp.MustCompile(`--csrf_token[=\s]+([a-zA-Z0-9\-]+)`)

// BuildLsofPortRegex creates a regular expression matching listening ports for the given PID.
func BuildLsofPortRegex(pid int) *regexp.Regexp {
	pattern := fmt.Sprintf(`(?im)^\S+\s+%d\s+.*?(?:TCP|UDP)\s+(?:\*|[\d.]+|\[[\da-f:]+\]):(\d+)\s+\(LISTEN\)`, pid)
	return regexp.MustCompile(pattern)
}

// QuotaInfo represents the remaining fraction and reset time of a model quota.
type QuotaInfo struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

// ModelQuota represents the configuration and quota state of a model.
type ModelQuota struct {
	Label     string     `json:"label"`
	Model     string     `json:"model"`
	QuotaInfo *QuotaInfo `json:"quotaInfo,omitempty"`
}

// UserStatusData encapsulates client model configurations.
type UserStatusData struct {
	ClientModelConfigs []ModelQuota `json:"clientModelConfigs,omitempty"`
}

// LanguageServerProcessInfo holds PID and CSRF token from the language_server process.
type LanguageServerProcessInfo struct {
	PID       int
	CSRFToken string
}

// CommandExecutor abstracts external shell command execution.
type CommandExecutor func(ctx context.Context, command string) (stdout string, stderr string, err error)

// ProcessDiscoverer abstracts cross-platform process and port discovery.
type ProcessDiscoverer interface {
	GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error)
	GetListeningPorts(ctx context.Context, pid int) ([]int, error)
}

// Service manages language_server process discovery, port resolution, and quota queries.
type Service struct {
	mu              sync.Mutex
	cachedPort      *int
	cachedCsrfToken *string
	cachedPid       *int

	execCmd    CommandExecutor
	discoverer ProcessDiscoverer
	httpClient *http.Client
	logger     *slog.Logger
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithCommandExecutor overrides the shell command execution function.
func WithCommandExecutor(execCmd CommandExecutor) ServiceOption {
	return func(s *Service) {
		s.execCmd = execCmd
	}
}

// WithProcessDiscoverer configures a custom process and port discoverer.
func WithProcessDiscoverer(discoverer ProcessDiscoverer) ServiceOption {
	return func(s *Service) {
		s.discoverer = discoverer
	}
}

// WithHTTPClient overrides the HTTP client used for language_server requests.
func WithHTTPClient(client *http.Client) ServiceOption {
	return func(s *Service) {
		s.httpClient = client
	}
}

// WithLogger overrides the logger used by Service.
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

func defaultCommandExecutor(ctx context.Context, command string) (string, string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// NewService instantiates a new QuotaService.
func NewService(opts ...ServiceOption) *Service {
	s := &Service{
		execCmd: defaultCommandExecutor,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// ParseUnixProcessInfoOutput parses PID and CSRF token from pgrep output.
func ParseUnixProcessInfoOutput(stdout string) *LanguageServerProcessInfo {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "--csrf_token") {
			parts := strings.Fields(strings.TrimSpace(line))
			if len(parts) == 0 {
				continue
			}
			pid, err := strconv.Atoi(parts[0])
			if err != nil || pid <= 0 {
				continue
			}
			cmd := strings.TrimSpace(line[len(parts[0]):])
			match := CsrfTokenRegex.FindStringSubmatch(cmd)
			if len(match) > 1 && match[1] != "" {
				return &LanguageServerProcessInfo{
					PID:       pid,
					CSRFToken: match[1],
				}
			}
		}
	}
	return nil
}

// ParseLsofListeningPortsOutput extracts deduplicated TCP listening ports from lsof output.
func ParseLsofListeningPortsOutput(stdout string, pid int) []int {
	re := BuildLsofPortRegex(pid)
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

// ParseUserStatusResponseBody parses the JSON response from GetUserStatus.
func ParseUserStatusResponseBody(body []byte) (*UserStatusData, error) {
	var raw struct {
		UserStatus struct {
			CascadeModelConfigData struct {
				ClientModelConfigs []map[string]any `json:"clientModelConfigs"`
			} `json:"cascadeModelConfigData"`
		} `json:"userStatus"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("Invalid JSON response")
	}

	rawConfigs := raw.UserStatus.CascadeModelConfigData.ClientModelConfigs
	configs := make([]ModelQuota, 0, len(rawConfigs))

	for _, c := range rawConfigs {
		label := getFirstString(c, "label", "displayName", "modelName")

		model := ""
		if moa, ok := c["modelOrAlias"].(map[string]any); ok {
			if m, ok := moa["model"].(string); ok && m != "" {
				model = m
			}
		}
		if model == "" {
			model = getFirstString(c, "model", "modelId")
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
					case int64:
						frac = float64(v)
					case json.Number:
						if f, err := v.Float64(); err == nil {
							frac = f
						}
					case string:
						if f, err := strconv.ParseFloat(v, 64); err == nil {
							frac = f
						}
					}
					break
				}
			}

			resetTime := getFirstString(qiMap, "resetTime", "resetAt")

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

func getFirstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if val, ok := m[k]; ok && val != nil {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func (s *Service) getUnixProcessInfo(ctx context.Context) *LanguageServerProcessInfo {
	if s.discoverer != nil {
		info, err := s.discoverer.GetProcessInfo(ctx)
		if err != nil {
			s.logger.Error("Failed to get process info", "error", err)
			return nil
		}
		return info
	}

	stdout, _, err := s.execCmd(ctx, PgrepLanguageServerCmd)
	if err != nil {
		s.logger.Error("Failed to get process info", "error", err)
		return nil
	}
	return ParseUnixProcessInfoOutput(stdout)
}

func (s *Service) getListeningPorts(ctx context.Context, pid int) []int {
	if s.discoverer != nil {
		ports, err := s.discoverer.GetListeningPorts(ctx, pid)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to get ports for pid %d", pid), "error", err)
			return []int{}
		}
		return ports
	}

	cmd := fmt.Sprintf(LsofListeningPortsFmt, pid)
	stdout, _, err := s.execCmd(ctx, cmd)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get ports for pid %d", pid), "error", err)
		return []int{}
	}
	return ParseLsofListeningPortsOutput(stdout, pid)
}

func (s *Service) requestAPI(ctx context.Context, port int, csrfToken string) (*UserStatusData, error) {
	url := fmt.Sprintf("https://%s:%d%s", QuotaAPIHost, port, QuotaAPIPath)

	req, err := http.NewRequestWithContext(ctx, QuotaAPIMethod, url, bytes.NewBufferString(QuotaRequestBodyJSON))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", QuotaHeaderContentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(QuotaRequestBodyJSON)))
	req.Header.Set(QuotaHeaderConnectProtocolKey, QuotaHeaderConnectProtocolVer)
	req.Header.Set(QuotaHeaderCodeiumCsrfTokenKey, csrfToken)

	client := s.httpClient
	if client == nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Safe: targets loopback 127.0.0.1
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   QuotaAPITimeout,
		}
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

	return ParseUserStatusResponseBody(body)
}

// FetchQuota retrieves model quotas from the language_server, caching discovery and retrying once on cached port failure.
func (s *Service) FetchQuota(ctx context.Context) []ModelQuota {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchQuotaWithRetry(ctx, 0)
}

func (s *Service) fetchQuotaWithRetry(ctx context.Context, retryCount int) []ModelQuota {
	procInfo := s.getUnixProcessInfo(ctx)
	if procInfo == nil {
		s.logger.Error("No language_server process found.")
		return []ModelQuota{}
	}

	pid := procInfo.PID
	csrfToken := procInfo.CSRFToken

	// If PID or Token changed, invalidate cached port
	if s.cachedPid == nil || *s.cachedPid != pid || s.cachedCsrfToken == nil || *s.cachedCsrfToken != csrfToken {
		s.cachedPort = nil
		s.cachedPid = &pid
		s.cachedCsrfToken = &csrfToken
	}

	targetPort := s.cachedPort

	if targetPort == nil {
		ports := s.getListeningPorts(ctx, pid)
		for _, port := range ports {
			data, err := s.requestAPI(ctx, port, csrfToken)
			if err != nil {
				continue // try next port
			}
			p := port
			s.cachedPort = &p
			if data.ClientModelConfigs == nil {
				return []ModelQuota{}
			}
			return data.ClientModelConfigs
		}
		return []ModelQuota{}
	}

	// Try cached port
	data, err := s.requestAPI(ctx, *targetPort, csrfToken)
	if err == nil {
		if data.ClientModelConfigs == nil {
			return []ModelQuota{}
		}
		return data.ClientModelConfigs
	}

	// Cached port failed — retry once with fresh port scan
	s.cachedPort = nil
	if retryCount < MaxQuotaCachedPortRetries {
		return s.fetchQuotaWithRetry(ctx, retryCount+1)
	}

	s.logger.Error("fetchQuota failed after retry:", "error", err)
	return []ModelQuota{}
}
