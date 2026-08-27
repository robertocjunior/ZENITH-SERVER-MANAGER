package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the full configuration structure.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Target    TargetConfig    `yaml:"target"`
	TSDB      TSDBConfig      `yaml:"tsdb"`
	Collector CollectorConfig `yaml:"collector"`
}

// ServerConfig configures the HTTP dashboard and API server.
type ServerConfig struct {
	ListenAddr      string        `yaml:"listen_addr"`
	AuthUsername    string        `yaml:"auth_username"`
	AuthPassword    string        `yaml:"auth_password"`
	AuthToken       string        `yaml:"auth_token"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// TargetConfig configures connection to the remote Protheus Windows host.
type TargetConfig struct {
	Host          string          `yaml:"host"`
	WinRMPort     int             `yaml:"winrm_port"`
	WinRMHTTPS    bool            `yaml:"winrm_https"`
	WinRMInsecure bool            `yaml:"winrm_insecure"`
	Username      string          `yaml:"username"`
	Password      string          `yaml:"password"`
	Domain        string          `yaml:"domain"`
	SMBShare      string          `yaml:"smb_share"`     // e.g. "C$"
	AppserverLog  string          `yaml:"appserver_log"` // e.g. "totvs/protheus/bin/appserver/console.log"
	DbaccessLog   string          `yaml:"dbaccess_log"`  // e.g. "totvs/dbaccess/dbaccess.log"
	TCPPorts      []TCPPortTarget `yaml:"tcp_ports"`
}

// TCPPortTarget represents a TCP port check configuration.
type TCPPortTarget struct {
	Name string `yaml:"name"` // e.g. "AppServer", "DBAccess", "License Server"
	Port int    `yaml:"port"` // e.g. 1234, 7890, 5555
}

// TSDBConfig configures connection and buffering for VictoriaMetrics.
type TSDBConfig struct {
	URL             string        `yaml:"url"`                   // e.g. "http://victoriametrics:8428"
	BatchSize       int           `yaml:"batch_size"`            // max metrics before flush (e.g. 500)
	FlushInterval   time.Duration `yaml:"flush_interval"`        // e.g. 2s
	MaxBufferSize   int           `yaml:"max_buffer_size"`       // max metrics queue size in memory (e.g. 50000)
	RequestTimeout  time.Duration `yaml:"request_timeout"`       // e.g. 5s
	MaxRetryBackoff time.Duration `yaml:"max_retry_backoff"`     // e.g. 30s
	InitialBackoff  time.Duration `yaml:"initial_retry_backoff"` // e.g. 1s
}

// CollectorConfig configures agentless telemetry polling intervals and modes.
type CollectorConfig struct {
	Interval       time.Duration `yaml:"interval"`        // e.g. 5s
	Timeout        time.Duration `yaml:"timeout"`         // e.g. 10s
	MockMode       bool          `yaml:"mock_mode"`       // if true, uses high-fidelity emulator
	MonitoredProcs []string      `yaml:"monitored_procs"` // e.g. ["appserver.exe", "dbaccess.exe"]
}

// DefaultConfig returns safe and optimized production defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddr:      ":8080",
			AuthUsername:    "admin",
			AuthPassword:    "zenith@2026",
			AuthToken:       "",
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Target: TargetConfig{
			Host:          "127.0.0.1",
			WinRMPort:     5985,
			WinRMHTTPS:    false,
			WinRMInsecure: true,
			Username:      "Administrator",
			Password:      "",
			Domain:        "",
			SMBShare:      "C$",
			AppserverLog:  "totvs/protheus/bin/appserver/console.log",
			DbaccessLog:   "totvs/dbaccess/dbaccess.log",
			TCPPorts: []TCPPortTarget{
				{Name: "AppServer", Port: 1234},
				{Name: "DBAccess", Port: 7890},
				{Name: "License Server", Port: 5555},
			},
		},
		TSDB: TSDBConfig{
			URL:             "http://127.0.0.1:8428",
			BatchSize:       250,
			FlushInterval:   2 * time.Second,
			MaxBufferSize:   20000,
			RequestTimeout:  5 * time.Second,
			InitialBackoff:  1 * time.Second,
			MaxRetryBackoff: 30 * time.Second,
		},
		Collector: CollectorConfig{
			Interval:       5 * time.Second,
			Timeout:        10 * time.Second,
			MockMode:       true, // Default to mock mode until user configures real target
			MonitoredProcs: []string{"appserver.exe", "dbaccess.exe"},
		},
	}
}

// Load loads configuration from a YAML file, with environment variable overrides.
// It also checks and loads a .env file if present in the current working directory.
func Load(filePath string) (*Config, error) {
	_ = LoadEnvFile(".env")

	cfg := DefaultConfig()

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config yaml: %w", err)
			}
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ZENITH_SERVER_PORT"); v != "" {
		if !strings.HasPrefix(v, ":") {
			cfg.Server.ListenAddr = ":" + v
		} else {
			cfg.Server.ListenAddr = v
		}
	}
	if v := os.Getenv("ZENITH_LISTEN_ADDR"); v != "" {
		cfg.Server.ListenAddr = v
	}
	if v := os.Getenv("ZENITH_AUTH_USERNAME"); v != "" {
		cfg.Server.AuthUsername = v
	}
	if v := os.Getenv("ZENITH_AUTH_PASSWORD"); v != "" {
		cfg.Server.AuthPassword = v
	}
	if v := os.Getenv("ZENITH_AUTH_TOKEN"); v != "" {
		cfg.Server.AuthToken = v
	}

	// Target Overrides
	if v := os.Getenv("ZENITH_TARGET_HOST"); v != "" {
		cfg.Target.Host = v
	}
	if v := os.Getenv("ZENITH_TARGET_WINRM_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Target.WinRMPort = p
		}
	}
	if v := os.Getenv("ZENITH_TARGET_WINRM_HTTPS"); v != "" {
		cfg.Target.WinRMHTTPS = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("ZENITH_TARGET_WINRM_INSECURE"); v != "" {
		cfg.Target.WinRMInsecure = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("ZENITH_TARGET_USERNAME"); v != "" {
		cfg.Target.Username = v
	}
	if v := os.Getenv("ZENITH_TARGET_PASSWORD"); v != "" {
		cfg.Target.Password = v
	}
	if v := os.Getenv("ZENITH_TARGET_DOMAIN"); v != "" {
		cfg.Target.Domain = v
	}
	if v := os.Getenv("ZENITH_TARGET_SMB_SHARE"); v != "" {
		cfg.Target.SMBShare = v
	}
	if v := os.Getenv("ZENITH_TARGET_APPSERVER_LOG"); v != "" {
		cfg.Target.AppserverLog = v
	}
	if v := os.Getenv("ZENITH_TARGET_DBACCESS_LOG"); v != "" {
		cfg.Target.DbaccessLog = v
	}
	if v := os.Getenv("ZENITH_TARGET_TCP_PORTS"); v != "" {
		ports := parseTCPPortsEnv(v)
		if len(ports) > 0 {
			cfg.Target.TCPPorts = ports
		}
	}

	// TSDB Overrides
	if v := os.Getenv("ZENITH_TSDB_URL"); v != "" {
		cfg.TSDB.URL = v
	}

	// Collector Overrides
	if v := os.Getenv("ZENITH_MOCK_MODE"); v != "" {
		cfg.Collector.MockMode = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("ZENITH_COLLECTOR_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Collector.Interval = d
		}
	}
}
