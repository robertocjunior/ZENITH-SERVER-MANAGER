package collector

import (
	"time"
)

// HostMetrics represents system-level telemetry collected from Windows.
type HostMetrics struct {
	Timestamp        time.Time       `json:"timestamp"`
	Host             string          `json:"host"`
	CPUPercent       float64         `json:"cpu_percent"`
	MemoryTotalBytes uint64          `json:"memory_total_bytes"`
	MemoryUsedBytes  uint64          `json:"memory_used_bytes"`
	MemoryFreeBytes  uint64          `json:"memory_free_bytes"`
	MemoryPercent    float64         `json:"memory_percent"`
	Disks            []DiskMetric    `json:"disks"`
	Processes        []ProcessMetric `json:"processes"`
}

// DiskMetric represents logical disk storage telemetry.
type DiskMetric struct {
	Device     string  `json:"device"` // e.g. "C:"
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	Percent    float64 `json:"percent"`
}

// ProcessMetric represents Windows process metrics for TOTVS Protheus services.
type ProcessMetric struct {
	Name            string  `json:"name"` // "appserver.exe", "dbaccess.exe"
	PID             int     `json:"pid"`
	CPUPercent      float64 `json:"cpu_percent"`
	WorkingSetBytes uint64  `json:"working_set_bytes"`
	VirtualBytes    uint64  `json:"virtual_bytes"`
	ThreadCount     int     `json:"thread_count"`
	HandleCount     int     `json:"handle_count"`
	Status          string  `json:"status"` // "RUNNING", "NOT_FOUND", "HIGH_MEMORY"
}

// TCPServiceStatus represents the availability and latency of a monitored port.
type TCPServiceStatus struct {
	Name        string    `json:"name"`
	Port        int       `json:"port"`
	Up          bool      `json:"up"`
	LatencyMs   float64   `json:"latency_ms"`
	LastChecked time.Time `json:"last_checked"`
	Error       string    `json:"error,omitempty"`
}

// LogEvent represents an alert or noteworthy event parsed from Protheus logs.
type LogEvent struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"` // "appserver", "dbaccess"
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`    // "INFO", "WARN", "ERROR", "CRITICAL"
	Category  string    `json:"category"` // "THREAD", "DATABASE", "LICENSE", "LOCK", "SYSTEM", "SLOW_QUERY"
	Message   string    `json:"message"`
	RawLine   string    `json:"raw_line"`
}

// Collector defines the interface for collecting Protheus telemetry.
type Collector interface {
	CollectMetrics() (*HostMetrics, error)
	CheckTCPPorts() ([]TCPServiceStatus, error)
	ReadNewLogs() ([]LogEvent, error)
	Close() error
}
