package collector

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	reThreadError   = regexp.MustCompile(`(?i)thread\s+error`)
	reFatalError    = regexp.MustCompile(`(?i)(fatal\s+error|out\s+of\s+memory|access\s+violation|dump\s+generated)`)
	reLockTimeout   = regexp.MustCompile(`(?i)(lock\s+timeout|record\s+lock|table\s+locked|waiting\s+lock)`)
	reLicense       = regexp.MustCompile(`(?i)(license|licença|hardkey|sem\s+licença|limite\s+de\s+usu[áa]rios)`)
	reDatabaseError = regexp.MustCompile(`(?i)(dbaccess|topconnect|tclink|dbms\s+error|connection\s+lost|deadlock|sql\s+error|odbc)`)
	reSlowQuery     = regexp.MustCompile(`(?i)(slow\s+query|took\s+\d+\s*ms|exec\s+time\s+exceeded)`)
	reWarn          = regexp.MustCompile(`(?i)(warning|alerta|attention|timeout|running\s+for\s+\d+)`)
)

// ParseLogLine parses a raw line from Protheus console.log or dbaccess.log into a structured LogEvent.
func ParseLogLine(source, line string) *LogEvent {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Filter out noisy routine lines if needed, or keep meaningful lines
	level := "INFO"
	category := "SYSTEM"

	switch {
	case reFatalError.MatchString(line):
		level = "CRITICAL"
		category = "SYSTEM"
	case reThreadError.MatchString(line):
		level = "ERROR"
		category = "THREAD"
	case reLockTimeout.MatchString(line):
		level = "ERROR"
		category = "LOCK"
	case reLicense.MatchString(line):
		level = "WARN"
		category = "LICENSE"
		if strings.Contains(strings.ToLower(line), "exceeded") || strings.Contains(strings.ToLower(line), "sem licença") || strings.Contains(strings.ToLower(line), "fail") {
			level = "ERROR"
		}
	case reDatabaseError.MatchString(line):
		level = "ERROR"
		category = "DATABASE"
	case reSlowQuery.MatchString(line):
		level = "WARN"
		category = "SLOW_QUERY"
	case reWarn.MatchString(line):
		level = "WARN"
		category = "SYSTEM"
	default:
		// Regular informational log
		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "erro") {
			level = "ERROR"
		}
	}

	// Generate deterministic event ID
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", source, line, time.Now().UnixNano())))
	id := fmt.Sprintf("%x", hash[:8])

	return &LogEvent{
		ID:        id,
		Source:    source,
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   line,
		RawLine:   line,
	}
}
