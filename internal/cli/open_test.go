package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestIsPortAvailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind ephemeral port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// While listening, port should not be available
	if IsPortAvailable(port) {
		t.Errorf("expected port %d to be unavailable while listener is active", port)
	}

	_ = ln.Close()

	// Once closed, port should be available
	if !IsPortAvailable(port) {
		t.Errorf("expected port %d to be available after listener is closed", port)
	}
}

func TestFindAvailablePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind ephemeral port: %v", err)
	}
	defer ln.Close()
	busyPort := ln.Addr().(*net.TCPAddr).Port

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind second ephemeral port: %v", err)
	}
	freePort := ln2.Addr().(*net.TCPAddr).Port
	_ = ln2.Close() // Make freePort available

	ports := []int{busyPort, freePort}
	found := FindAvailablePort(ports)
	if found != freePort {
		t.Errorf("expected FindAvailablePort to return %d, got %d", freePort, found)
	}

	// Test when all ports are busy
	ln3, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(freePort))
	if err == nil {
		defer ln3.Close()
		allBusy := FindAvailablePort([]int{busyPort, freePort})
		if allBusy != 0 {
			t.Errorf("expected 0 when all ports are busy, got %d", allBusy)
		}
	}
}

func TestOpenAction_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind ephemeral port: %v", err)
	}
	freePort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	var launchedPort int
	mockLauncher := func(goos string, port int) error {
		launchedPort = port
		return nil
	}

	var buf bytes.Buffer
	err = OpenActionWithDeps(context.Background(), []int{freePort}, mockLauncher, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if launchedPort != freePort {
		t.Errorf("expected launcher to receive %d, got %d", freePort, launchedPort)
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "Found available port") {
		t.Errorf("expected output to contain 'Found available port', got: %s", outStr)
	}
	if !strings.Contains(outStr, "Antigravity opened on CDP port") {
		t.Errorf("expected output to contain 'Antigravity opened on CDP port', got: %s", outStr)
	}
}

func TestOpenAction_NoPortsAvailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind ephemeral port: %v", err)
	}
	defer ln.Close()
	busyPort := ln.Addr().(*net.TCPAddr).Port

	var buf bytes.Buffer
	err = OpenActionWithDeps(context.Background(), []int{busyPort}, nil, &buf)
	if err == nil {
		t.Fatal("expected error when no ports available, got nil")
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "No available CDP ports found") {
		t.Errorf("expected error message in output, got: %s", outStr)
	}
}

func TestOpenAction_LaunchFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind ephemeral port: %v", err)
	}
	freePort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	mockLauncher := func(goos string, port int) error {
		return errors.New("binary not found")
	}

	var buf bytes.Buffer
	err = OpenActionWithDeps(context.Background(), []int{freePort}, mockLauncher, &buf)
	if err == nil {
		t.Fatal("expected launcher error to propagate, got nil")
	}

	outStr := buf.String()
	if !strings.Contains(outStr, "binary not found") {
		t.Errorf("expected output to contain error message, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Make sure Antigravity is installed") {
		t.Errorf("expected output to contain installation hint, got: %s", outStr)
	}
}
