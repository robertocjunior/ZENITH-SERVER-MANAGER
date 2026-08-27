package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// WinRMCollector connects to a Windows host via WinRM to gather OS and process metrics.
type WinRMCollector struct {
	cfg         config.TargetConfig
	collectorCfg config.CollectorConfig
	retryCfg    RetryConfig
	client      *winrm.Client
}

// NewWinRMCollector creates a new WinRM collector instance.
func NewWinRMCollector(targetCfg config.TargetConfig, colCfg config.CollectorConfig) (*WinRMCollector, error) {
	username := targetCfg.Username
	if targetCfg.Domain != "" {
		username = fmt.Sprintf("%s\\%s", targetCfg.Domain, targetCfg.Username)
	}

	endpoint := winrm.NewEndpoint(
		targetCfg.Host,
		targetCfg.WinRMPort,
		targetCfg.WinRMHTTPS,
		targetCfg.WinRMInsecure,
		nil, nil, nil,
		colCfg.Timeout,
	)

	client, err := winrm.NewClient(endpoint, username, targetCfg.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create WinRM client: %w", err)
	}

	return &WinRMCollector{
		cfg:          targetCfg,
		collectorCfg: colCfg,
		retryCfg:     DefaultRetryConfig(),
		client:       client,
	}, nil
}

// ExecutePowerShell runs a sanitized PowerShell command over WinRM with retry and timeout.
func (w *WinRMCollector) ExecutePowerShell(ctx context.Context, script string) (string, error) {
	var stdout, stderr bytes.Buffer

	// Wrap PowerShell execution safely
	encodedScript := fmt.Sprintf("powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command \"%s\"", strings.ReplaceAll(script, "\"", "\\\""))

	err := DoWithRetry(ctx, w.retryCfg, func(c context.Context, attempt int) error {
		stdout.Reset()
		stderr.Reset()
		_, runErr := w.client.RunWithContext(c, encodedScript, &stdout, &stderr)
		if runErr != nil {
			return fmt.Errorf("WinRM attempt %d failed: %w (stderr: %s)", attempt, runErr, stderr.String())
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CollectMetrics retrieves CPU, RAM, disk, and process telemetry from the remote Windows server.
func (w *WinRMCollector) CollectMetrics(ctx context.Context) (*HostMetrics, error) {
	// Sanitize process names to monitor
	var safeProcs []string
	for _, proc := range w.collectorCfg.MonitoredProcs {
		safeName, err := SanitizeProcessName(proc)
		if err != nil {
			continue
		}
		// Strip .exe if present for Get-Process
		baseName := strings.TrimSuffix(safeName, ".exe")
		safeProcs = append(safeProcs, fmt.Sprintf("'%s'", EscapePowerShellString(baseName)))
	}

	procsFilter := strings.Join(safeProcs, ",")
	if procsFilter == "" {
		procsFilter = "'appserver','dbaccess'"
	}

	// Single consolidated PowerShell script returning JSON to minimize network roundtrips
	psScript := fmt.Sprintf(`
$res = @{}
$cpu = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average
$res['cpu'] = [double]$cpu

$os = Get-CimInstance Win32_OperatingSystem
$res['mem_total_kb'] = [double]$os.TotalVisibleMemorySize
$res['mem_free_kb'] = [double]$os.FreePhysicalMemory

$disks = @()
Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | ForEach-Object {
    $disks += @{
        device = $_.DeviceID
        size = [double]$_.Size
        free = [double]$_.FreeSpace
    }
}
$res['disks'] = $disks

$procs = @()
Get-Process -Name %s -ErrorAction SilentlyContinue | ForEach-Object {
    $procs += @{
        name = $_.ProcessName + '.exe'
        pid = $_.Id
        ws = [double]$_.WorkingSet64
        vm = [double]$_.VirtualMemorySize64
        cpu = [double]$_.CPU
        threads = $_.Threads.Count
        handles = $_.Handles
    }
}
$res['processes'] = $procs

$res | ConvertTo-Json -Compress
`, procsFilter)

	out, err := w.ExecutePowerShell(ctx, psScript)
	if err != nil {
		return nil, fmt.Errorf("failed collecting Windows telemetry via WinRM: %w", err)
	}

	return parseWinRMJSONOutput(out, w.cfg.Host)
}

type winrmTelemetryJSON struct {
	CPU        float64 `json:"cpu"`
	MemTotalKB float64 `json:"mem_total_kb"`
	MemFreeKB  float64 `json:"mem_free_kb"`
	Disks      []struct {
		Device string  `json:"device"`
		Size   float64 `json:"size"`
		Free   float64 `json:"free"`
	} `json:"disks"`
	Processes []struct {
		Name    string  `json:"name"`
		PID     int     `json:"pid"`
		WS      float64 `json:"ws"`
		VM      float64 `json:"vm"`
		CPU     float64 `json:"cpu"`
		Threads int     `json:"threads"`
		Handles int     `json:"handles"`
	} `json:"processes"`
}

func parseWinRMJSONOutput(jsonStr, host string) (*HostMetrics, error) {
	// Find beginning of JSON if output has PowerShell noise
	idx := strings.Index(jsonStr, "{")
	if idx >= 0 {
		jsonStr = jsonStr[idx:]
	}

	var data winrmTelemetryJSON
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to parse WinRM telemetry JSON: %w (output: %s)", err, jsonStr)
	}

	totalBytes := uint64(data.MemTotalKB * 1024)
	freeBytes := uint64(data.MemFreeKB * 1024)
	usedBytes := uint64(0)
	if totalBytes > freeBytes {
		usedBytes = totalBytes - freeBytes
	}
	memPercent := 0.0
	if totalBytes > 0 {
		memPercent = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}

	disks := make([]DiskMetric, 0, len(data.Disks))
	for _, d := range data.Disks {
		dTotal := uint64(d.Size)
		dFree := uint64(d.Free)
		dUsed := uint64(0)
		if dTotal > dFree {
			dUsed = dTotal - dFree
		}
		pct := 0.0
		if dTotal > 0 {
			pct = (float64(dUsed) / float64(dTotal)) * 100.0
		}
		disks = append(disks, DiskMetric{
			Device:     d.Device,
			TotalBytes: dTotal,
			UsedBytes:  dUsed,
			FreeBytes:  dFree,
			Percent:    pct,
		})
	}

	procs := make([]ProcessMetric, 0, len(data.Processes))
	for _, p := range data.Processes {
		procs = append(procs, ProcessMetric{
			Name:            p.Name,
			PID:             p.PID,
			CPUPercent:      p.CPU,
			WorkingSetBytes: uint64(p.WS),
			VirtualBytes:    uint64(p.VM),
			ThreadCount:     p.Threads,
			HandleCount:     p.Handles,
			Status:          "RUNNING",
		})
	}

	return &HostMetrics{
		Timestamp:        time.Now(),
		Host:             host,
		CPUPercent:       data.CPU,
		MemoryTotalBytes: totalBytes,
		MemoryUsedBytes:  usedBytes,
		MemoryFreeBytes:  freeBytes,
		MemoryPercent:    memPercent,
		Disks:            disks,
		Processes:        procs,
	}, nil
}
