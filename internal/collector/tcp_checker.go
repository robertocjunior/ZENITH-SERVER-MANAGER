package collector

import (
	"net"
	"strconv"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// TCPChecker performs health checks on Protheus TCP ports.
type TCPChecker struct {
	Host    string
	Ports   []config.TCPPortTarget
	Timeout time.Duration
}

// NewTCPChecker creates a new TCP health checker.
func NewTCPChecker(host string, ports []config.TCPPortTarget, timeout time.Duration) *TCPChecker {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &TCPChecker{
		Host:    host,
		Ports:   ports,
		Timeout: timeout,
	}
}

// CheckAll checks all configured TCP ports concurrently and returns their status.
func (c *TCPChecker) CheckAll() []TCPServiceStatus {
	results := make([]TCPServiceStatus, len(c.Ports))
	type checkResult struct {
		index  int
		status TCPServiceStatus
	}

	resChan := make(chan checkResult, len(c.Ports))

	for i, target := range c.Ports {
		go func(idx int, tgt config.TCPPortTarget) {
			status := c.checkPort(tgt)
			resChan <- checkResult{index: idx, status: status}
		}(i, target)
	}

	for i := 0; i < len(c.Ports); i++ {
		res := <-resChan
		results[res.index] = res.status
	}

	return results
}

func (c *TCPChecker) checkPort(target config.TCPPortTarget) TCPServiceStatus {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(target.Port))
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, c.Timeout)
	elapsed := time.Since(start)

	status := TCPServiceStatus{
		Name:        target.Name,
		Port:        target.Port,
		LastChecked: time.Now(),
	}

	if err != nil {
		status.Up = false
		status.LatencyMs = float64(elapsed.Microseconds()) / 1000.0
		status.Error = err.Error()
		return status
	}

	_ = conn.Close()
	status.Up = true
	status.LatencyMs = float64(elapsed.Microseconds()) / 1000.0
	return status
}
