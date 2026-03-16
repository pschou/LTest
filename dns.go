package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

// testDNS performs a DNS lookup query
func testDNS(ctx context.Context, target string, timeoutMs int, qryHost string, customPort string) Result {
	start := time.Now()
	var latency time.Duration

	// Parse host and port
	host, port, err := parseTarget(target)
	if err != nil {
		return Result{
			Target:   target,
			Protocol: "DNS",
			Latency:  0,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// Default DNS port is 53
	// Default NTP port is 123
	if port == "" {
		if customPort != "" {
			port = customPort
		} else {
			port = "53"
		}
	}

	// Create timeout
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// Create UDP address
	addr := net.JoinHostPort(host, port)

	// Create DNS client
	client := new(dns.Client)
	client.Timeout = timeout

	// Create DNS question for A record lookup
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qryHost), dns.TypeA)
	msg.RecursionDesired = true

	// Send DNS request and receive response
	conn, err := client.DialContext(ctx, addr)
	if err != nil {
		latency = time.Since(start)
		return Result{
			Target:   target,
			Protocol: "DNS",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}
	defer conn.Close()

	r, _, err := client.ExchangeWithConnContext(ctx, msg, conn)
	latency = time.Since(start)

	if err != nil {
		return Result{
			Target:   target,
			Protocol: "DNS",
			Latency:  latency,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// Check if we got any answers
	if len(r.Answer) == 0 {
		return Result{
			Target:   target,
			Protocol: "DNS",
			Latency:  latency,
			Success:  false,
			Message:  "No answers received",
		}
	}

	// Get the first IP address
	var firstIP string
	for _, ans := range r.Answer {
		if a, ok := ans.(*dns.A); ok {
			if a.A != nil {
				firstIP = a.A.String()
				break
			}
		}
	}

	if firstIP == "" {
		return Result{
			Target:   target,
			Protocol: "DNS",
			Latency:  latency,
			Success:  false,
			Message:  "No A records found",
		}
	}

	return Result{
		Target:   target,
		Protocol: "DNS",
		Latency:  latency,
		Success:  true,
		IP:       conn.RemoteAddr(),
		Message:  fmt.Sprintf("DNS lookup successful, found %d IP(s)", len(r.Answer)),
	}
}
