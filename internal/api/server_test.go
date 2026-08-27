package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/robertocjunior/zenith-server-manager/internal/collector"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
	"github.com/robertocjunior/zenith-server-manager/internal/tsdb"
)

func setupTestServer(t *testing.T) (*Server, *config.Config) {
	cfg := config.DefaultConfig()
	cfg.Collector.MockMode = true
	cfg.Server.AuthPassword = "" // disable auth for basic tests
	cfg.Server.AuthToken = ""

	col, err := collector.NewService(cfg)
	if err != nil {
		t.Fatalf("failed creating collector: %v", err)
	}

	// Run 1 mock collection cycle to populate data
	_, _, _, _ = col.CollectCycle(context.Background())

	tsdbClient := tsdb.NewClient(cfg.TSDB)
	server := NewServer(cfg, col, tsdbClient)

	return server, cfg
}

func TestHealthzEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding healthz JSON: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
	if resp["mock_mode"] != true {
		t.Errorf("expected mock_mode true, got %v", resp["mock_mode"])
	}
}

func TestRealtimeMetricsEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/realtime", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp struct {
		Host     *collector.HostMetrics        `json:"host"`
		TCPPorts []collector.TCPServiceStatus  `json:"tcp_ports"`
		IsMock   bool                          `json:"is_mock"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding realtime metrics: %v", err)
	}

	if resp.Host == nil {
		t.Fatal("expected non-nil host metrics")
	}
	if len(resp.TCPPorts) == 0 {
		t.Error("expected tcp ports list")
	}
}

func TestSecurityHeaders(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	headers := rec.Header()
	if headers.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing or invalid X-Content-Type-Options: %s", headers.Get("X-Content-Type-Options"))
	}
	if headers.Get("X-Frame-Options") != "DENY" {
		t.Errorf("missing or invalid X-Frame-Options: %s", headers.Get("X-Frame-Options"))
	}
	if headers.Get("Content-Security-Policy") == "" {
		t.Error("missing Content-Security-Policy header")
	}
}

func TestAuthProtection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collector.MockMode = true
	cfg.Server.AuthUsername = "admin"
	cfg.Server.AuthPassword = "secret-password"

	col, _ := collector.NewService(cfg)
	tsdbClient := tsdb.NewClient(cfg.TSDB)
	server := NewServer(cfg, col, tsdbClient)

	// 1. Unauthenticated request to /api/v1/status must fail with 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	recUnauth := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recUnauth, reqUnauth)

	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", recUnauth.Code)
	}

	// 2. Healthz should still succeed without auth
	reqHealthz := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	recHealthz := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recHealthz, reqHealthz)
	if recHealthz.Code != http.StatusOK {
		t.Fatalf("expected 200 for healthz without auth, got %d", recHealthz.Code)
	}

	// 3. Authenticated request with Basic Auth must succeed with 200
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	reqAuth.SetBasicAuth("admin", "secret-password")
	recAuth := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid credentials, got %d", recAuth.Code)
	}
}

func TestDashboardHTMLServed(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content-type, got %s", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !testing.Short() && len(body) < 100 {
		t.Errorf("expected dashboard html body, got length %d", len(body))
	}
}
