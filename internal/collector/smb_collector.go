package collector

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/hirochachacha/go-smb2"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// SMBLogCollector connects to remote Windows SMB shares to tail and parse Protheus logs.
type SMBLogCollector struct {
	mu           sync.Mutex
	cfg          config.TargetConfig
	colCfg       config.CollectorConfig
	session      *smb2.Session
	share        *smb2.Share
	offsets      map[string]int64
	retryCfg     RetryConfig
	closed       bool
}

// NewSMBLogCollector creates a new SMB log collector.
func NewSMBLogCollector(targetCfg config.TargetConfig, colCfg config.CollectorConfig) *SMBLogCollector {
	return &SMBLogCollector{
		cfg:      targetCfg,
		colCfg:   colCfg,
		offsets:  make(map[string]int64),
		retryCfg: DefaultRetryConfig(),
	}
}

// connect establishes an SMB2 session with retry.
func (s *SMBLogCollector) connect(ctx context.Context) error {
	if s.share != nil {
		return nil
	}

	sanitizedShare, err := SanitizePath(s.cfg.SMBShare)
	if err != nil {
		return fmt.Errorf("invalid SMB share name: %w", err)
	}

	addr := fmt.Sprintf("%s:445", s.cfg.Host)

	return DoWithRetry(ctx, s.retryCfg, func(c context.Context, attempt int) error {
		dialer := net.Dialer{Timeout: s.colCfg.Timeout}
		conn, err := dialer.DialContext(c, "tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to dial SMB port 445 on %s: %w", addr, err)
		}

		initiator := &smb2.NTLMInitiator{
			User:     s.cfg.Username,
			Password: s.cfg.Password,
			Domain:   s.cfg.Domain,
		}

		d := &smb2.Dialer{
			Initiator: initiator,
		}

		session, err := d.Dial(conn)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("failed SMB authentication: %w", err)
		}

		share, err := session.Mount(sanitizedShare)
		if err != nil {
			_ = session.Logoff()
			_ = conn.Close()
			return fmt.Errorf("failed mounting SMB share %s: %w", sanitizedShare, err)
		}

		s.session = session
		s.share = share
		return nil
	})
}

// ReadNewLogs tails both appserver and dbaccess log files remotely via SMB.
func (s *SMBLogCollector) ReadNewLogs(ctx context.Context) ([]LogEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, nil
	}

	if err := s.connect(ctx); err != nil {
		return nil, fmt.Errorf("SMB connection error: %w", err)
	}

	var allEvents []LogEvent

	// Tail AppServer log
	if s.cfg.AppserverLog != "" {
		events, err := s.tailFile("appserver", s.cfg.AppserverLog)
		if err != nil {
			s.resetConnection()
			return nil, fmt.Errorf("error reading appserver log: %w", err)
		}
		allEvents = append(allEvents, events...)
	}

	// Tail DBAccess log
	if s.cfg.DbaccessLog != "" {
		events, err := s.tailFile("dbaccess", s.cfg.DbaccessLog)
		if err != nil {
			s.resetConnection()
			return nil, fmt.Errorf("error reading dbaccess log: %w", err)
		}
		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}

func (s *SMBLogCollector) tailFile(source, rawPath string) ([]LogEvent, error) {
	safePath, err := SanitizePath(rawPath)
	if err != nil {
		return nil, err
	}

	file, err := s.share.Open(safePath)
	if err != nil {
		return nil, fmt.Errorf("failed opening remote log file %s: %w", safePath, err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed stat on %s: %w", safePath, err)
	}

	fileSize := fi.Size()
	lastOffset := s.offsets[safePath]

	// Handle log rotation or first read
	if lastOffset == 0 {
		// First read: start reading from last 64KB to avoid loading gigabytes into memory
		if fileSize > 65536 {
			lastOffset = fileSize - 65536
		} else {
			lastOffset = 0
		}
	} else if fileSize < lastOffset {
		// File truncated/rotated
		lastOffset = 0
	}

	if fileSize == lastOffset {
		// No new data
		return nil, nil
	}

	bytesToRead := fileSize - lastOffset
	// Cap max read per poll to 5MB to avoid memory pressure
	if bytesToRead > 5*1024*1024 {
		bytesToRead = 5 * 1024 * 1024
		lastOffset = fileSize - bytesToRead
	}

	buf := make([]byte, bytesToRead)
	_, err = file.ReadAt(buf, lastOffset)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed reading %s at offset %d: %w", safePath, lastOffset, err)
	}

	s.offsets[safePath] = fileSize

	var events []LogEvent
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := scanner.Text()
		event := ParseLogLine(source, line)
		if event != nil {
			events = append(events, *event)
		}
	}

	return events, nil
}

func (s *SMBLogCollector) resetConnection() {
	if s.share != nil {
		_ = s.share.Umount()
		s.share = nil
	}
	if s.session != nil {
		_ = s.session.Logoff()
		s.session = nil
	}
}

// Close closes the SMB session cleanly.
func (s *SMBLogCollector) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.resetConnection()
	return nil
}
