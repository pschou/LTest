package main

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// getServicePort returns the TCP port for a given service
func getServicePort(service string) string {
	// Regular expression to match service lines in /etc/services
	re := regexp.MustCompile(`^\s*(\w+)\s+(\d+)\/tcp`)

	filePath := "/etc/services"
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindStringSubmatch(line)
		if len(match) == 3 {
			name := strings.TrimSpace(match[1])
			port := match[2]
			if strings.EqualFold(name, service) {
				return strings.TrimSpace(port)
			}
		}
	}
	return ""
}
