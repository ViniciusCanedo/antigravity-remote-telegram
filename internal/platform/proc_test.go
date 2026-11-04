package platform

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestParseUnixProcessInfo_Valid(t *testing.T) {
	stdout := `1234 /usr/bin/python3 other.py
56789 /Applications/Antigravity.app/Contents/Resources/app/bin/language_server --csrf_token test-token-12345 --other-flag
9999 /bin/bash script.sh
`
	info := ParseUnixProcessInfo(stdout)
	if info == nil {
		t.Fatal("expected non-nil LanguageServerProcessInfo")
	}
	if info.PID != 56789 {
		t.Fatalf("expected PID 56789, got %d", info.PID)
	}
	if info.CSRFToken != "test-token-12345" {
		t.Fatalf("expected CSRF token 'test-token-12345', got '%s'", info.CSRFToken)
	}
}

func TestParseUnixProcessInfo_WithEquals(t *testing.T) {
	stdout := `4321 /opt/antigravity/language_server --csrf_token=abc-xyz-789 --mode=daemon`
	info := ParseUnixProcessInfo(stdout)
	if info == nil {
		t.Fatal("expected non-nil LanguageServerProcessInfo")
	}
	if info.PID != 4321 {
		t.Fatalf("expected PID 4321, got %d", info.PID)
	}
	if info.CSRFToken != "abc-xyz-789" {
		t.Fatalf("expected CSRF token 'abc-xyz-789', got '%s'", info.CSRFToken)
	}
}

func TestParseUnixProcessInfo_NotFound(t *testing.T) {
	stdout := `1234 /bin/sleep 100`
	info := ParseUnixProcessInfo(stdout)
	if info != nil {
		t.Fatalf("expected nil for no match, got: %+v", info)
	}
}

func TestParseLsofListeningPorts(t *testing.T) {
	targetPID := 12345
	stdout := `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
language_ 12345 user   12u  IPv4 0x1234567890abcdef      0t0  TCP 127.0.0.1:45678 (LISTEN)
language_ 12345 user   13u  IPv6 0xabcdef1234567890      0t0  TCP [::1]:45679 (LISTEN)
language_ 12345 user   14u  IPv4 0x1234567890abcdef      0t0  TCP *:45680 (LISTEN)
language_ 12345 user   15u  IPv4 0x1234567890abcdef      0t0  TCP 127.0.0.1:45678 (LISTEN)
other_pr  99999 user   16u  IPv4 0x1234567890abcdef      0t0  TCP 127.0.0.1:8888 (LISTEN)
`
	ports := ParseLsofListeningPorts(stdout, targetPID)
	expected := []int{45678, 45679, 45680}
	if !reflect.DeepEqual(ports, expected) {
		t.Fatalf("expected ports %v, got %v", expected, ports)
	}
}

func TestParseSSListeningPorts(t *testing.T) {
	targetPID := 12345
	stdout := `State  Recv-Q Send-Q Local Address:Port Peer Address:PortProcess
LISTEN 0      128        127.0.0.1:45678      0.0.0.0:*    users:(("language_server",pid=12345,fd=7))
LISTEN 0      128            [::1]:45679         [::]:*    users:(("language_server",pid=12345,fd=8))
LISTEN 0      128                *:45680            *:*    users:(("language_server",pid=12345,fd=9))
LISTEN 0      128        127.0.0.1:45678      0.0.0.0:*    users:(("language_server",pid=12345,fd=10))
LISTEN 0      128        127.0.0.1:8888       0.0.0.0:*    users:(("other",pid=99999,fd=3))
`
	ports := ParseSSListeningPorts(stdout, targetPID)
	expected := []int{45678, 45679, 45680}
	if !reflect.DeepEqual(ports, expected) {
		t.Fatalf("expected ports %v, got %v", expected, ports)
	}
}

func TestParseWindowsProcessInfo_PowerShellJSON(t *testing.T) {
	stdout := `[
  {
    "ProcessId": 12345,
    "CommandLine": "C:\\Users\\User\\AppData\\Local\\Programs\\Antigravity\\language_server.exe --csrf_token win-token-456 --random"
  }
]`
	info := ParseWindowsProcessInfo(stdout)
	if info == nil {
		t.Fatal("expected non-nil Windows process info")
	}
	if info.PID != 12345 {
		t.Fatalf("expected PID 12345, got %d", info.PID)
	}
	if info.CSRFToken != "win-token-456" {
		t.Fatalf("expected CSRF token 'win-token-456', got '%s'", info.CSRFToken)
	}

	// Single object JSON
	singleJSON := `{"ProcessId": 54321, "CommandLine": "language_server.exe --csrf_token single-token"}`
	infoSingle := ParseWindowsProcessInfo(singleJSON)
	if infoSingle == nil || infoSingle.PID != 54321 || infoSingle.CSRFToken != "single-token" {
		t.Fatalf("unexpected single JSON result: %+v", infoSingle)
	}
}

