package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/collector"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
	"github.com/robertocjunior/zenith-server-manager/internal/dashboard"
	"github.com/robertocjunior/zenith-server-manager/internal/tsdb"
)

var regexpMetricName = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// Server coordinates the HTTP REST API and Dashboard.
type Server struct {
	cfg        *config.Config
	collector  *collector.Service
	tsdbClient *tsdb.Client
	httpServer *http.Server
}

// NewServer creates a new API and Dashboard HTTP server.
func NewServer(cfg *config.Config, col *collector.Service, tsdb *tsdb.Client) *Server {
	s := &Server{
		cfg:        cfg,
		collector:  col,
		tsdbClient: tsdb,
	}

	mux := http.NewServeMux()

	// Static assets and Dashboard
	staticHandler := http.StripPrefix("/static/", dashboard.Handler())
	mux.Handle("/static/", staticHandler)
	mux.HandleFunc("/", s.handleIndex)

	// API v1 Endpoints
	mux.HandleFunc("/api/v1/healthz", s.handleHealthz)
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/metrics/realtime", s.handleRealtimeMetrics)
	mux.HandleFunc("/api/v1/metrics/history", s.handleHistory)
	mux.HandleFunc("/api/v1/logs", s.handleLogs)

	// Middleware chain
	var handler http.Handler = mux
	handler = AuthMiddleware(cfg.Server, handler)
	handler = SecurityHeadersMiddleware(handler)
	handler = LoggingMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return s
}

// Start runs the HTTP server asynchronously.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	htmlBytes, err := dashboard.IndexHTML()
	if err != nil {
		http.Error(w, "Dashboard index not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(htmlBytes)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	isHealthy, bufLen, dropped, _ := s.tsdbClient.Status()

	resp := map[string]interface{}{
		"status":          "ok",
		"time":            time.Now().UTC().Format(time.RFC3339),
		"mock_mode":       s.collector.IsMock(),
		"tsdb_healthy":    isHealthy,
		"tsdb_buffer_len": bufLen,
		"tsdb_dropped":    dropped,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	isHealthy, bufLen, dropped, lastErr := s.tsdbClient.Status()

	resp := map[string]interface{}{
		"target_host":     s.cfg.Target.Host,
		"winrm_port":      s.cfg.Target.WinRMPort,
		"smb_share":       s.cfg.Target.SMBShare,
		"mock_mode":       s.collector.IsMock(),
		"tsdb_url":        s.cfg.TSDB.URL,
		"tsdb_healthy":    isHealthy,
		"tsdb_buffer_len": bufLen,
		"tsdb_dropped":    dropped,
		"tsdb_last_error": lastErr,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRealtimeMetrics(w http.ResponseWriter, r *http.Request) {
	host, tcps, _ := s.collector.GetLatestSnapshot()

	resp := map[string]interface{}{
		"host":      host,
		"tcp_ports": tcps,
		"is_mock":   s.collector.IsMock(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	_, _, logs := s.collector.GetLatestSnapshot()

	levelFilter := strings.ToUpper(r.URL.Query().Get("level"))
	limitStr := r.URL.Query().Get("limit")

	limit := len(logs)
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l < limit {
			limit = l
		}
	}

	filtered := make([]collector.LogEvent, 0, len(logs))
	for _, l := range logs {
		if levelFilter != "" && levelFilter != "ALL" && l.Level != levelFilter {
			continue
		}
		filtered = append(filtered, l)
	}

	// Return recent first
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		query = "protheus_cpu_percent"
	}

	// Validate query name against command/script injection
	if !regexpMetricName.MatchString(query) {
		http.Error(w, "invalid metric query", http.StatusBadRequest)
		return
	}

	end := time.Now()
	start := end.Add(-1 * time.Hour)
	step := 15 * time.Second

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results, err := s.tsdbClient.QueryRange(ctx, query, start, end, step)
	if err != nil {
		// If VictoriaMetrics query fails (e.g. offline during test), return graceful empty array
		writeJSON(w, http.StatusOK, []tsdb.TimeSeriesResult{})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
