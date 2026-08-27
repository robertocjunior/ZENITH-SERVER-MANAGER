package collector

import (
	"context"
	"fmt"
	"sync"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// Service coordinates all collection routines (OS metrics, TCP ports, SMB logs).
type Service struct {
	mu           sync.RWMutex
	cfg          *config.Config
	mockMode     bool
	winrm        *WinRMCollector
	smb          *SMBLogCollector
	tcpChecker   *TCPChecker
	mock         *MockCollector
	latestHost   *HostMetrics
	latestTCP    []TCPServiceStatus
	recentLogs   []LogEvent
	maxRecentLogs int
	closed       bool
}

// NewService creates a collection service configured for either real or mock mode.
func NewService(cfg *config.Config) (*Service, error) {
	s := &Service{
		cfg:           cfg,
		mockMode:      cfg.Collector.MockMode,
		maxRecentLogs: 200,
		recentLogs:    make([]LogEvent, 0, 200),
	}

	if s.mockMode {
		s.mock = NewMockCollector(cfg)
		return s, nil
	}

	// Real mode initialization
	winrmCol, err := NewWinRMCollector(cfg.Target, cfg.Collector)
	if err != nil {
		return nil, fmt.Errorf("failed creating WinRM collector: %w", err)
	}
	s.winrm = winrmCol

	s.smb = NewSMBLogCollector(cfg.Target, cfg.Collector)
	s.tcpChecker = NewTCPChecker(cfg.Target.Host, cfg.Target.TCPPorts, cfg.Collector.Timeout)

	return s, nil
}

// CollectCycle executes one full collection cycle and stores the latest snapshot in memory.
func (s *Service) CollectCycle(ctx context.Context) (*HostMetrics, []TCPServiceStatus, []LogEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, nil, nil, fmt.Errorf("collector service is closed")
	}

	if s.mockMode {
		metrics, err := s.mock.CollectMetrics()
		if err != nil {
			return nil, nil, nil, err
		}
		tcps, _ := s.mock.CheckTCPPorts()
		logs, _ := s.mock.ReadNewLogs()

		s.latestHost = metrics
		s.latestTCP = tcps
		s.appendLogs(logs)

		return metrics, tcps, logs, nil
	}

	// Real collection
	var metrics *HostMetrics
	var errMetrics error

	// Run WinRM collection
	if s.winrm != nil {
		metrics, errMetrics = s.winrm.CollectMetrics(ctx)
		if errMetrics == nil {
			s.latestHost = metrics
		}
	}

	// Run TCP port checks
	var tcps []TCPServiceStatus
	if s.tcpChecker != nil {
		tcps = s.tcpChecker.CheckAll()
		s.latestTCP = tcps
	}

	// Run SMB log tailing
	var newLogs []LogEvent
	if s.smb != nil {
		logs, err := s.smb.ReadNewLogs(ctx)
		if err == nil && len(logs) > 0 {
			newLogs = logs
			s.appendLogs(logs)
		}
	}

	return metrics, tcps, newLogs, errMetrics
}

func (s *Service) appendLogs(newLogs []LogEvent) {
	for _, l := range newLogs {
		s.recentLogs = append(s.recentLogs, l)
		if len(s.recentLogs) > s.maxRecentLogs {
			s.recentLogs = s.recentLogs[len(s.recentLogs)-s.maxRecentLogs:]
		}
	}
}

// GetLatestSnapshot returns the cached latest telemetry state.
func (s *Service) GetLatestSnapshot() (*HostMetrics, []TCPServiceStatus, []LogEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logsCopy := make([]LogEvent, len(s.recentLogs))
	copy(logsCopy, s.recentLogs)

	tcpCopy := make([]TCPServiceStatus, len(s.latestTCP))
	copy(tcpCopy, s.latestTCP)

	return s.latestHost, tcpCopy, logsCopy
}

// IsMock returns whether mock mode is currently enabled.
func (s *Service) IsMock() bool {
	return s.mockMode
}

// Close gracefully closes collector resources.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.smb != nil {
		_ = s.smb.Close()
	}
	if s.mock != nil {
		_ = s.mock.Close()
	}
	return nil
}
