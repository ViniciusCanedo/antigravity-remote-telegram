package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"antigravity-telegram-remote/internal/config"
)

func TestCheckEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(origWd) }()

	exists, path := CheckEnvFile()
	if exists {
		t.Errorf("expected .env to not exist in empty tempDir")
	}
	if filepath.Base(path) != ".env" {
		t.Errorf("expected path to end with .env, got %s", path)
	}

	_ = os.WriteFile(path, []byte("TEST=1"), 0644)
	existsAfter, _ := CheckEnvFile()
	if !existsAfter {
		t.Errorf("expected .env to exist after writing")
	}
}

func TestCheckRequiredEnvVars(t *testing.T) {
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("ALLOWED_USER_IDS")

	statuses := CheckRequiredEnvVars()
	for _, s := range statuses {
		if s.Set {
			t.Errorf("expected %s to be unset", s.Name)
		}
	}

	os.Setenv("TELEGRAM_BOT_TOKEN", "123:token")
	os.Setenv("ALLOWED_USER_IDS", "12345")
	defer func() {
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
		os.Unsetenv("ALLOWED_USER_IDS")
	}()

	statusesSet := CheckRequiredEnvVars()
	for _, s := range statusesSet {
		if !s.Set {
			t.Errorf("expected %s to be set", s.Name)
		}
	}
}

func TestCheckPort(t *testing.T) {
	// Start an HTTP server responding with a JSON array
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"1"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Extract port from server.URL
	parts := strings.Split(server.URL, ":")
	portStr := parts[len(parts)-1]
	port, _ := strconv.Atoi(portStr)

	res := CheckPort(context.Background(), port)
	if !res {
		t.Errorf("expected CheckPort on test server %d to return true", port)
	}

	// Connect to non-existing port
	resBad := CheckPort(context.Background(), 59998)
	if resBad {
		t.Errorf("expected CheckPort on closed port to return false")
	}
}

func TestDoctorAction_MissingVars_Fails(t *testing.T) {
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("ALLOWED_USER_IDS")

	// Ensure config.json doesn't have it either
	tempHome := t.TempDir()
	config.DefaultLoader.HomeDirFunc = func() (string, error) { return tempHome, nil }

	var buf bytes.Buffer
	mockPortChecker := func(ctx context.Context, port int) bool { return true }

	err := DoctorActionWithDeps(context.Background(), DoctorOptions{}, &buf, mockPortChecker)
	if err == nil {
		t.Fatal("expected doctor to fail when required vars are missing")
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "TELEGRAM_BOT_TOKEN is NOT set") {
		t.Errorf("expected output to mention missing token: %s", outStr)
	}
	if !strings.Contains(outStr, "Some checks failed") {
		t.Errorf("expected failure summary: %s", outStr)
	}
}

func TestDoctorAction_NoCDPResponding_Fails(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "mock-token")
	os.Setenv("ALLOWED_USER_IDS", "123456")
	defer func() {
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
		os.Unsetenv("ALLOWED_USER_IDS")
	}()

	tempHome := t.TempDir()
	config.DefaultLoader.HomeDirFunc = func() (string, error) { return tempHome, nil }
	_ = config.Save(config.PersistedConfig{
		TelegramBotToken: stringPtr("mock-token"),
		AllowedUserIds:   []string{"123456"},
	})

	var buf bytes.Buffer
	mockPortChecker := func(ctx context.Context, port int) bool { return false }

	err := DoctorActionWithDeps(context.Background(), DoctorOptions{}, &buf, mockPortChecker)
	if err == nil {
		t.Fatal("expected doctor to fail when no CDP port is responding")
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "No CDP ports responding") {
		t.Errorf("expected 'No CDP ports responding' in output: %s", outStr)
	}
}

func TestDoctorAction_AllPass(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "mock-token")
	os.Setenv("ALLOWED_USER_IDS", "123456")
	defer func() {
		os.Unsetenv("TELEGRAM_BOT_TOKEN")
		os.Unsetenv("ALLOWED_USER_IDS")
	}()

	tempHome := t.TempDir()
	config.DefaultLoader.HomeDirFunc = func() (string, error) { return tempHome, nil }
	_ = config.Save(config.PersistedConfig{
		TelegramBotToken: stringPtr("mock-token"),
		AllowedUserIds:   []string{"123456"},
	})

	// Also fake ANTIGRAVITY_PATH to a temporary executable file so platform check passes
	fakeApp := filepath.Join(tempHome, "fake-antigravity")
	_ = os.WriteFile(fakeApp, []byte("#!/bin/sh\n"), 0755)
	os.Setenv("ANTIGRAVITY_PATH", fakeApp)
	defer os.Unsetenv("ANTIGRAVITY_PATH")

	var buf bytes.Buffer
	mockPortChecker := func(ctx context.Context, port int) bool {
		return port == 9222
	}

	err := DoctorActionWithDeps(context.Background(), DoctorOptions{}, &buf, mockPortChecker)
	if err != nil {
		t.Fatalf("unexpected doctor failure: %v\nOutput: %s", err, buf.String())
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "All checks passed!") {
		t.Errorf("expected 'All checks passed!' in output, got: %s", outStr)
	}
}

func stringPtr(s string) *string {
	return &s
}
