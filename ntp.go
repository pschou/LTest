package main

import (
	"bytes"
	"context"
	"net"
	"time"
)

// testNTP performs a full NTP query with time synchronization
func testNTP(ctx context.Context, target string, timeoutMs int, customPort string) Result {
	start := time.Now()

	var latency time.Duration

	// Parse host and port
	host, port, err := parseTarget(target)
	if err != nil {
		return Result{
			Target:   target,
			Protocol: "NTP",
			Latency:  0,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// Default NTP port is 123
	if port == "" {
		if customPort != "" {
			port = customPort
		} else {
			port = "123"
		}
	}

	// Create UDP address
	addr := net.JoinHostPort(host, port)

	// Create timeout
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// Create UDP dialer with timeout
	dialer := net.Dialer{
		Timeout: timeout,
	}

	// Connect to NTP server
	conn, err := dialer.DialContext(ctx, "udp", addr)
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "NTP",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}
	defer conn.Close()

	// Prepare NTP packet
	packet := buildNTPPacket()

	// Send request
	_, err = conn.Write(packet.Bytes())
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "NTP",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// Receive response
	// Set read deadline
	err = conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "NTP",
			Latency:  latency,
			Success:  false,
			Message:  "Failed to set read deadline",
		}
	}

	// Buffer for response
	buf := make([]byte, 48)
	_, err = conn.Read(buf)
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "NTP",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}

	latency = time.Since(start)

	return Result{
		Target:   target,
		Protocol: "NTP",
		Latency:  latency,
		Success:  true,
		IP:       conn.RemoteAddr(),
		Message:  "NTP query completed",
	}
}

// buildNTPPacket creates a valid NTP request packet
func buildNTPPacket() *bytes.Buffer {
	packet := &bytes.Buffer{}

	// Set mode to client (3) and version to 4
	// First byte: LI=0, VN=4, Mode=3 (client)
	packet.WriteByte(0x23)

	// Rest of the packet initialized to 0
	for i := 0; i < 47; i++ {
		packet.WriteByte(0)
	}

	return packet
}
