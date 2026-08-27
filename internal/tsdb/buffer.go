package tsdb

import (
	"sync"
	"sync/atomic"
)

// BoundedBuffer is a thread-safe, memory-bounded FIFO queue for metrics.
// When max capacity is reached, it drops the oldest metric, guaranteeing
// strictly constant memory usage and zero memory leaks during TSDB outages.
type BoundedBuffer struct {
	mu           sync.Mutex
	items        []MetricPoint
	head         int
	tail         int
	count        int
	capacity     int
	droppedCount uint64
	enqueuedCount uint64
}

// NewBoundedBuffer creates a new bounded metric buffer.
func NewBoundedBuffer(capacity int) *BoundedBuffer {
	if capacity <= 0 {
		capacity = 10000
	}
	return &BoundedBuffer{
		items:    make([]MetricPoint, capacity),
		capacity: capacity,
	}
}

// Push adds a metric point to the buffer. If the buffer is full,
// the oldest metric point is dropped to prevent memory growth.
func (b *BoundedBuffer) Push(item MetricPoint) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.enqueuedCount++

	if b.count == b.capacity {
		// Buffer is full: overwrite oldest item at head
		b.head = (b.head + 1) % b.capacity
		b.count--
		atomic.AddUint64(&b.droppedCount, 1)
	}

	b.items[b.tail] = item
	b.tail = (b.tail + 1) % b.capacity
	b.count++
}

// PushBatch adds multiple metric points to the buffer in one locked operation.
func (b *BoundedBuffer) PushBatch(items []MetricPoint) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, item := range items {
		b.enqueuedCount++
		if b.count == b.capacity {
			b.head = (b.head + 1) % b.capacity
			b.count--
			atomic.AddUint64(&b.droppedCount, 1)
		}
		b.items[b.tail] = item
		b.tail = (b.tail + 1) % b.capacity
		b.count++
	}
}

// PopBatch removes and returns up to n items from the head of the buffer.
func (b *BoundedBuffer) PopBatch(n int) []MetricPoint {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 || n <= 0 {
		return nil
	}

	take := n
	if take > b.count {
		take = b.count
	}

	result := make([]MetricPoint, take)
	for i := 0; i < take; i++ {
		result[i] = b.items[b.head]
		// Clear reference to allow GC
		b.items[b.head] = MetricPoint{}
		b.head = (b.head + 1) % b.capacity
		b.count--
	}

	return result
}

// Len returns the current number of metrics in the buffer.
func (b *BoundedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// Capacity returns the maximum buffer capacity.
func (b *BoundedBuffer) Capacity() int {
	return b.capacity
}

// DroppedCount returns the total number of dropped metrics due to buffer overflow.
func (b *BoundedBuffer) DroppedCount() uint64 {
	return atomic.LoadUint64(&b.droppedCount)
}

// EnqueuedCount returns the total number of metrics pushed into the buffer.
func (b *BoundedBuffer) EnqueuedCount() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.enqueuedCount
}
