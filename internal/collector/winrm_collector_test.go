package collector

import (
	"testing"
)

func TestParseWinRMJSONOutput(t *testing.T) {
	sampleJSON := `
{
  "cpu": 28.5,
  "mem_total_kb": 16777216,
  "mem_free_kb": 4194304,
  "disks": [
    {"device": "C:", "size": 536870912000, "free": 268435456000},
    {"device": "D:", "size": 1073741824000, "free": 805306368000}
  ],
  "processes": [
    {
      "name": "appserver.exe",
      "pid": 4512,
      "ws": 1258291200,
      "vm": 2147483648,
      "cpu": 14.2,
      "threads": 48,
      "handles": 1200
    },
    {
      "name": "dbaccess.exe",
      "pid": 5820,
      "ws": 629145600,
      "vm": 1073741824,
      "cpu": 5.1,
      "threads": 24,
      "handles": 800
    }
  ]
}
`

	metrics, err := parseWinRMJSONOutput(sampleJSON, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to parse WinRM sample: %v", err)
	}

	if metrics.Host != "192.168.1.100" {
		t.Errorf("expected host 192.168.1.100, got %s", metrics.Host)
	}
	if metrics.CPUPercent != 28.5 {
		t.Errorf("expected CPU 28.5, got %f", metrics.CPUPercent)
	}
	if len(metrics.Disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(metrics.Disks))
	}
	if metrics.Disks[0].Device != "C:" {
		t.Errorf("expected disk C:, got %s", metrics.Disks[0].Device)
	}
	if len(metrics.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(metrics.Processes))
	}
	if metrics.Processes[0].Name != "appserver.exe" {
		t.Errorf("expected appserver.exe, got %s", metrics.Processes[0].Name)
	}
	if metrics.Processes[0].ThreadCount != 48 {
		t.Errorf("expected 48 threads, got %d", metrics.Processes[0].ThreadCount)
	}
}
