package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/remeh/sizedwaitgroup"
)

// Arguments for the CLI tool
type Args struct {
	N              int      `arg:"-n,--num" help:"Number of latency replies to return (default: all)"`
	T              int      `arg:"-t,--timeout" help:"Timeout in milliseconds to consider"`
	Kind           string   `arg:"-k,--kind" help:"Test type: 'tcp' or 'ntp' (default: tcp)"`
	Bare           bool     `arg:"-b,--bare" help:"Only print the targets in the result, one per line"`
	Targets        []string `arg:"positional" help:"TCP or NTP targets to test"`
	Sort           bool     `arg:"-s,--sort" help:"Sort the list by latency"`
	Reverse        bool     `arg:"-r,--reverse" help:"Reverse the list (useful with sorting the results)"`
	Version        bool     `arg:"-V,--version" help:"Print version and exit"`
	Parallel       int      `arg:"-P,--parallel" help:"Number of concurrent allowed connections (default: 8)"`
	TCPDefaultPort int      `arg:"-p,--tcp-port" help:"Default port for TCP targets"`
	FilterSubnet   int      `arg:"-f,--filter-subnet" help:"Filter to one result per subnet (for example: 8 is /24)" default:"-1"`
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
		args.N = len(args.Targets) // not specified
	}
	if args.T <= 0 {
		args.T = 5000 // 5 seconds default
	}

	if args.Parallel <= 0 {
		args.Parallel = 8
	}

	if len(args.Targets) == 0 {
		fmt.Fprintln(os.Stderr, "No targets specified")
		os.Exit(1)
	}

	// Run tests
	results := make(chan *Result)
	printer := results

	// Collect data
	go func() {
		defer close(results)
		// Allow only concurrent routines
		swg := sizedwaitgroup.New(args.Parallel)
		for i, target := range args.Targets {
			swg.Add()
			go func(i int) {
				defer swg.Done()
				var result Result

				switch args.Kind {
				case "tcp", "":
					result = testTCP(target, args.T, fmt.Sprintf("%d", args.TCPDefaultPort))
				case "ntp":
					result = testNTP(target, args.T)
				default:
					panic("unsupported protocol: " + args.Kind)
				}

				results <- &result
			}(i)
		}
		swg.Wait()
	}()

	if args.Sort {
		printer = make(chan *Result)
		go func() {
			defer close(printer)

			// Collect results
			var allResults []*Result
			for r := range results {
				allResults = append(allResults, r)
			}

			// Sort by latency
			sort.Slice(allResults, func(i, j int) bool {
				if allResults[i].Success == allResults[j].Success {
					return (allResults[i].Latency < allResults[j].Latency) != args.Reverse
				}
				return allResults[i].Success
			})

			for _, r := range allResults {
				printer <- r
			}
		}()
	}

	var dedup = make(map[string]struct{})

	if args.Bare {
		i := 0
		for r := range printer {
			if i == args.N {
				break
			}
			if r.Success {
				baseAddr := getBaseAddress(r.IP, args.FilterSubnet)
				if _, ok := dedup[baseAddr]; !ok {
					if args.FilterSubnet >= 0 {
						dedup[baseAddr] = struct{}{}
					}
					i++
					fmt.Println(r.Target)
				}
			}
		}
	} else {
		// Print results
		fmt.Println("┌───────────────────────────────────────────────────────────────────────────────────────────────────────┐")
		fmt.Println("│ Target                    │ Protocol │  Latency  │ Success │ Details                                  │")
		fmt.Println("├───────────────────────────────────────────────────────────────────────────────────────────────────────┤")

		i := 0
		for r := range printer {
			if i == args.N {
				fmt.Printf("│ %-25s │ %-8s │ %-9s │ %-7s │ % 19d more results omitted │\n",
					"",
					"",
					"",
					"",
					len(args.Targets)-args.N)
				break
			}
			baseAddr := getBaseAddress(r.IP, args.FilterSubnet)
			if _, ok := dedup[baseAddr]; !ok {
				if args.FilterSubnet >= 0 {
					dedup[baseAddr] = struct{}{}
				}
				i++
				fmt.Printf("│ %-25s │ %-8s │ %-9s │ %-7s │ %-40s │\n",
					r.Target,
					r.Protocol,
					fmt.Sprintf("%3.2fms", r.Latency.Seconds()*1000),
					fmt.Sprintf("%v", r.Success),
					strings.ReplaceAll(r.Message, r.Target, ""))
			}

		}

		fmt.Println("└───────────────────────────────────────────────────────────────────────────────────────────────────────┘")
	}
}

// Result represents a single latency test result
type Result struct {
	Target   string
	Protocol string
	Latency  time.Duration
	Success  bool
	Message  string
	IP       net.Addr
}

// testTCP tests a TCP target with a SYN-only handshake
func testTCP(target string, timeoutMs int, defaultPort string) Result {
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
	conn, err := dialer.Dial("tcp", net.JoinHostPort(host, portStr))
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

func getBaseAddress(addr net.Addr, trimBits int) string {
	var ip net.IP

	switch v := addr.(type) {
	case *net.IPAddr:
		ip = v.IP
	case *net.TCPAddr:
		ip = v.IP
	case *net.UDPAddr:
		ip = v.IP
	default:
		return ""
	}

	if ip == nil || trimBits < 0 {
		return ""
	}

	mask := net.CIDRMask(len(ip)*8-trimBits, len(ip)*8)
	baseIP := make(net.IP, len(ip))
	for i := range ip {
		baseIP[i] = ip[i] & mask[i]
	}
	return baseIP.String()
}
