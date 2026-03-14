package main

import (
	"net"
	"time"

	ping "github.com/prometheus-community/pro-bing"
)

// testICMP performs a simpler ICMP ping without privileged mode
func testICMP(target string, timeoutMs int) Result {
	start := time.Now()
	var latency time.Duration

	// Parse host and port
	host, port, err := parseTarget(target)
	if err != nil {
		return Result{
			Target:   target,
			Protocol: "ICMP",
			Latency:  0,
			Success:  false,
			Message:  err.Error(),
		}
	}

	_ = port

	// Try using the ping library with non-privileged mode
	pinger, err := ping.NewPinger(host)
	if err != nil {
		panic(err)
	}
	pinger.Timeout = time.Duration(timeoutMs) * time.Millisecond
	pinger.Count = 2
	interval := time.Duration(timeoutMs) * time.Millisecond / 2
	if interval < time.Second/4 {
		interval = time.Second / 4
	} else if interval > time.Second {
		interval = time.Second
	}
	pinger.Interval = interval
	pinger.SetPrivileged(false) // Use non-privileged mode

	// Run the ping
	err = pinger.Run()
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "ICMP",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// Get statistics
	statistics := pinger.Statistics()

	if statistics.PacketsRecv > 0 {
		latency = statistics.MinRtt
	} else {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "ICMP",
			Latency:  latency,
			Success:  false,
			Message:  "no packets received",
		}
	}

	return Result{
		Target:   target,
		Protocol: "ICMP",
		Latency:  latency,
		Success:  true,
		IP:       &net.IPAddr{IP: net.ParseIP(host)},
		Message:  "ICMP echo successful",
	}
}
