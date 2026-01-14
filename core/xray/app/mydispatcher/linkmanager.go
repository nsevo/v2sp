package mydispatcher

import (
	"container/list"
	"sync"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
)

type ManagedWriter struct {
	writer     buf.Writer
	manager    *LinkManager
	createTime time.Time
	element    *list.Element // Reference to list element for O(1) removal
}

func (w *ManagedWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
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

// NOTE: We intentionally do not implement any connection limiting here.
