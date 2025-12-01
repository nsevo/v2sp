package mydispatcher

import (
	sync "sync"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
)

type ManagedWriter struct {
	writer     buf.Writer
	manager    *LinkManager
	createTime time.Time
}

func (w *ManagedWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	return w.writer.WriteMultiBuffer(mb)
}

func (w *ManagedWriter) Close() error {
	w.manager.RemoveWriter(w)
	return common.Close(w.writer)
}

type LinkManager struct {
	links map[*ManagedWriter]buf.Reader
	mu    sync.Mutex
}

func (m *LinkManager) AddLink(writer *ManagedWriter, reader buf.Reader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links[writer] = reader
}

func (m *LinkManager) RemoveWriter(writer *ManagedWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.links, writer)
}

func (m *LinkManager) CloseAll() {
	for w, r := range m.links {
		common.Close(w)
		common.Interrupt(r)
	}
}

func (m *LinkManager) GetConnectionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.links)
}

func (m *LinkManager) RemoveOldest() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.links) == 0 {
		return false
	}
	
	var oldestWriter *ManagedWriter
	var oldestTime time.Time
	
	// Find the oldest connection
	for w := range m.links {
		if oldestWriter == nil || w.createTime.Before(oldestTime) {
			oldestWriter = w
			oldestTime = w.createTime
		}
	}
	
	if oldestWriter != nil {
		reader := m.links[oldestWriter]
		delete(m.links, oldestWriter)
		// Close the oldest connection
		common.Close(oldestWriter.writer)
		common.Interrupt(reader)
		return true
	}
	
	return false
}
