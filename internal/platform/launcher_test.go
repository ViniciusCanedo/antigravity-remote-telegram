package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"antigravity-telegram-remote/internal/logger"
)

func TestEnsureAntigravityRunning_ReturnsImmediatelyWhenPortResponds(t *testing.T) {
	var debugBuf bytes.Buffer
	var warnBuf bytes.Buffer

	origLogger := logger.Default
	defer func() { logger.Default = origLogger }()

	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		DebugOut: &debugBuf,
		WarnOut:  &warnBuf,
	})
	logger.Default = testLogger

	var queriedPorts sync.Map

	mockHTTPGet := func(ctx context.Context, url string) (int, []byte, error) {
		for _, port := range CDPPorts {
			if strings.Contains(url, fmt.Sprintf(":%d/", port)) {
				queriedPorts.Store(port, true)
				if port == 9222 {
					return 200, []byte("[]"), nil
				}
				return 0, nil, errors.New("connection refused")
			}
		}
		return 0, nil, errors.New("unknown url")
	}

	EnsureAntigravityRunning(mockHTTPGet)

	// Verify all 6 ports were queried in parallel
	for _, port := range CDPPorts {
		if _, ok := queriedPorts.Load(port); !ok {
			t.Fatalf("expected port %d to be queried", port)
		}
	}

	debugOut := debugBuf.String()
	warnOut := warnBuf.String()

	if !strings.Contains(debugOut, "\x1b[2m[DEBUG]\x1b[0m") {
		t.Fatalf("expected debug tag in debug output, got: %s", debugOut)
	}
	if !strings.Contains(debugOut, "[AntigravityLauncher] OK — Port 9222 responding") {
		t.Fatalf("expected port 9222 OK message, got: %s", debugOut)
	}
	if strings.Contains(warnOut, "Antigravity CDP ports are not responding") {
		t.Fatalf("did not expect warning banner when port responded, got: %s", warnOut)
	}
}

func TestEnsureAntigravityRunning_OutputsWarningWhenAllPortsFail(t *testing.T) {
	var debugBuf bytes.Buffer
	var warnBuf bytes.Buffer

	origLogger := logger.Default
	defer func() { logger.Default = origLogger }()

	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		DebugOut: &debugBuf,
		WarnOut:  &warnBuf,
	})
	logger.Default = testLogger

	var queriedPorts sync.Map

	mockHTTPGet := func(ctx context.Context, url string) (int, []byte, error) {
		for _, port := range CDPPorts {
			if strings.Contains(url, fmt.Sprintf(":%d/", port)) {
				queriedPorts.Store(port, true)
			}
		}
		return 0, nil, errors.New("connection refused")
	}

	EnsureAntigravityRunning(mockHTTPGet)

	// Verify all 6 ports were queried
	for _, port := range CDPPorts {
		if _, ok := queriedPorts.Load(port); !ok {
			t.Fatalf("expected port %d to be queried", port)
		}
	}

	warnOut := warnBuf.String()
	if !strings.Contains(warnOut, "\x1b[33m[WARN]\x1b[0m") {
		t.Fatalf("expected warn tag in warn output, got: %s", warnOut)
	}
	if !strings.Contains(warnOut, "  Antigravity CDP ports are not responding") {
		t.Fatalf("expected warning banner message, got: %s", warnOut)
	}
	if !strings.Contains(warnOut, "======================================================================") {
		t.Fatalf("expected 70 '=' divider in banner, got: %s", warnOut)
	}
}

func TestCheckPort_InvalidJSON(t *testing.T) {
	mockHTTPGet := func(ctx context.Context, url string) (int, []byte, error) {
		return 200, []byte("{\"not\": \"array\"}"), nil
	}

	if CheckPort(9222, mockHTTPGet) {
		t.Fatal("expected CheckPort to return false for non-array JSON")
	}
}
