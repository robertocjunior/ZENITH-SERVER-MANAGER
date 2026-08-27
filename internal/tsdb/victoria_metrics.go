package tsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robertocjunior/zenith-server-manager/internal/collector"
	"github.com/robertocjunior/zenith-server-manager/internal/config"
)

// Client handles ingestion and querying against VictoriaMetrics TSDB.
type Client struct {
	mu           sync.RWMutex
	cfg          config.TSDBConfig
	buffer       *BoundedBuffer
	httpClient   *http.Client
	importURL    string
	queryURL     string
	queryRangeURL string
	healthURL    string
	stopChan     chan struct{}
	wg           sync.WaitGroup
	isHealthy    bool
	lastError    string
	consecutiveFailures int
}

// NewClient creates and initializes a VictoriaMetrics client with bounded memory buffering.
func NewClient(cfg config.TSDBConfig) *Client {
	baseURL := strings.TrimRight(cfg.URL, "/")

	client := &Client{
		cfg:           cfg,
		buffer:        NewBoundedBuffer(cfg.MaxBufferSize),
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		importURL:     baseURL + "/api/v1/import/prometheus",
		queryURL:      baseURL + "/api/v1/query",
		queryRangeURL: baseURL + "/api/v1/query_range",
		healthURL:     baseURL + "/health",
		stopChan:      make(chan struct{}),
	}

	return client
}

// Start launches the background flusher goroutine.
func (c *Client) Start() {
	c.wg.Add(1)
	go c.flushLoop()
}

// Stop stops the background flusher and flushes remaining points if possible.
func (c *Client) Stop() {
	close(c.stopChan)
	c.wg.Wait()
	_ = c.Flush(context.Background())
}

// EnqueueHostMetrics extracts and buffers all time-series points from HostMetrics.
func (c *Client) EnqueueHostMetrics(m *collector.HostMetrics) {
	if m == nil {
		return
	}

	hostLabel := map[string]string{"host": m.Host}

	var points []MetricPoint
	points = append(points,
		MetricPoint{Name: "protheus_cpu_percent", Labels: hostLabel, Value: m.CPUPercent, Timestamp: m.Timestamp},
		MetricPoint{Name: "protheus_memory_total_bytes", Labels: hostLabel, Value: float64(m.MemoryTotalBytes), Timestamp: m.Timestamp},
		MetricPoint{Name: "protheus_memory_used_bytes", Labels: hostLabel, Value: float64(m.MemoryUsedBytes), Timestamp: m.Timestamp},
		MetricPoint{Name: "protheus_memory_free_bytes", Labels: hostLabel, Value: float64(m.MemoryFreeBytes), Timestamp: m.Timestamp},
		MetricPoint{Name: "protheus_memory_percent", Labels: hostLabel, Value: m.MemoryPercent, Timestamp: m.Timestamp},
	)

	// Disks
	for _, d := range m.Disks {
		labels := map[string]string{"host": m.Host, "device": d.Device}
		points = append(points,
			MetricPoint{Name: "protheus_disk_total_bytes", Labels: labels, Value: float64(d.TotalBytes), Timestamp: m.Timestamp},
			MetricPoint{Name: "protheus_disk_used_bytes", Labels: labels, Value: float64(d.UsedBytes), Timestamp: m.Timestamp},
			MetricPoint{Name: "protheus_disk_percent", Labels: labels, Value: d.Percent, Timestamp: m.Timestamp},
		)
	}

	// Processes
	for _, p := range m.Processes {
		labels := map[string]string{"host": m.Host, "process": p.Name, "pid": strconv.Itoa(p.PID)}
		points = append(points,
			MetricPoint{Name: "protheus_process_cpu_percent", Labels: labels, Value: p.CPUPercent, Timestamp: m.Timestamp},
			MetricPoint{Name: "protheus_process_working_set_bytes", Labels: labels, Value: float64(p.WorkingSetBytes), Timestamp: m.Timestamp},
			MetricPoint{Name: "protheus_process_virtual_bytes", Labels: labels, Value: float64(p.VirtualBytes), Timestamp: m.Timestamp},
			MetricPoint{Name: "protheus_process_thread_count", Labels: labels, Value: float64(p.ThreadCount), Timestamp: m.Timestamp},
			MetricPoint{Name: "protheus_process_handle_count", Labels: labels, Value: float64(p.HandleCount), Timestamp: m.Timestamp},
		)
	}

	c.buffer.PushBatch(points)
}

