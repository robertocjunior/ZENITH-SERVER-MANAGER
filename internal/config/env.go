package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// LoadEnvFile loads environment variables from a .env file if present.
// Does not overwrite variables already set in the operating system environment.
func LoadEnvFile(filename string) error {
	if filename == "" {
		filename = ".env"
	}

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // .env is optional
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip inline comment if unquoted
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") && len(val) >= 2 {
			val = val[1 : len(val)-1]
		} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") && len(val) >= 2 {
			val = val[1 : len(val)-1]
		} else {
			// Unquoted: strip comments starting with #
			if commentIdx := strings.Index(val, "#"); commentIdx >= 0 {
				val = strings.TrimSpace(val[:commentIdx])
			}
		}

		// Only set if not already present in OS environment
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}

	return scanner.Err()
}

// parseTCPPortsEnv parses a string like "AppServer:1234,DBAccess:7890" or "1234,7890"
func parseTCPPortsEnv(raw string) []TCPPortTarget {
	var targets []TCPPortTarget
	parts := strings.Split(raw, ",")

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if strings.Contains(p, ":") {
			sub := strings.SplitN(p, ":", 2)
			name := strings.TrimSpace(sub[0])
			portStr := strings.TrimSpace(sub[1])
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				targets = append(targets, TCPPortTarget{Name: name, Port: port})
			}
		} else {
			if port, err := strconv.Atoi(p); err == nil && port > 0 {
				targets = append(targets, TCPPortTarget{Name: "Port " + p, Port: port})
			}
		}
	}

	return targets
}
