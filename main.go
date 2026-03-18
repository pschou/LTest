package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/pschou/LTest/topk"
)

// Arguments for the CLI tool
type Args struct {
	Num          uint     `arg:"-n,--num" help:"Number of latency replies to return [default: all]"`
	Timeout      int      `arg:"-t,--timeout" help:"Timeout in milliseconds to consider"`
	Kind         string   `arg:"-k,--kind" help:"Protocol: tcp, ntp, dns, icmp, or #/tcp in /etc/services" default:"tcp"`
	Bare         bool     `arg:"-b,--bare" help:"Only print the targets in the result, one per line"`
	JSON         bool     `arg:"-j,--json" help:"Print the results in JSON for external parsing"`
	Targets      []string `arg:"positional" help:"Host targets to test (\"host:port\" or \"host\" if icmp)"`
	Sort         bool     `arg:"-s,--sort" help:"Sort the list by latency"`
	Reverse      bool     `arg:"-r,--reverse" help:"Reverse the list (useful with sorting the results)"`
	Version      bool     `arg:"-V,--version" help:"Print version and exit"`
	Parallel     int      `arg:"-P,--parallel" help:"Number of concurrent allowed connections" default:"8"`
	CustomPort   *uint16  `arg:"-p,--port" help:"Custom port to scan (if not specified in the target)"`
	FilterSubnet *uint8   `arg:"-f,--filter-subnet" help:"Filter to one result per subnet (8 = /24)"`
	DNSQuery     string   `arg:"--dns-query" help:"DNS query to make" default:"github.com"`
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
	if args.Num == 0 {
		args.Num = uint(len(args.Targets)) // not specified or invalid request
	}
	if args.Timeout <= 0 {
		args.Timeout = 5000 // 5 seconds default
	}

	switch args.Kind = strings.ToLower(args.Kind); args.Kind {
	case "icmp", "ping":
		if args.CustomPort != nil {
			fmt.Fprintln(os.Stderr, "Latency Tester: Invalid port specification for ICMP")
			os.Exit(1)
		}
	case "tcp", "ntp", "dns":
	default:
		tcpPort := getServicePort(args.Kind)
		if n, err := strconv.ParseUint(tcpPort, 10, 16); n > 0 && err == nil {
			args.Kind = "tcp"
			if args.CustomPort == nil {
				httpPort := uint16(n)
				args.CustomPort = &httpPort
			}
		} else {
			fmt.Fprintln(os.Stderr, "Latency Tester: Kind not supported or not found in /etc/services: ", args.Kind)
			os.Exit(1)
		}
	}

	if args.Parallel <= 0 {
		args.Parallel = 8
	}

	if len(args.Targets) == 0 {
		if args.Bare {
		} else if args.JSON {
			fmt.Println("[]")
		} else {
			fmt.Fprintln(os.Stderr, "Latency Tester: No targets specified")
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Run tests
	results := make(chan *Result)
	printer := results
	ctx, cancel := context.WithCancel(context.Background())

	// Collect data
	go func() {
		var (
			chanClosed bool
			chanLock   sync.Mutex
		)
		defer func() {
			chanLock.Lock()
			defer chanLock.Unlock()
			chanClosed = true
			close(results)
		}()

		var shrinktTimeoutWindowIPFilter = make(map[string]struct{})
		var shrinktTimeoutWindowIPMutex sync.Mutex

		var customPort string
		if args.CustomPort != nil {
			customPort = fmt.Sprintf("%d", *args.CustomPort)
		}

		// Allow only Num concurrent routines
		tk := topk.New(ctx, args.Parallel, int(args.Num),
			time.Duration(args.Timeout)*time.Millisecond*2+time.Second)

		for i, target := range args.Targets {
			ctx, Done := tk.Add()
			go func(i int, ctx context.Context, done topk.DoneFunc) {
				var result Result

				switch args.Kind {
				case "tcp":
					result = testTCP(ctx, target, args.Timeout, customPort)
				case "ntp":
					result = testNTP(ctx, target, args.Timeout, customPort)
				case "dns":
					result = testDNS(ctx, target, args.Timeout, args.DNSQuery, customPort)
				case "icmp", "ping":
					if host, port, err := net.SplitHostPort(target); err == nil && port != "" {
						target = host
					}
					result = testICMP(ctx, target, args.Timeout)
				}

				if args.FilterSubnet != nil {
					baseAddr := getBaseAddress(result.IP, *args.FilterSubnet)
					shrinktTimeoutWindowIPMutex.Lock()
					if _, ok := shrinktTimeoutWindowIPFilter[baseAddr]; !ok && result.Success {
						if baseAddr != "" {
							shrinktTimeoutWindowIPFilter[baseAddr] = struct{}{}
						}
						done(true)
					} else {
						done(false) // Ignore second hits in the filtering efforts, as we don't want to shrink the timeout window too soon.
					}
					shrinktTimeoutWindowIPMutex.Unlock()
				} else {
					done(result.Success)
				}

				chanLock.Lock()
				if !chanClosed {
					results <- &result
				}
				chanLock.Unlock()
			}(i, ctx, Done)
		}
		tk.Wait()
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

	var subnetIPFilter = make(map[string]struct{})
	toPrint := func(r *Result) bool {
		if args.FilterSubnet == nil {
			return true
		}
		baseAddr := getBaseAddress(r.IP, *args.FilterSubnet)
		if _, ok := subnetIPFilter[baseAddr]; !ok {
			if baseAddr != "" {
				subnetIPFilter[baseAddr] = struct{}{}
			}
			return true
		}
		return false
	}

	if args.Bare {
		i := 0
		for r := range printer {
			if r.Success {
				if toPrint(r) {
					i++
					fmt.Println(r.Target)
				}
			}
			if i == int(args.Num) {
				cancel()
				break
			}
		}
	} else if args.JSON {
		fmt.Printf("[")
		var i, j int

		for r := range printer {
			if r.Success {
				if toPrint(r) {
					i++
				}
			}
			j++
			if j > 1 {
				fmt.Printf(",\n")
			}
			dat, _ := json.Marshal(r)
			fmt.Printf("%s", string(dat))

			if i == int(args.Num) {
				cancel()
				break
			}
		}
		fmt.Println("]")
	} else {
		// Print results
		maxLength := 6
		for _, t := range args.Targets {
			if len(t) > maxLength {
				maxLength = len(t)
			}
		}
		maxLengthStr := strconv.Itoa(maxLength)

		fmt.Println("┌─" + dash(maxLength) + "─────────────────────────────────────────────────────────────────────────────┐")
		fmt.Printf("│ %-"+maxLengthStr+"s │ Protocol │  Latency  │ Success │ Details                                  │\n", "Target")
		fmt.Println("├─" + dash(maxLength) + "─────────────────────────────────────────────────────────────────────────────┤")

		var i, j int
		for r := range printer {
			if toPrint(r) {
				i++
				if r.Success {
					j++
				}
				fmt.Printf("│ %-"+maxLengthStr+"s │ %-8s │ %-9s │ %-7s │ %-40s │\n",
					r.Target,
					r.Protocol,
					FormatDuration(r.Latency.Seconds()),
					fmt.Sprintf("%v", r.Success),
					strings.ReplaceAll(r.Message, r.Target, ""))
			}
			if j == int(args.Num) && len(args.Targets) > i {
				cancel()
				fmt.Printf("│ %-"+maxLengthStr+"s │ %-8s │ %-9s │ %-7s │ % 19d more results omitted │\n",
					"",
					"",
					"",
					"",
					len(args.Targets)-i)
				break
			}
		}

		fmt.Println("└─" + dash(maxLength) + "─────────────────────────────────────────────────────────────────────────────┘")
	}
}

func dash(n int) (s string) {
	for ; n > 0; n-- {
		s = s + "─"
	}
	return
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

// FormatDuration formats a given duration to a string with a minimum precision of 0.01 ms
func FormatDuration(duration float64) string {
	// Define units and their corresponding values
	units := []struct {
		value  float64
		suffix string
	}{
		{3600, "h"},
		{60, "m"},
		{1, "s"},
		//{0.001, "ms"},
	}

	// Find the most suitable unit
	for _, unit := range units {
		if math.Abs(duration) >= unit.value {
			// Format the duration to 6 significant digits
			return fmt.Sprintf("%6.6g%s", duration/unit.value, unit.suffix)
		}
	}

	// If the duration is very small, display it in ms with 6 significant digits
	return fmt.Sprintf("%0.2fms", duration*1000)
}
