package collector

import (
	"testing"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

func TestMockCollector(t *testing.T) {
	cfg := config.DefaultConfig()
	mock := NewMockCollector(cfg)
	defer mock.Close()

	metrics, err := mock.CollectMetrics()
	if err != nil {
		t.Fatalf("unexpected error collecting mock metrics: %v", err)
	}

	if metrics.CPUPercent < 0 || metrics.CPUPercent > 100 {
		t.Errorf("expected CPU percent between 0 and 100, got %f", metrics.CPUPercent)
	}
	if len(metrics.Processes) != 2 {
		t.Errorf("expected 2 monitored processes, got %d", len(metrics.Processes))
	}
	if len(metrics.Disks) != 2 {
		t.Errorf("expected 2 disks, got %d", len(metrics.Disks))
	}

	tcps, err := mock.CheckTCPPorts()
	if err != nil {
		t.Fatalf("unexpected error checking TCP: %v", err)
	}
	if len(tcps) != len(cfg.Target.TCPPorts) {
		t.Errorf("expected %d ports, got %d", len(cfg.Target.TCPPorts), len(tcps))
	}
	for _, p := range tcps {
		if !p.Up {
			t.Errorf("expected port %d to be Up in mock", p.Port)
		}
	}

	logs, err := mock.ReadNewLogs()
	if err != nil {
		t.Fatalf("unexpected error reading mock logs: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected at least one mock log event")
	}
}
