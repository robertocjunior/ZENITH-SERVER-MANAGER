package collector

import (
	"crypto/sha256"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// MockCollector simulates a real Windows TOTVS Protheus server for testing and evaluation.
type MockCollector struct {
	mu           sync.Mutex
	cfg          *config.Config
	step         float64
	sampleLogs   []string
	logCursor    int
	lastMetrics  *HostMetrics
	lastTCP      []TCPServiceStatus
}

// NewMockCollector creates a high-fidelity Protheus mock collector.
func NewMockCollector(cfg *config.Config) *MockCollector {
	sampleLogs := []string{
		"[INFO] THREAD [1042] START: User 'operador01' connected from 192.168.10.45",
		"[INFO] THREAD [1042] FINISH: Routine MATA010 executed in 340 ms",
		"[WARN] Slow query on table SC6010 took 3120 ms (Index: C6_FILIAL+C6_NUM)",
		"[INFO] License Server heartbeat: 38/50 active licenses allocated",
		"[WARN] Memory threshold: appserver.exe working set reached 1.35 GB",
		"[INFO] DBAccess TOPCONNECT: Connection pool healthy (18 active / 32 idle)",
		"[ERROR] LOCK TIMEOUT: Table SB1010 Record #1084 held by thread 1088 for 35s",
		"[INFO] Routine FIN040 executed successfully for company 01 branch 01",
		"[WARN] License Server: 46/50 licenses in use (92% capacity reached)",
		"[INFO] THREAD [1105] START: User 'faturamento' connected",
	}

	return &MockCollector{
		cfg:        cfg,
		sampleLogs: sampleLogs,
		step:       0.0,
	}
}

// CollectMetrics generates realistic varying telemetry for CPU, RAM, Disks, and Processes.
func (m *MockCollector) CollectMetrics() (*HostMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.step += 0.2
	// Sinusoidal variation with random jitter: range ~20% to ~75%
	baseCPU := 35.0 + 25.0*math.Sin(m.step) + (rand.Float64()*8.0 - 4.0)
	if baseCPU < 10.0 {
		baseCPU = 10.0
	} else if baseCPU > 95.0 {
		baseCPU = 95.0
	}

	// 16 GB Total RAM
	totalMem := uint64(16) * 1024 * 1024 * 1024
	usedMemPercent := 55.0 + 15.0*math.Sin(m.step*0.5) + (rand.Float64()*4.0 - 2.0)
	usedMem := uint64(float64(totalMem) * (usedMemPercent / 100.0))
	freeMem := totalMem - usedMem

	// Disks
	cTotal := uint64(250) * 1024 * 1024 * 1024 // 250GB
	cUsed := uint64(140) * 1024 * 1024 * 1024  // 140GB
	cPct := (float64(cUsed) / float64(cTotal)) * 100.0

	dTotal := uint64(1000) * 1024 * 1024 * 1024 // 1TB
	dUsed := uint64(620) * 1024 * 1024 * 1024   // 620GB
	dPct := (float64(dUsed) / float64(dTotal)) * 100.0

	disks := []DiskMetric{
		{Device: "C:", TotalBytes: cTotal, UsedBytes: cUsed, FreeBytes: cTotal - cUsed, Percent: cPct},
		{Device: "D:", TotalBytes: dTotal, UsedBytes: dUsed, FreeBytes: dTotal - dUsed, Percent: dPct},
	}

	// Processes
	appserverWS := uint64(1200*1024*1024) + uint64(rand.Intn(150*1024*1024))
	appserverCPU := baseCPU * 0.65
	threadsApp := 45 + rand.Intn(15)

	dbaccessWS := uint64(580*1024*1024) + uint64(rand.Intn(60*1024*1024))
	dbaccessCPU := baseCPU * 0.25
	threadsDB := 22 + rand.Intn(8)

	procs := []ProcessMetric{
		{
			Name:            "appserver.exe",
			PID:             4210,
			CPUPercent:      appserverCPU,
			WorkingSetBytes: appserverWS,
			VirtualBytes:    appserverWS * 2,
			ThreadCount:     threadsApp,
			HandleCount:     1450 + rand.Intn(100),
			Status:          "RUNNING",
		},
		{
			Name:            "dbaccess.exe",
			PID:             5180,
			CPUPercent:      dbaccessCPU,
			WorkingSetBytes: dbaccessWS,
			VirtualBytes:    dbaccessWS * 2,
			ThreadCount:     threadsDB,
			HandleCount:     850 + rand.Intn(50),
			Status:          "RUNNING",
		},
	}

	metrics := &HostMetrics{
		Timestamp:        time.Now(),
		Host:             m.cfg.Target.Host + " [MOCK]",
		CPUPercent:       baseCPU,
		MemoryTotalBytes: totalMem,
		MemoryUsedBytes:  usedMem,
		MemoryFreeBytes:  freeMem,
		MemoryPercent:    usedMemPercent,
		Disks:            disks,
		Processes:        procs,
	}

	m.lastMetrics = metrics
	return metrics, nil
}

// CheckTCPPorts generates status for all configured Protheus ports.
func (m *MockCollector) CheckTCPPorts() ([]TCPServiceStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var statuses []TCPServiceStatus
	for _, port := range m.cfg.Target.TCPPorts {
		latency := 0.8 + rand.Float64()*2.5 // ~0.8 to 3.3 ms
		statuses = append(statuses, TCPServiceStatus{
			Name:        port.Name,
			Port:        port.Port,
			Up:          true,
			LatencyMs:   latency,
			LastChecked: time.Now(),
		})
	}

	m.lastTCP = statuses
	return statuses, nil
}

// ReadNewLogs returns simulated realistic TOTVS log lines.
func (m *MockCollector) ReadNewLogs() ([]LogEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	numLogs := 1 + rand.Intn(2)
	var events []LogEvent

	for i := 0; i < numLogs; i++ {
		line := m.sampleLogs[m.logCursor%len(m.sampleLogs)]
		m.logCursor++

		source := "appserver"
		if rand.Float64() < 0.3 {
			source = "dbaccess"
		}

		event := ParseLogLine(source, line)
		if event != nil {
			// Ensure unique ID with timestamp
			h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", line, time.Now().UnixNano(), rand.Int())))
			event.ID = fmt.Sprintf("%x", h[:8])
			events = append(events, *event)
		}
	}

	return events, nil
}

// Close cleans up mock resources.
func (m *MockCollector) Close() error {
	return nil
}
