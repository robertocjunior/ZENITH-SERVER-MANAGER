package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/api"
	"github.com/robertocjunior/zenith-server-manager/internal/collector"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
	"github.com/robertocjunior/zenith-server-manager/internal/tsdb"
)

var (
	Version   = "1.0.0"
	GitCommit = "dev"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to Zenith configuration YAML file")
	flag.Parse()

	log.Println("================================================================")
	log.Printf(" ⚡ ZENITH SERVER MANAGER — TOTVS Protheus Agentless Monitor %s", Version)
	log.Printf("    Commit: %s | High Performance Agentless Telemetry", GitCommit)
	log.Println("================================================================")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to load configuration: %v", err)
	}

	modeStr := "AGENTLESS LIVE"
	if cfg.Collector.MockMode {
		modeStr = "MOCK / EMULATOR (offline validation)"
	}
	log.Printf("[INIT] Mode: %s", modeStr)
	log.Printf("[INIT] Target Windows Host: %s (WinRM:%d, SMB:%s)", cfg.Target.Host, cfg.Target.WinRMPort, cfg.Target.SMBShare)
	log.Printf("[INIT] TSDB VictoriaMetrics URL: %s (BatchSize:%d, Flush:%v)", cfg.TSDB.URL, cfg.TSDB.BatchSize, cfg.TSDB.FlushInterval)
	log.Printf("[INIT] Dashboard Server listening on: %s", cfg.Server.ListenAddr)

	// Initialize Collector
	collectorSvc, err := collector.NewService(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize collector service: %v", err)
	}
	defer collectorSvc.Close()

	// Initialize VictoriaMetrics Client
	tsdbClient := tsdb.NewClient(cfg.TSDB)
	tsdbClient.Start()
	defer tsdbClient.Stop()

	// Context for graceful shutdown of background routines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background collection loop
	go runCollectionLoop(ctx, cfg, collectorSvc, tsdbClient)

	// Initialize and start HTTP API & Dashboard Server
	server := api.NewServer(cfg, collectorSvc, tsdbClient)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	log.Printf("[READY] Zenith Server Manager is running. Open http://localhost%s in your browser", cfg.Server.ListenAddr)

	// Graceful shutdown on SIGINT / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[SHUTDOWN] Received signal %v. Initiating graceful shutdown...", sig)

	// Cancel background collection
	cancel()

	// Shutdown HTTP Server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[WARN] HTTP server shutdown error: %v", err)
	}

	log.Println("[SHUTDOWN] Zenith Server Manager stopped cleanly.")
}

func runCollectionLoop(ctx context.Context, cfg *config.Config, col *collector.Service, tsdbClient *tsdb.Client) {
	ticker := time.NewTicker(cfg.Collector.Interval)
	defer ticker.Stop()

	// Trigger initial collection immediately
	collectOnce(ctx, col, tsdbClient)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectOnce(ctx, col, tsdbClient)
		}
	}
}

func collectOnce(ctx context.Context, col *collector.Service, tsdbClient *tsdb.Client) {
	collectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	metrics, tcps, logs, err := col.CollectCycle(collectCtx)
	if err != nil {
		log.Printf("[WARN] Collection cycle warning: %v", err)
	}

	if metrics != nil {
		tsdbClient.EnqueueHostMetrics(metrics)
	}
	if len(tcps) > 0 {
		tsdbClient.EnqueueTCPMetrics(tcps)
	}

	if len(logs) > 0 {
		for _, l := range logs {
			if l.Level == "CRITICAL" || l.Level == "ERROR" {
				log.Printf("[%s] [%s] %s", l.Level, l.Category, l.Message)
			}
		}
	}
}
