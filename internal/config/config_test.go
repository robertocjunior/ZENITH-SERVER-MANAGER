package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.Server.ListenAddr)
	}
	if len(cfg.Target.TCPPorts) == 0 {
		t.Error("expected default TCP ports")
	}
	if !cfg.Collector.MockMode {
		t.Error("expected default mock mode to be true")
	}
}

func TestLoadWithYAMLAndEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlData := `
server:
  listen_addr: ":9090"
  auth_username: "zenith"
target:
  host: "protheus.local"
  winrm_port: 5986
  winrm_https: true
collector:
  interval: 10s
  mock_mode: false
`
	if err := os.WriteFile(configFile, []byte(yamlData), 0600); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	// Set an environment override
	t.Setenv("ZENITH_TARGET_HOST", "protheus-env.local")
	t.Setenv("ZENITH_AUTH_PASSWORD", "env-password-123")

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.ListenAddr != ":9090" {
		t.Errorf("expected :9090, got %s", cfg.Server.ListenAddr)
	}
	if cfg.Target.Host != "protheus-env.local" {
		t.Errorf("expected env override protheus-env.local, got %s", cfg.Target.Host)
	}
	if cfg.Server.AuthPassword != "env-password-123" {
		t.Errorf("expected env password override, got %s", cfg.Server.AuthPassword)
	}
	if cfg.Collector.Interval != 10*time.Second {
		t.Errorf("expected 10s, got %v", cfg.Collector.Interval)
	}
	if cfg.Collector.MockMode != false {
		t.Errorf("expected mock_mode false, got %v", cfg.Collector.MockMode)
	}
}
