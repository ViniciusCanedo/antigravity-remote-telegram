package cdp

import (
	"net"
	"reflect"
	"testing"
)

func TestDefaultCDPPorts(t *testing.T) {
	expected := []int{9222, 9223, 9333, 9444, 9555, 9666}
	if !reflect.DeepEqual(DefaultCDPPorts, expected) {
		t.Fatalf("expected %v, got %v", expected, DefaultCDPPorts)
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Find a dynamically allocated free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Port is currently in use
	if IsPortAvailable(port) {
		t.Fatalf("expected port %d to be in use (false)", port)
	}

	// Close listener -> port should now be available
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}

	if !IsPortAvailable(port) {
		t.Fatalf("expected port %d to be available (true)", port)
	}
}

func TestFindAvailablePort(t *testing.T) {
	// Allocate two listeners
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate port 1: %v", err)
	}
	defer ln1.Close()
	port1 := ln1.Addr().(*net.TCPAddr).Port

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate port 2: %v", err)
	}
	port2 := ln2.Addr().(*net.TCPAddr).Port

	// When both are occupied
	port, ok := FindAvailablePort([]int{port1, port2})
	if ok || port != 0 {
		t.Fatalf("expected (0, false), got (%d, %v)", port, ok)
	}

	// Close ln2
	_ = ln2.Close()
	port, ok = FindAvailablePort([]int{port1, port2})
	if !ok || port != port2 {
		t.Fatalf("expected (%d, true), got (%d, %v)", port2, port, ok)
	}

	// Empty list
	port, ok = FindAvailablePort([]int{})
	if ok || port != 0 {
		t.Fatalf("expected (0, false) for empty slice, got (%d, %v)", port, ok)
	}
}
