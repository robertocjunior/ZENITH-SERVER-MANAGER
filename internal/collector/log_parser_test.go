package collector

import (
	"testing"
)

func TestParseLogLine(t *testing.T) {
	tests := []struct {
		name             string
		source           string
		line             string
		expectedLevel    string
		expectedCategory string
	}{
		{
			name:             "Thread error",
			source:           "appserver",
			line:             "THREAD ERROR ([1234], admin, User) 27/08/2026 10:00:00 variable does not exist",
			expectedLevel:    "ERROR",
			expectedCategory: "THREAD",
		},
		{
			name:             "Fatal server error",
			source:           "appserver",
			line:             "FATAL ERROR: Server out of memory crash dump generated",
			expectedLevel:    "CRITICAL",
			expectedCategory: "SYSTEM",
		},
		{
			name:             "Lock timeout",
			source:           "appserver",
			line:             "LOCK TIMEOUT: Record lock on table SB1010 failed after 30s",
			expectedLevel:    "ERROR",
			expectedCategory: "LOCK",
		},
		{
			name:             "License limit reached",
			source:           "appserver",
			line:             "License Server: limit of licenses exceeded for module SIGAFAT",
			expectedLevel:    "ERROR",
			expectedCategory: "LICENSE",
		},
		{
			name:             "DBAccess connection lost",
			source:           "dbaccess",
			line:             "TOPCONNECT: DBMS Error: connection lost to SQL Server",
			expectedLevel:    "ERROR",
			expectedCategory: "DATABASE",
		},
		{
			name:             "Slow query",
			source:           "dbaccess",
			line:             "Slow query: SELECT * FROM SE1010 took 5200 ms",
			expectedLevel:    "WARN",
			expectedCategory: "SLOW_QUERY",
		},
		{
			name:             "Normal startup",
			source:           "appserver",
			line:             "Server ready to accept connections on port 1234",
			expectedLevel:    "INFO",
			expectedCategory: "SYSTEM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := ParseLogLine(tt.source, tt.line)
			if ev == nil {
				t.Fatalf("expected non-nil event for line: %s", tt.line)
			}
			if ev.Level != tt.expectedLevel {
				t.Errorf("expected level %s, got %s", tt.expectedLevel, ev.Level)
			}
			if ev.Category != tt.expectedCategory {
				t.Errorf("expected category %s, got %s", tt.expectedCategory, ev.Category)
			}
			if ev.Source != tt.source {
				t.Errorf("expected source %s, got %s", tt.source, ev.Source)
			}
		})
	}
}
