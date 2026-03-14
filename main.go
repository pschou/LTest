package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/remeh/sizedwaitgroup"
)

// Arguments for the CLI tool
type Args struct {
	N        int      `arg:"-n,--num" help:"Number of lowest latency replies to return (default: all)"`
	T        int      `arg:"-t,--timeout" help:"Timeout in milliseconds to consider"`
	Kind     string   `arg:"-k,--kind" help:"Test type: 'tcp' or 'ntp' (default: tcp)"`
	Bare     bool     `arg:"-b,--bare" help:"Only print the targets in the result, one per line"`
	Targets  []string `arg:"positional" help:"TCP or NTP targets to test"`
	Sort     bool     `arg:"-s,--sort" help:"Sort the list by latency"`
	Reverse  bool     `arg:"-r,--reverse" help:"Reverse the list (useful with sorting the results)"`
	Version  bool     `arg:"-V,--version" help:"Print version and exit"`
	Parallel int      `arg:"-p,--parallel" help:"Number of concurrent allowed connections (default: 8)`
}

// NTP packet structure
type NTPPacket struct {
	LeapIndicator  byte
	Version        byte
	Mode           byte
	Stratum        uint8
	Poll           uint8
	Precision      int8
	RootDelay      float64
	RootDispersion float64
	ReferenceID    [4]byte
	RefTimestamp   [8]float64
	OrigTimestamp  [8]float64
	RecvTimestamp  [8]float64
	SendTimestamp  [8]float64
}

var version string

func main() {
	var args Args
	arg.MustParse(&args)

	if args.Version {
		fmt.Println("Latency Tester (https://github.com/pschou/ltest) version:", version)
		return
	}

	// Set defaults
	if args.N <= 0 {
		args.N = -1 // Indicate "not specified"
	}
	if args.T <= 0 {
		args.T = 5000 // 5 seconds default
	}

	if args.Parallel <= 0 {
		args.Parallel = 8
	}

	if len(args.Targets) == 0 {
		fmt.Println("No targets specified")
		os.Exit(1)
	}

	// Run tests
	results := make([]*Result, len(args.Targets))

	// Allow only concurrent routines
	swg := sizedwaitgroup.New(args.Parallel)
	for i, target := range args.Targets {
		swg.Add()
		go func(i int) {
			defer swg.Done()
			var result Result

			switch args.Kind {
			case "tcp", "":
				result = testTCP(target, args.T)
			case "ntp":
				result = testNTP(target, args.T)
			default:
				panic("unsupported protocol: " + args.Kind)
			}

			results[i] = &result
		}(i)
	}
	swg.Wait()

	// Set default for -n if not specified
	if args.N < 0 {
		args.N = len(results)
	}

	if args.Sort {
		// Sort by latency
		sort.Slice(results, func(i, j int) bool {
			if results[i].Success && results[j].Success {
				return results[i].Latency < results[j].Latency
			}
			if results[i].Success {
				return true
			}
			if results[j].Success {
				return false
			}
			return results[i].Latency < results[j].Latency
		})
	}

	if args.Reverse {
		slices.Reverse(results)
	}

	if args.Bare {
		for i := 0; i < min(args.N, len(results)); i++ {
			if results[i].Success {
				fmt.Println(results[i].Target)
			}
		}
	} else {
		// Print results
		fmt.Println("\nResults:")
		fmt.Println("┌──────────────────────────────────────────────────────────────────────────────────────────────────────┐")
		fmt.Println("│ Target                    │ Protocol │  Latency │ Success │ Details                                  │")
		fmt.Println("├──────────────────────────────────────────────────────────────────────────────────────────────────────┤")

		for i := 0; i < min(args.N, len(results)); i++ {
			t := results[i].Latency.Seconds() * 1000
			//if !results[i].Success {
			//	t = math.NaN()
			//}
			fmt.Printf("│ %-25s │ %-8s │ %-8s │ %-7s │ %-40s │\n",
				results[i].Target,
				results[i].Protocol,
				fmt.Sprintf("% 5.2fms", t),
				fmt.Sprintf("%v", results[i].Success),
				strings.ReplaceAll(results[i].Message, results[i].Target, ""))
		}

		if len(results) > args.N {
			fmt.Printf("│ %-26s │ %-8s │ %-8s │ %-8s │ %d more results omitted │\n",
				"",
				"",
				"",
				"",
				len(results)-args.N)
		}

		fmt.Println("└──────────────────────────────────────────────────────────────────────────────────────────────────────┘")
	}
}

// Result represents a single latency test result
type Result struct {
	Target   string
	Protocol string
	Latency  time.Duration
	Success  bool
	Message  string
}

// testTCP tests a TCP target with a SYN-only handshake
func testTCP(target string, timeoutMs int) Result {
	start := time.Now()
	var latency time.Duration

	// Parse host:port
	host, port, err := parseTarget(target)
	if err != nil {
		return Result{
			Target:   target,
			Protocol: "TCP",
			Latency:  0,
			Success:  false,
			Message:  err.Error(),
		}
	}

	// Convert timeout to seconds
	timeout := time.Duration(timeoutMs) * time.Millisecond

	// Create dialer with timeout
	dialer := net.Dialer{
		Timeout: timeout,
	}

	// Connect to target
	conn, err := dialer.Dial("tcp", net.JoinHostPort(host, port))
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
		Protocol: "TCP",
		Latency:  latency,
		Success:  true,
		Message:  "SYN sent and acknowledged",
	}
}

// testNTP performs a full NTP query with time synchronization
func testNTP(target string, timeoutMs int) Result {
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
		port = "123"
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
	conn, err := dialer.Dial("udp", addr)
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

// parseTarget extracts host and port from a target string
func parseTarget(target string) (string, string, error) {
	// Check if target has port
	if host, port, err := net.SplitHostPort(target); err == nil {
		if host == "" {
			return "", "", fmt.Errorf("invalid host:port format")
		}
		return host, port, nil
	}

	// Assume it's just a hostname
	return target, "", nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
