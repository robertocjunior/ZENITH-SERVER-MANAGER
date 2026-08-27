package tsdb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/collector"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

func TestToPrometheusLine(t *testing.T) {
	pt := MetricPoint{
		Name:      "protheus_cpu_percent",
		Labels:    map[string]string{"host": "srv01", "environment": "prod"},
		Value:     42.5,
		Timestamp: time.UnixMilli(1724750000000),
	}

	line := pt.ToPrometheusLine()
	expected := `protheus_cpu_percent{environment="prod",host="srv01"} 42.5 1724750000000`
	if line != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, line)
	}
}

func TestVictoriaMetricsFlushSuccess(t *testing.T) {
	var receivedBody string
	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/import/prometheus" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			receivedBody = string(body)
			atomic.AddInt32(&reqCount, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.TSDBConfig{
		URL:            server.URL,
		BatchSize:      10,
		FlushInterval:  100 * time.Millisecond,
		MaxBufferSize:  100,
		RequestTimeout: 1 * time.Second,
	}

	client := NewClient(cfg)
	client.buffer.Push(MetricPoint{
		Name:      "test_metric",
		Labels:    map[string]string{"host": "localhost"},
		Value:     100.0,
		Timestamp: time.Now(),
	})

	err := client.Flush(context.Background())
	if err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if atomic.LoadInt32(&reqCount) != 1 {
		t.Errorf("expected 1 request to VictoriaMetrics, got %d", reqCount)
	}
	if !strings.Contains(receivedBody, "test_metric") {
		t.Errorf("expected body to contain test_metric, got %s", receivedBody)
	}
}

func TestVictoriaMetricsOutageNoMemoryLeak(t *testing.T) {
	// Server returning 500 error simulating broken TSDB
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal tsdb error", http.StatusInternalServerError)
	}))
	defer server.Close()

	bufferCap := 50
	cfg := config.TSDBConfig{
		URL:            server.URL,
		BatchSize:      20,
		FlushInterval:  50 * time.Millisecond,
		MaxBufferSize:  bufferCap,
		RequestTimeout: 200 * time.Millisecond,
	}

	client := NewClient(cfg)

	// Simulate high load while TSDB is down: push 500 metrics into buffer of 50
	for i := 0; i < 500; i++ {
		client.EnqueueHostMetrics(&collector.HostMetrics{
			Timestamp:        time.Now(),
			Host:             "test-host",
			CPUPercent:       float64(i % 100),
			MemoryTotalBytes: 16000000000,
			MemoryUsedBytes:  8000000000,
		})
	}

	// Attempt flush - will fail because server returns 500
	_ = client.Flush(context.Background())

	isHealthy, bufLen, dropped, lastErr := client.Status()
	if isHealthy {
		t.Error("expected isHealthy to be false during TSDB outage")
	}
	if lastErr == "" {
		t.Error("expected lastErr to report failure")
	}
	if bufLen > bufferCap {
		t.Errorf("buffer exceeded max capacity: %d > %d", bufLen, bufferCap)
	}
	if dropped == 0 {
		t.Error("expected dropped metrics count to be > 0 during overflow")
	}
}
