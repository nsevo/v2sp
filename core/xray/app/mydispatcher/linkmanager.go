package mydispatcher

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
)

type ManagedWriter struct {
	writer     buf.Writer
	manager    *LinkManager
	createTime time.Time
	element    *list.Element // Reference to list element for O(1) removal
	// Activity tracking for idle cleanup and LRU ordering
	lastActive atomic.Int64
	lastMoved  atomic.Int64
}

func (w *ManagedWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	w.manager.touch(w)
	return w.writer.WriteMultiBuffer(mb)
}

func (w *ManagedWriter) Close() error {
	w.manager.RemoveWriter(w)
	return common.Close(w.writer)
}

// linkEntry stores the writer-reader pair in the ordered list
type linkEntry struct {
	writer *ManagedWriter
	reader buf.Reader
}

// LinkManager manages connections using a doubly-linked list for O(1) oldest removal
type LinkManager struct {
	links *list.List                    // Ordered list (oldest at front)
	index map[*ManagedWriter]*linkEntry // Fast lookup by writer
	mu    sync.Mutex
}

// NewLinkManager creates a new LinkManager
func NewLinkManager() *LinkManager {
	return &LinkManager{
		links: list.New(),
		index: make(map[*ManagedWriter]*linkEntry),
	}
}

func (m *LinkManager) AddLink(writer *ManagedWriter, reader buf.Reader) {
	now := time.Now().UnixNano()
	writer.lastActive.Store(now)
	writer.lastMoved.Store(now)
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &linkEntry{writer: writer, reader: reader}
	// Add to back of list (newest)
	elem := m.links.PushBack(entry)
	writer.element = elem
	m.index[writer] = entry
}

func (m *LinkManager) RemoveWriter(writer *ManagedWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if writer.element != nil {
		m.links.Remove(writer.element)
		writer.element = nil
	}
	delete(m.index, writer)
}

func (m *LinkManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for elem := m.links.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*linkEntry)
		common.Close(entry.writer.writer)
		common.Interrupt(entry.reader)
	}
	m.links.Init()
	m.index = make(map[*ManagedWriter]*linkEntry)
}

func (m *LinkManager) GetConnectionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.links.Len()
}

// RemoveOldest removes the oldest connection. O(1) operation.
func (m *LinkManager) RemoveOldest() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the oldest (front of list)
	front := m.links.Front()
	if front == nil {
		return false
	}

	entry := front.Value.(*linkEntry)
	m.links.Remove(front)
	entry.writer.element = nil
	delete(m.index, entry.writer)

	// Close the connection
	common.Close(entry.writer.writer)
	common.Interrupt(entry.reader)

	return true
}

// CloseOldestN closes up to n oldest connections (including active ones).
// Returns the number of connections closed.
func (m *LinkManager) CloseOldestN(n int) int {
	if n <= 0 {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	closed := 0
	for elem := m.links.Front(); elem != nil && closed < n; {
		next := elem.Next()
		entry := elem.Value.(*linkEntry)
		m.links.Remove(elem)
		entry.writer.element = nil
		delete(m.index, entry.writer)
		common.Close(entry.writer.writer)
		common.Interrupt(entry.reader)
		closed++
		elem = next
	}

	return closed
}

// touch updates activity and occasionally moves the connection to the back (LRU).
func (m *LinkManager) touch(writer *ManagedWriter) {
	now := time.Now().UnixNano()
	writer.lastActive.Store(now)

	// Throttle expensive move operations.
	if now-writer.lastMoved.Load() < int64(touchThrottleWindow) {
		return
	}
	writer.lastMoved.Store(now)

	m.mu.Lock()
	if writer.element != nil {
		m.links.MoveToBack(writer.element)
	}
	m.mu.Unlock()
}

// CleanupIdle scans up to maxScan oldest entries and closes those idle beyond idleTimeout.
// Returns number of connections closed.
func (m *LinkManager) CleanupIdle(maxScan int, idleTimeout time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if maxScan <= 0 {
		return 0
	}

	now := time.Now()
	closed := 0
	scanned := 0
	for elem := m.links.Front(); elem != nil && scanned < maxScan; {
		next := elem.Next()
		entry := elem.Value.(*linkEntry)
		last := time.Unix(0, entry.writer.lastActive.Load())
		if now.Sub(last) > idleTimeout {
			m.links.Remove(elem)
			entry.writer.element = nil
			delete(m.index, entry.writer)
			common.Close(entry.writer.writer)
			common.Interrupt(entry.reader)
			closed++
		}
		scanned++
		elem = next
	}
	return closed
}
