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
Results:
┌──────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Target                    │ Protocol │  Latency │ Success │ Details                                  │
├──────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ntp.example.net:80        │ TCP      │  45.23ms │ true    │ NTP query completed                      │
│ google.com:80             │ TCP      │ 123.45ms │ true    │ SYN sent and acknowledged                 │
│ time.google.com:80        │ TCP      │ 156.78ms │ true    │ NTP query completed                      │
└──────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Bare (One per line)
```
ntp.example.net:80
google.com:80
time.google.com:80
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
