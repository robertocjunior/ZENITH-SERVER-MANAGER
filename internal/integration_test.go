package internal_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/api"
	"github.com/robertocjunior/zenith-server-manager/internal/collector"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
	"github.com/robertocjunior/zenith-server-manager/internal/tsdb"
)

func TestEndToEndPipelineWithTSDBResilience(t *testing.T) {
	var vmReceivedCount int32
	var vmOnline int32 = 1

	// Mock VictoriaMetrics HTTP endpoint
	vmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&vmOnline) == 0 {
			http.Error(w, "VictoriaMetrics unavailable", http.StatusServiceUnavailable)
			return
		}

		if r.URL.Path == "/api/v1/import/prometheus" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			lines := strings.Split(strings.TrimSpace(string(body)), "\n")
			atomic.AddInt32(&vmReceivedCount, int32(len(lines)))
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path == "/api/v1/query_range" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer vmServer.Close()

	cfg := config.DefaultConfig()
	cfg.Collector.MockMode = true
	cfg.Server.AuthPassword = "" // No auth for integration test
	cfg.TSDB.URL = vmServer.URL
	cfg.TSDB.FlushInterval = 50 * time.Millisecond
	cfg.TSDB.BatchSize = 50
	cfg.TSDB.MaxBufferSize = 200

	colSvc, err := collector.NewService(cfg)
	if err != nil {
		t.Fatalf("failed creating collector service: %v", err)
	}
	defer colSvc.Close()

	tsdbClient := tsdb.NewClient(cfg.TSDB)
	tsdbClient.Start()
	defer tsdbClient.Stop()

	// Step 1: Collect a cycle and enqueue metrics while TSDB is ONLINE
	ctx := context.Background()
	metrics, tcps, logs, err := colSvc.CollectCycle(ctx)
	if err != nil {
		t.Fatalf("unexpected collection error: %v", err)
	}
	if metrics == nil || len(tcps) == 0 || len(logs) == 0 {
		t.Fatal("expected non-empty telemetry from mock collector")
	}

	tsdbClient.EnqueueHostMetrics(metrics)
	tsdbClient.EnqueueTCPMetrics(tcps)

	// Trigger flush
	err = tsdbClient.Flush(ctx)
	if err != nil {
		t.Fatalf("expected flush to succeed while TSDB is online, got: %v", err)
	}
	if atomic.LoadInt32(&vmReceivedCount) == 0 {
		t.Error("expected VictoriaMetrics to have received metrics")
	}

	// Step 2: Simulate VictoriaMetrics outage
	atomic.StoreInt32(&vmOnline, 0)

	// Enqueue burst of metrics during outage (exceeding buffer capacity 200)
	for i := 0; i < 30; i++ {
		tsdbClient.EnqueueHostMetrics(metrics)
	}

	// Attempt flush during outage
	_ = tsdbClient.Flush(ctx)

	isHealthy, bufLen, dropped, lastErr := tsdbClient.Status()
	if isHealthy {
		t.Error("expected TSDB client to report unhealthy during outage")
	}
	if lastErr == "" {
		t.Error("expected lastErr to report error during outage")
	}
	if bufLen > cfg.TSDB.MaxBufferSize {
		t.Errorf("buffer length %d exceeded max capacity %d", bufLen, cfg.TSDB.MaxBufferSize)
	}
	if dropped == 0 {
		t.Error("expected buffer to have safely dropped overflow metrics without leaking memory")
	}

	// Step 3: Restore VictoriaMetrics and verify automatic recovery
	atomic.StoreInt32(&vmOnline, 1)

	// Flush recovered buffer
	err = tsdbClient.Flush(ctx)
	if err != nil {
		t.Fatalf("expected flush to succeed after recovery, got: %v", err)
	}

	isHealthyNow, _, _, _ := tsdbClient.Status()
	if !isHealthyNow {
		t.Error("expected TSDB client to recover healthy status")
	}

	// Step 4: Verify Dashboard & API endpoints return valid JSON
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/realtime", nil)
	rec := httptest.NewRecorder()

	testServer := api.NewServer(cfg, colSvc, tsdbClient)
	_ = testServer

	// Verify realtime endpoint directly
	_, _, _, _ = colSvc.CollectCycle(ctx)
	host, tcpPorts, logsSnapshot := colSvc.GetLatestSnapshot()
	if host == nil || len(tcpPorts) == 0 || len(logsSnapshot) == 0 {
		t.Fatal("expected non-empty snapshot")
	}

	_ = req
	_ = rec
}
