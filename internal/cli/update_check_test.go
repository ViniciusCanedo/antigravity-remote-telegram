package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldCheckForUpdates_NoCacheFile(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	if !ShouldCheckForUpdates(homeFunc) {
		t.Errorf("expected ShouldCheckForUpdates to be true when no cache file exists")
	}
}

func TestShouldCheckForUpdates_OldTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	cacheDir := filepath.Join(tempDir, ConfigDirName)
	_ = os.MkdirAll(cacheDir, 0755)
	oldTimestamp := time.Now().UnixMilli() - CooldownMs - 1000
	cacheData, _ := json.Marshal(UpdateCheckCache{LastCheck: oldTimestamp})
	_ = os.WriteFile(filepath.Join(cacheDir, UpdateCheckFile), cacheData, 0644)

	if !ShouldCheckForUpdates(homeFunc) {
		t.Errorf("expected ShouldCheckForUpdates to be true for old timestamp")
	}
}

func TestShouldCheckForUpdates_RecentTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	cacheDir := filepath.Join(tempDir, ConfigDirName)
	_ = os.MkdirAll(cacheDir, 0755)
	recentTimestamp := time.Now().UnixMilli() - 1000
	cacheData, _ := json.Marshal(UpdateCheckCache{LastCheck: recentTimestamp})
	_ = os.WriteFile(filepath.Join(cacheDir, UpdateCheckFile), cacheData, 0644)

	if ShouldCheckForUpdates(homeFunc) {
		t.Errorf("expected ShouldCheckForUpdates to be false for recent timestamp")
	}
}

func TestShouldCheckForUpdates_CorruptedJson(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	cacheDir := filepath.Join(tempDir, ConfigDirName)
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, UpdateCheckFile), []byte("not-json"), 0644)

	if !ShouldCheckForUpdates(homeFunc) {
		t.Errorf("expected ShouldCheckForUpdates to be true for corrupted JSON")
	}
}

func TestShouldCheckForUpdates_ReadError(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	cacheDir := filepath.Join(tempDir, ConfigDirName)
	_ = os.MkdirAll(cacheDir, 0755)
	filePath := filepath.Join(cacheDir, UpdateCheckFile)
	_ = os.WriteFile(filePath, []byte("{}"), 0000)

	// In unix systems, 0000 causes read error; either way ShouldCheckForUpdates returns true
	res := ShouldCheckForUpdates(homeFunc)
	if !res {
		t.Errorf("expected true on read error")
	}
}

func TestFetchLatestVersion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"1.2.3"}`))
	}))
	defer server.Close()

	version, err := FetchLatestVersion(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", version)
	}
}

func TestFetchLatestVersion_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	_, err := FetchLatestVersion(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected error containing 'HTTP 404', got %v", err)
	}
}

func TestFetchLatestVersion_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	_, err := FetchLatestVersion(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestFetchLatestVersion_NetworkError(t *testing.T) {
	// Connect to non-listening port
	_, err := FetchLatestVersion(context.Background(), "http://127.0.0.1:59999")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestFetchLatestVersion_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := FetchLatestVersion(ctx, server.URL)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected timeout message, got %v", err)
	}
}

func TestCompareSemver(t *testing.T) {
	if CompareSemver("0.1.0", "1.0.0") >= 0 {
		t.Errorf("expected 0.1.0 < 1.0.0")
	}
	if CompareSemver("1.0.0", "0.1.0") <= 0 {
		t.Errorf("expected 1.0.0 > 0.1.0")
	}
	if CompareSemver("1.2.3", "1.2.3") != 0 {
		t.Errorf("expected 1.2.3 == 1.2.3")
	}
	if CompareSemver("0.1.0", "99.0.0") >= 0 {
		t.Errorf("expected 0.1.0 < 99.0.0")
	}
	if CompareSemver("0.1.0", "0.0.9") <= 0 {
		t.Errorf("expected 0.1.0 > 0.0.9")
	}
}

func TestCheckForUpdates_NewerAvailable(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"99.0.0"}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	CheckForUpdatesWithDeps(context.Background(), "0.1.0", homeFunc, server.URL, &buf)

	output := buf.String()
	if !strings.Contains(output, "Update available") {
		t.Errorf("expected notice to contain 'Update available', got: %s", output)
	}
	if !strings.Contains(output, "99.0.0") {
		t.Errorf("expected notice to contain '99.0.0', got: %s", output)
	}

	// Verify cache file was written
	cachePath := filepath.Join(tempDir, ConfigDirName, UpdateCheckFile)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("expected cache file to be written: %v", err)
	}
	if !strings.Contains(string(data), "lastCheck") {
		t.Errorf("expected cache to have 'lastCheck', got: %s", string(data))
	}
}

func TestCheckForUpdates_SameVersion(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	CheckForUpdatesWithDeps(context.Background(), "0.1.0", homeFunc, server.URL, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected no output when versions match, got: %s", buf.String())
	}

	// Should still write cache
	cachePath := filepath.Join(tempDir, ConfigDirName, UpdateCheckFile)
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("expected cache file to be written even when versions match: %v", err)
	}
}

func TestCheckForUpdates_LocalNewer(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.0.9"}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	CheckForUpdatesWithDeps(context.Background(), "0.1.0", homeFunc, server.URL, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected no output when local is newer, got: %s", buf.String())
	}
}

func TestCheckForUpdates_SkipsWithinCooldown(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	cacheDir := filepath.Join(tempDir, ConfigDirName)
	_ = os.MkdirAll(cacheDir, 0755)
	cacheData, _ := json.Marshal(UpdateCheckCache{LastCheck: time.Now().UnixMilli()})
	_ = os.WriteFile(filepath.Join(cacheDir, UpdateCheckFile), cacheData, 0644)

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	CheckForUpdatesWithDeps(context.Background(), "0.1.0", homeFunc, server.URL, &buf)

	if called {
		t.Errorf("expected HTTP request to be skipped within cooldown")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output when skipped, got: %s", buf.String())
	}
}

func TestCheckForUpdates_SwallowsNetworkErrors(t *testing.T) {
	tempDir := t.TempDir()
	homeFunc := func() string { return tempDir }

	var buf bytes.Buffer
	// Invalid server port -> network error
	CheckForUpdatesWithDeps(context.Background(), "0.1.0", homeFunc, "http://127.0.0.1:59999", &buf)

	if buf.Len() != 0 {
		t.Errorf("expected network error to be silently swallowed, got: %s", buf.String())
	}
}
