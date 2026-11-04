package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ConfigDirName    = ".antigravity-telegram-remote"
	UpdateCheckFile  = "update-check.json"
	CooldownMs       = int64(24 * 60 * 60 * 1000) // 86400000 ms (24 hours)
	CooldownDuration = 24 * time.Hour
	RegistryURL      = "https://registry.npmjs.org/antigravity-telegram-remote/latest"
	RequestTimeoutMs = 5000
	RequestTimeout   = 5 * time.Second
)

// UpdateCheckCache models the JSON cached in ~/.antigravity-telegram-remote/update-check.json.
type UpdateCheckCache struct {
	LastCheck int64 `json:"lastCheck"` // Unix timestamp in milliseconds
}

func getCachePath(homeDirFunc ...func() string) string {
	var home string
	if len(homeDirFunc) > 0 && homeDirFunc[0] != nil {
		home = homeDirFunc[0]()
	} else {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			home = os.Getenv("HOME")
		}
	}
	return filepath.Join(home, ConfigDirName, UpdateCheckFile)
}

// ShouldCheckForUpdates returns true if cache file doesn't exist, is corrupted, or cooldown expired.
func ShouldCheckForUpdates(homeDirFunc ...func() string) bool {
	cachePath := getCachePath(homeDirFunc...)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return true
	}
	var cache UpdateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return true
	}
	return time.Now().UnixMilli()-cache.LastCheck >= CooldownMs
}

// FetchLatestVersion fetches the latest version from npm registry.
func FetchLatestVersion(ctx context.Context, registryURL ...string) (string, error) {
	url := RegistryURL
	if len(registryURL) > 0 && registryURL[0] != "" {
		url = registryURL[0]
	}

	reqCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return "", errors.New("Request timed out")
		}
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	return data.Version, nil
}

func writeCache(homeDirFunc ...func() string) {
	cachePath := getCachePath(homeDirFunc...)
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	cache := UpdateCheckCache{LastCheck: time.Now().UnixMilli()}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	_ = os.WriteFile(cachePath, data, 0644)
}

// CompareSemver compares two semver strings: -1 if a < b, 0 if a == b, 1 if a > b.
func CompareSemver(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var valA, valB int
		if i < len(partsA) {
			valA, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			valB, _ = strconv.Atoi(partsB[i])
		}
		diff := valA - valB
		if diff < 0 {
			return -1
		}
		if diff > 0 {
			return 1
		}
	}
	return 0
}

// CheckForUpdatesWithDeps executes the update check with injected dependencies for testing.
func CheckForUpdatesWithDeps(ctx context.Context, currentVersion string, homeDirFunc func() string, registryURL string, out io.Writer) {
	if !ShouldCheckForUpdates(homeDirFunc) {
		return
	}

	latest, err := FetchLatestVersion(ctx, registryURL)
	if err != nil {
		return
	}

	writeCache(homeDirFunc)

	if CompareSemver(currentVersion, latest) < 0 {
		msg := fmt.Sprintf("\n  Update available: %s → %s — run %snpm i -g antigravity-telegram-remote%s\n", currentVersion, latest, ColorCyan, ColorReset)
		if out != nil {
			fmt.Fprint(out, msg)
		} else {
			fmt.Print(msg)
		}
	}
}

// CheckForUpdates executes non-blocking update check with default home dir and registry URL.
func CheckForUpdates(ctx context.Context, currentVersion string) {
	CheckForUpdatesWithDeps(ctx, currentVersion, nil, "", os.Stdout)
}
