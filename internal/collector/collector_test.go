package collector

import (
	"context"
	"testing"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

func TestCollectorServiceMock(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collector.MockMode = true

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	if !svc.IsMock() {
		t.Error("expected mock mode true")
	}

	metrics, tcps, logs, err := svc.CollectCycle(context.Background())
	if err != nil {
		t.Fatalf("unexpected CollectCycle error: %v", err)
	}

	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if len(tcps) == 0 {
		t.Error("expected TCP ports status")
	}
	if len(logs) == 0 {
		t.Error("expected logs")
	}

	snapHost, snapTCP, snapLogs := svc.GetLatestSnapshot()
	if snapHost == nil || len(snapTCP) == 0 || len(snapLogs) == 0 {
		t.Error("expected cached snapshot to contain valid data")
	}
}