// EnqueueTCPMetrics extracts and buffers latency and availability metrics from TCP checks.
func (c *Client) EnqueueTCPMetrics(tcps []collector.TCPServiceStatus) {
	var points []MetricPoint
	now := time.Now()

	for _, s := range tcps {
		labels := map[string]string{"service": s.Name, "port": strconv.Itoa(s.Port)}
		upVal := 0.0
		if s.Up {
			upVal = 1.0
		}

		points = append(points,
			MetricPoint{Name: "protheus_tcp_up", Labels: labels, Value: upVal, Timestamp: now},
			MetricPoint{Name: "protheus_tcp_latency_ms", Labels: labels, Value: s.LatencyMs, Timestamp: now},
		)
	}

	c.buffer.PushBatch(points)
}

func (c *Client) flushLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), c.cfg.RequestTimeout)
			_ = c.Flush(ctx)
			cancel()
		}
	}
}

// Flush sends up to BatchSize metrics from buffer to VictoriaMetrics.
func (c *Client) Flush(ctx context.Context) error {
	batch := c.buffer.PopBatch(c.cfg.BatchSize)
	if len(batch) == 0 {
		return nil
	}

	var sb strings.Builder
	for _, pt := range batch {
		sb.WriteString(pt.ToPrometheusLine())
		sb.WriteByte('\n')
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.importURL, bytes.NewBufferString(sb.String()))
	if err != nil {
		// Requeue metrics if possible
		c.buffer.PushBatch(batch)
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.mu.Lock()
		c.isHealthy = false
		c.lastError = err.Error()
		c.consecutiveFailures++
		c.mu.Unlock()

		// Requeue failed batch to bounded buffer
		c.buffer.PushBatch(batch)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("VictoriaMetrics responded with status %d: %s", resp.StatusCode, string(body))
		c.mu.Lock()
		c.isHealthy = false
		c.lastError = errMsg
		c.consecutiveFailures++
		c.mu.Unlock()

		// Requeue failed batch
		c.buffer.PushBatch(batch)
		return fmt.Errorf("%s", errMsg)
	}

	c.mu.Lock()
	c.isHealthy = true
	c.lastError = ""
	c.consecutiveFailures = 0
	c.mu.Unlock()

	return nil
}

// QueryRange executes a MetricsQL/PromQL range query against VictoriaMetrics.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]TimeSeriesResult, error) {
	u, err := url.Parse(c.queryRangeURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", start.Unix()))
	q.Set("end", fmt.Sprintf("%d", end.Unix()))
	q.Set("step", fmt.Sprintf("%ds", int(step.Seconds())))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query_range error (status %d): %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed decoding query response: %w", err)
	}

	var results []TimeSeriesResult
	for _, res := range parsed.Data.Result {
		var points []TimePoint
		for _, v := range res.Values {
			if len(v) < 2 {
				continue
			}
			tsSec, _ := v[0].(float64)
			valStr, _ := v[1].(string)
			val, _ := strconv.ParseFloat(valStr, 64)
			points = append(points, TimePoint{
				Timestamp: int64(tsSec),
				Value:     val,
			})
		}
		results = append(results, TimeSeriesResult{
			Metric: res.Metric,
			Values: points,
		})
	}

	return results, nil
}

// Status returns health and buffer statistics for VictoriaMetrics integration.
func (c *Client) Status() (isHealthy bool, bufferLen int, dropped uint64, lastErr string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isHealthy, c.buffer.Len(), c.buffer.DroppedCount(), c.lastError
}
