package main

import (
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
	N              int      `arg:"-n,--num" help:"Number of latency replies to return [default: all]"`
	T              int      `arg:"-t,--timeout" help:"Timeout in milliseconds to consider"`
	Kind           string   `arg:"-k,--kind" help:"Test type: 'tcp', 'ntp', 'dns', or 'icmp'" default:"tcp"`
	Bare           bool     `arg:"-b,--bare" help:"Only print the targets in the result, one per line"`
	Targets        []string `arg:"positional" help:"TCP or NTP targets to test"`
	Sort           bool     `arg:"-s,--sort" help:"Sort the list by latency"`
	Reverse        bool     `arg:"-r,--reverse" help:"Reverse the list (useful with sorting the results)"`
	Version        bool     `arg:"-V,--version" help:"Print version and exit"`
	Parallel       int      `arg:"-P,--parallel" help:"Number of concurrent allowed connections" default:"8"`
	TCPDefaultPort int      `arg:"-p,--tcp-port" help:"Default port for TCP targets"`
	FilterSubnet   int      `arg:"-f,--filter-subnet" help:"Filter to one result per subnet (8 = /24)" default:"-1"`
	DNSQuery       string   `arg:"--dns-query" help:"DNS query to make" default:"yahoo.com"`
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
				case "dns":
					result = testDNS(target, args.T, args.DNSQuery)
				case "icmp", "ping":
					result = testICMP(target, args.T)
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
