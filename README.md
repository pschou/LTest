# Latency Tester

A fast, concurrent TCP/NTP/DNS/ICMP latency tester for testing multiple targets. Uses SYN-only for TCP, full NTP handshake for time servers, DNS queries for domain resolution, and ICMP ping for network latency. Designed to help administrators pick the nearest servers from a list.

## Features

- **Concurrent testing**: Test multiple targets in parallel with configurable concurrency
- **TCP testing**: Connects with SYN, immediately closes connection
- **NTP testing**: Full NTP protocol query with time synchronization
- **DNS testing**: Perform DNS queries to resolve domain names and measure resolution latency
- **ICMP testing**: Send ICMP echo requests (ping) to measure network latency
- **Flexible output**: Configurable output styles (table, bare/raw)
- **Raw output**: Use `-b,--bare` for one target per line output
- **Configurable results**: Specify number of lowest latency responses (-n)
- **Configurable timeout**: Set timeout in milliseconds (-t)

## Installation

```bash
make build
```

## Usage

```bash
./ltest google.com:80 github.com:80
```

### Arguments

```bash
Usage: ltest [--num NUM] [--timeout TIMEOUT] [--kind KIND] [--bare] [--sort] [--reverse] [--version] [--parallel PARALLEL] [--port PORT] [--filter-subnet FILTER-SUBNET] [--dns-query DNS-QUERY] [TARGETS [TARGETS ...]]

Positional arguments:
  TARGETS                Host targets to test ("host:port" or "host" if icmp)

Options:
  --num NUM, -n NUM      Number of latency replies to return [default: all]
  --timeout TIMEOUT, -t TIMEOUT
                         Timeout in milliseconds to consider
  --kind KIND, -k KIND   Protocol: tcp, ntp, dns, icmp, or #/tcp in /etc/services [default: tcp]
  --bare, -b             Only print the targets in the result, one per line
  --sort, -s             Sort the list by latency
  --reverse, -r          Reverse the list (useful with sorting the results)
  --version, -V          Print version and exit
  --parallel PARALLEL, -P PARALLEL
                         Number of concurrent allowed connections [default: 8]
  --port PORT, -p PORT   Custom port to scan (if not specified in the target)
  --filter-subnet FILTER-SUBNET, -f FILTER-SUBNET
                         Filter to one result per subnet (8 = /24)
  --dns-query DNS-QUERY
                         DNS query to make [default: github.com]
  --help, -h             display this help and exit
```

### Examples

```bash
# Test TCP targets, show all results
./ltest google.com:80 github.com:80

# Test TCP targets, show 5 lowest latencies
./ltest -n 5 google.com:80 github.com:80

# Test with custom timeout (3 seconds)
./ltest -t 3000 ntp.example.net google.com:80

# Test a single TCP target
./ltest 192.168.1.1:80

# Raw output: just target names one per line
./ltest -b -n 5 example.net:80 google.com:80 time.google.com:80

# Sort results by latency and show all
./ltest -s example.net:80 google.com:80 github.com:80

# Reverse sort (slowest first)
./ltest -r -s ntp.example.net google.com:80 github.com:80

# Test with high parallelism (20 concurrent connections)
./ltest -k ntp -p 20 ntp.example.net time.google.com time.cloudflare.com

# Test NTP servers specifically
./ltest -k ntp ntp.example.net time.google.com

# Test DNS resolution latency
./ltest -k dns google.com github.com example.com

# Test DNS with custom timeout (2 seconds)
./ltest -k dns -t 2000 google.com github.com example.com

# Print version
./ltest -V
```

## Output Formats

