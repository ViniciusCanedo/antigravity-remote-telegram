package cdp

import (
	"fmt"
	"net"
)

// DefaultCDPPorts corresponds to CDP_PORTS in src/utils/cdpPorts.ts:
// export const CDP_PORTS = [9222, 9223, 9333, 9444, 9555, 9666] as const;
var DefaultCDPPorts = []int{9222, 9223, 9333, 9444, 9555, 9666}

// IsPortAvailable checks if a local TCP port on 127.0.0.1 is available for use,
// performing a temporary bind via net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)).
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// FindAvailablePort returns the first free port from the provided list
// (or 0 and false if all candidate ports are occupied).
func FindAvailablePort(ports []int) (int, bool) {
	for _, port := range ports {
		if IsPortAvailable(port) {
			return port, true
		}
	}
	return 0, false
}
