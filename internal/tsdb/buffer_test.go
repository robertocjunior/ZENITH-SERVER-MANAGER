package tsdb

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBoundedBufferOverflowAndDropPolicy(t *testing.T) {
	capacity := 10
	buf := NewBoundedBuffer(capacity)

	// Push 25 items into a buffer of size 10
	for i := 0; i < 25; i++ {
		buf.Push(MetricPoint{
			Name:      fmt.Sprintf("metric_%d", i),
			Value:     float64(i),
			Timestamp: time.Now(),
		})
	}

	// Len must be capped at capacity
	if buf.Len() != capacity {
		t.Fatalf("expected Len() to be %d, got %d", capacity, buf.Len())
	}

	// Dropped count must be 15 (25 - 10)
	if buf.DroppedCount() != 15 {
		t.Fatalf("expected 15 dropped items, got %d", buf.DroppedCount())
	}

	// Items remaining should be metric_15 through metric_24
	batch := buf.PopBatch(capacity)
	if len(batch) != capacity {
		t.Fatalf("expected batch of %d items, got %d", capacity, len(batch))
	}

	for i, item := range batch {
		expectedVal := float64(15 + i)
		if item.Value != expectedVal {
			t.Errorf("at index %d: expected value %f, got %f", i, expectedVal, item.Value)
		}
	}

	// Buffer should now be empty
	if buf.Len() != 0 {
		t.Errorf("expected buffer Len to be 0 after PopBatch, got %d", buf.Len())
	}
}

func TestBoundedBufferConcurrency(t *testing.T) {
	buf := NewBoundedBuffer(100)
	var wg sync.WaitGroup

	// Concurrent producers
	for p := 0; p < 10; p++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				buf.Push(MetricPoint{
					Name:      "test_metric",
					Value:     float64(producerID*100 + i),
					Timestamp: time.Now(),
				})
			}
		}(p)
	}

	// Concurrent consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = buf.PopBatch(10)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()

	if buf.Len() > 100 {
		t.Errorf("buffer exceeded max capacity: %d", buf.Len())
	}
}