func TestParseWindowsProcessInfo_WMIC_CSV(t *testing.T) {
	stdout := `Node,CommandLine,ProcessId
DESKTOP-ABC,"language_server.exe --csrf_token=csv-token-789 --flag",67890
`
	info := ParseWindowsProcessInfo(stdout)
	if info == nil {
		t.Fatal("expected non-nil Windows CSV process info")
	}
	if info.PID != 67890 {
		t.Fatalf("expected PID 67890, got %d", info.PID)
	}
	if info.CSRFToken != "csv-token-789" {
		t.Fatalf("expected CSRF token 'csv-token-789', got '%s'", info.CSRFToken)
	}
}

func TestParseNetstatListeningPorts(t *testing.T) {
	targetPID := 12345
	stdout := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    127.0.0.1:54321        0.0.0.0:0              LISTENING       12345
  TCP    [::1]:54322            [::]:0                 LISTENING       12345
  TCP    0.0.0.0:54321          0.0.0.0:0              LISTENING       12345
  TCP    127.0.0.1:9999         0.0.0.0:0              LISTENING       99999
`
	ports := ParseNetstatListeningPorts(stdout, targetPID)
	expected := []int{54321, 54322}
	if !reflect.DeepEqual(ports, expected) {
		t.Fatalf("expected ports %v, got %v", expected, ports)
	}
}

type mockProcessFinder struct {
	processInfoFunc    func(ctx context.Context) (*LanguageServerProcessInfo, error)
	listeningPortsFunc func(ctx context.Context, pid int) ([]int, error)
}

func (m *mockProcessFinder) GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error) {
	if m.processInfoFunc != nil {
		return m.processInfoFunc(ctx)
	}
	return nil, nil
}

func (m *mockProcessFinder) GetListeningPorts(ctx context.Context, pid int) ([]int, error) {
	if m.listeningPortsFunc != nil {
		return m.listeningPortsFunc(ctx, pid)
	}
	return []int{}, nil
}

func TestQuotaService_FetchQuota_SuccessAndCaching(t *testing.T) {
	finder := &mockProcessFinder{
		processInfoFunc: func(ctx context.Context) (*LanguageServerProcessInfo, error) {
			return &LanguageServerProcessInfo{
				PID:       1234,
				CSRFToken: "token-abc",
			}, nil
		},
		listeningPortsFunc: func(ctx context.Context, pid int) ([]int, error) {
			return []int{5001, 5002}, nil
		},
	}

	requestCount := 0
	svc := NewQuotaService(finder)
	svc.requestAPI = func(ctx context.Context, port int, csrfToken string) (*UserStatusData, error) {
		requestCount++
		if port == 5001 {
			return nil, errors.New("port 5001 failed")
		}
		if port == 5002 {
			return &UserStatusData{
				ClientModelConfigs: []ModelQuota{
					{
						Label: "Claude 3.5 Sonnet",
						Model: "claude-3-5-sonnet",
						QuotaInfo: &QuotaInfo{
							RemainingFraction: 0.85,
							ResetTime:         "2026-11-04T18:00:00Z",
						},
					},
				},
			}, nil
		}
		return nil, errors.New("unexpected port")
	}

	ctx := context.Background()

	// 1st call: scans ports 5001 (fails), then 5002 (succeeds)
	quotas1 := svc.FetchQuota(ctx)
	if len(quotas1) != 1 || quotas1[0].Model != "claude-3-5-sonnet" {
		t.Fatalf("unexpected quotas on 1st call: %+v", quotas1)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests on initial call, got %d", requestCount)
	}

	// 2nd call: should use cached port 5002 directly (1 request)
	quotas2 := svc.FetchQuota(ctx)
	if len(quotas2) != 1 || quotas2[0].Model != "claude-3-5-sonnet" {
		t.Fatalf("unexpected quotas on 2nd call: %+v", quotas2)
	}
	if requestCount != 3 {
		t.Fatalf("expected 3 total requests after cached call, got %d", requestCount)
	}
}

func TestQuotaService_FetchQuota_RetryOnCachedPortFailure(t *testing.T) {
	finder := &mockProcessFinder{
		processInfoFunc: func(ctx context.Context) (*LanguageServerProcessInfo, error) {
			return &LanguageServerProcessInfo{
				PID:       2222,
				CSRFToken: "token-retry",
			}, nil
		},
		listeningPortsFunc: func(ctx context.Context, pid int) ([]int, error) {
			return []int{6001, 6002}, nil
		},
	}

	cachedPortActive := true
	svc := NewQuotaService(finder)
	p := 6001
	svc.cachedPort = &p
	svc.cachedPid = 2222
	svc.cachedCsrfToken = "token-retry"

	svc.requestAPI = func(ctx context.Context, port int, csrfToken string) (*UserStatusData, error) {
		if port == 6001 {
			if cachedPortActive {
				cachedPortActive = false
				return nil, errors.New("cached port died")
			}
			return nil, errors.New("port 6001 dead")
		}
		if port == 6002 {
			return &UserStatusData{
				ClientModelConfigs: []ModelQuota{
					{
						Label: "GPT-4o",
						Model: "gpt-4o",
					},
				},
			}, nil
		}
		return nil, errors.New("unknown port")
	}

	quotas := svc.FetchQuota(context.Background())
	if len(quotas) != 1 || quotas[0].Model != "gpt-4o" {
		t.Fatalf("expected retry to succeed with gpt-4o, got: %+v", quotas)
	}
}

func TestQuotaService_FetchQuota_NoProcessFound(t *testing.T) {
	finder := &mockProcessFinder{
		processInfoFunc: func(ctx context.Context) (*LanguageServerProcessInfo, error) {
			return nil, nil
		},
	}
	svc := NewQuotaService(finder)
	quotas := svc.FetchQuota(context.Background())
	if len(quotas) != 0 {
		t.Fatalf("expected empty quotas when no process found, got: %+v", quotas)
	}
}

func TestQuotaService_ThreadSafety(t *testing.T) {
	finder := &mockProcessFinder{
		processInfoFunc: func(ctx context.Context) (*LanguageServerProcessInfo, error) {
			return &LanguageServerProcessInfo{
				PID:       3333,
				CSRFToken: "concurrent-token",
			}, nil
		},
		listeningPortsFunc: func(ctx context.Context, pid int) ([]int, error) {
			return []int{7001}, nil
		},
	}

	svc := NewQuotaService(finder)
	svc.requestAPI = func(ctx context.Context, port int, csrfToken string) (*UserStatusData, error) {
		return &UserStatusData{
			ClientModelConfigs: []ModelQuota{
				{Label: "Model-X", Model: "mx"},
			},
		}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := svc.FetchQuota(context.Background())
			if len(res) != 1 || res[0].Model != "mx" {
				t.Errorf("unexpected concurrent result: %+v", res)
			}
		}()
	}
	wg.Wait()
}

func TestParseUserStatusData_VariedShapes(t *testing.T) {
	rawJSON := `{
  "userStatus": {
    "cascadeModelConfigData": {
      "clientModelConfigs": [
        {
          "displayName": "Claude 3.5",
          "modelOrAlias": {
            "model": "claude-3-5"
          },
          "quota": {
            "remaining": 0.42,
            "resetAt": "2026-11-04T20:00:00Z"
          }
        },
        {
          "modelName": "Gemini 1.5 Pro",
          "modelId": "gemini-1-5-pro",
          "usageInfo": {
            "remainingFraction": "0.75",
            "resetTime": "2026-11-04T22:00:00Z"
          }
        },
        {
          "label": "O1 Preview",
          "model": "o1-preview"
        }
      ]
    }
  }
}`
	data, err := parseUserStatusData([]byte(rawJSON))
	if err != nil {
		t.Fatalf("parseUserStatusData failed: %v", err)
	}

	configs := data.ClientModelConfigs
	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}

	// 1st config
	if configs[0].Label != "Claude 3.5" || configs[0].Model != "claude-3-5" {
		t.Errorf("1st config mismatch: %+v", configs[0])
	}
	if configs[0].QuotaInfo == nil || configs[0].QuotaInfo.RemainingFraction != 0.42 || configs[0].QuotaInfo.ResetTime != "2026-11-04T20:00:00Z" {
		t.Errorf("1st quotaInfo mismatch: %+v", configs[0].QuotaInfo)
	}

	// 2nd config
	if configs[1].Label != "Gemini 1.5 Pro" || configs[1].Model != "gemini-1-5-pro" {
		t.Errorf("2nd config mismatch: %+v", configs[1])
	}
	if configs[1].QuotaInfo == nil || configs[1].QuotaInfo.RemainingFraction != 0.75 || configs[1].QuotaInfo.ResetTime != "2026-11-04T22:00:00Z" {
		t.Errorf("2nd quotaInfo mismatch: %+v", configs[1].QuotaInfo)
	}

	// 3rd config (without quotaInfo)
	if configs[2].Label != "O1 Preview" || configs[2].Model != "o1-preview" {
		t.Errorf("3rd config mismatch: %+v", configs[2])
	}
	if configs[2].QuotaInfo != nil {
		t.Errorf("expected nil quotaInfo for 3rd config, got: %+v", configs[2].QuotaInfo)
	}
}

func TestPlatformProcessFinder_WithMockRunner(t *testing.T) {
	mockRunner := func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "pgrep" {
			return "99999 /Applications/Antigravity.app/Contents/Resources/app/bin/language_server --csrf_token runner-token-123\n", nil
		}
		if name == "lsof" {
			return "language_server 99999 user 12u IPv4 0x123 0t0 TCP 127.0.0.1:8888 (LISTEN)\n", nil
		}
		return "", errors.New("command not supported in mock")
	}

	info, err := GetLanguageServerProcessInfo(context.Background(), mockRunner)
	if err != nil || info == nil {
		t.Fatalf("expected discovered process info, got: %+v (err: %v)", info, err)
	}
	if info.PID != 99999 || info.CSRFToken != "runner-token-123" {
		t.Fatalf("unexpected process info: %+v", info)
	}

	ports, err := GetListeningPorts(context.Background(), 99999, mockRunner)
	if err != nil || len(ports) != 1 || ports[0] != 8888 {
		t.Fatalf("expected listening port [8888], got: %v (err: %v)", ports, err)
	}
}