### Default (Table)
```
$ ltest -p 443 gitlab.com github.com yahoo.com altavista.com nasa.gov time.gov
┌───────────────────────────────────────────────────────────────────────────────────────────┐
│ Target        │ Protocol │  Latency  │ Success │ Details                                  │
├───────────────────────────────────────────────────────────────────────────────────────────┤
│ gitlab.com    │ TCP      │ 30.71ms   │ true    │ SYN sent and acknowledged                │
│ github.com    │ TCP      │ 30.69ms   │ true    │ SYN sent and acknowledged                │
│ nasa.gov      │ TCP      │ 30.78ms   │ true    │ SYN sent and acknowledged                │
│ altavista.com │ TCP      │ 30.39ms   │ true    │ SYN sent and acknowledged                │
│ yahoo.com     │ TCP      │ 38.38ms   │ true    │ SYN sent and acknowledged                │
│ time.gov      │ TCP      │ 69.27ms   │ true    │ SYN sent and acknowledged                │
└───────────────────────────────────────────────────────────────────────────────────────────┘

$ ltest -k icmp gitlab.com github.com yahoo.com altavista.com nasa.gov time.gov
┌───────────────────────────────────────────────────────────────────────────────────────────┐
│ Target        │ Protocol │  Latency  │ Success │ Details                                  │
├───────────────────────────────────────────────────────────────────────────────────────────┤
│ github.com    │ ICMP     │ 4.16ms    │ true    │ ICMP echo successful                     │
│ gitlab.com    │ ICMP     │ 3.28ms    │ true    │ ICMP echo successful                     │
│ yahoo.com     │ ICMP     │ 12.39ms   │ true    │ ICMP echo successful                     │
│ altavista.com │ ICMP     │ 3.59ms    │ true    │ ICMP echo successful                     │
│ nasa.gov      │ ICMP     │ 3.22ms    │ true    │ ICMP echo successful                     │
│ time.gov      │ ICMP     │ 5.06236s  │ false   │ no packets received                      │
└───────────────────────────────────────────────────────────────────────────────────────────┘
```

### JSON
Simple ping test with multiple endpoints
```
$ ltest -j -k icmp gitlab.com github.com yahoo.com altavista.com nasa.gov time.gov
[{"Target":"github.com","Protocol":"ICMP","Latency":4193751,"Success":true,"Message":"ICMP echo successful","IP":{"IP":"140.82.112.4","Zone":""}},
{"Target":"yahoo.com","Protocol":"ICMP","Latency":13091938,"Success":true,"Message":"ICMP echo successful","IP":{"IP":"74.6.143.25","Zone":""}},
{"Target":"altavista.com","Protocol":"ICMP","Latency":3022756,"Success":true,"Message":"ICMP echo successful","IP":{"IP":"76.223.84.192","Zone":""}},
{"Target":"nasa.gov","Protocol":"ICMP","Latency":4134529,"Success":true,"Message":"ICMP echo successful","IP":{"IP":"192.0.66.108","Zone":""}},
{"Target":"gitlab.com","Protocol":"ICMP","Latency":3561454,"Success":true,"Message":"ICMP echo successful","IP":{"IP":"172.65.251.78","Zone":""}},
{"Target":"time.gov","Protocol":"ICMP","Latency":5073435931,"Success":false,"Message":"no packets received","IP":null}]
```

Sorted query against ntp servers for finding the closest 5 by using either an NTP query or a TCP threeway handshake:
```
$ ltest -j -s -n 5 -k ntp $( cat list )

$ ltest -j -s -n 5 -p 80 $( cat list )
```

### Bare (One per line)
```
$ ./ltest -b google.com:80 github.com:80
google.com:80
github.com:80
```

## Protocol Details

### TCP
- Uses `net.Dial` with TCP protocol
- Establishes connection, immediately closes it
- Measures time from SYN sent to SYN/ACK received

### NTP
- Uses UDP protocol on port 123
- Sends NTP request packet (mode 3 = client)
- Waits for NTP response with time information
- Full handshake with server time synchronization

### DNS
- Uses UDP protocol to query DNS servers
- Queries DNS servers to resolve domain names
- Measures time from query start to response receipt

### ICMP
- Sends two ICMP echo requests to each target
- Measures the round trip times and keeps the shortest

### Other protocols
- Other protocols are supported as long as they are TCP and are listed in /etc/services

## Building

```bash
# Build for current platform
make build

# Build with version info
make build

# Build for multiple platforms
make build-all

# Build without debug info
make build
```

## Development

```bash
# Install dependencies
make deps

# Run tests
make test

# Clean build artifacts
make clean
```
