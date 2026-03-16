package main

import (
	"context"
	"net"
	"time"
)

// testTCP tests a TCP target with a SYN-only handshake
func testTCP(ctx context.Context, target string, timeoutMs int, defaultPort string) Result {
	start := time.Now()
	var latency time.Duration

	// Parse host:port
	host, portStr, err := parseTarget(target)
	if err != nil {
		return Result{
			Target:   target,
			Protocol: "TCP",
			Latency:  0,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// If port is not specified, use the default TCP port from args
	if portStr == "" && defaultPort != "" {
		portStr = defaultPort
	}

	// Convert timeout to seconds
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// Create dialer with timeout
	dialer := net.Dialer{
		Timeout: timeout,
	}

	// Connect to target
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, portStr))
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "TCP",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}
	defer conn.Close()

	latency = time.Since(start)

	return Result{
		Target:   target,
		IP:       conn.RemoteAddr(),
		Protocol: "TCP",
		Latency:  latency,
		Success:  true,
		Message:  "SYN sent and acknowledged",
	}
}
