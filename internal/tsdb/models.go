package tsdb

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// MetricPoint represents a single metric observation with labels and timestamp.
type MetricPoint struct {
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp time.Time
}

// ToPrometheusLine converts the MetricPoint to Prometheus text exposition format.
// e.g. protheus_cpu_percent{host="srv01"} 42.5 1724750000000
func (m *MetricPoint) ToPrometheusLine() string {
	tsMs := m.Timestamp.UnixMilli()
	if len(m.Labels) == 0 {
		return fmt.Sprintf("%s %g %d", m.Name, m.Value, tsMs)
	}

	keys := make([]string, 0, len(m.Labels))
	for k := range m.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var labelPairs []string
	for _, k := range keys {
		escapedVal := strings.ReplaceAll(m.Labels[k], "\"", "\\\"")
		labelPairs = append(labelPairs, fmt.Sprintf("%s=\"%s\"", k, escapedVal))
	}

	return fmt.Sprintf("%s{%s} %g %d", m.Name, strings.Join(labelPairs, ","), m.Value, tsMs)
}

// TimeSeriesResult represents a series of data points returned from a TSDB query.
type TimeSeriesResult struct {
	Metric map[string]string `json:"metric"`
	Values []TimePoint       `json:"values"`
}

// TimePoint represents a timestamp and float value pair.
type TimePoint struct {
	Timestamp int64   `json:"timestamp"` // Unix timestamp in seconds
	Value     float64 `json:"value"`
}
