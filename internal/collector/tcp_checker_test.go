package collector

import (
	"net"
	"testing"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

func TestTCPChecker(t *testing.T) {
	// Start a local dummy TCP listener to simulate an open port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	// Channel to accept one connection
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	ports := []config.TCPPortTarget{
		{Name: "OpenService", Port: port},
		{Name: "ClosedService", Port: 64999}, // Should fail
	}

	checker := NewTCPChecker("127.0.0.1", ports, 1*time.Second)
	results := checker.CheckAll()

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if !results[0].Up {
		t.Errorf("expected OpenService to be Up, got error: %s", results[0].Error)
	}
	if results[0].LatencyMs < 0 {
		t.Errorf("expected valid latency, got %f", results[0].LatencyMs)
	}

	if results[1].Up {
		t.Errorf("expected ClosedService to be Down")
	}
	if results[1].Error == "" {
		t.Errorf("expected error message for closed service")
	}
}
