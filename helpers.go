package main

import (
	"fmt"
	"net"
)

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

func getBaseAddress(addr net.Addr, trimBits uint8) string {
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

	if ip == nil {
		return ""
	}

	mask := net.CIDRMask(len(ip)*8-int(trimBits), len(ip)*8)
	baseIP := make(net.IP, len(ip))
	for i := range ip {
		baseIP[i] = ip[i] & mask[i]
	}
	return baseIP.String()
}
