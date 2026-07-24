package session

import (
	"bytes"
	"sync"
)

// MemoryStorage is an in-memory Storage for tests.
type MemoryStorage struct {
	mu          sync.Mutex
	header      []byte
	entries     [][]byte
	checkpoints int
	closed      bool
}

// NewMemoryStorage creates an empty in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

// WriteHeader stores the header line (first write wins).
func (m *MemoryStorage) WriteHeader(header []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.header != nil {
		return nil
	}
	m.header = append([]byte(nil), header...)
	return nil
}

// AppendEntry stores one entry line.
func (m *MemoryStorage) AppendEntry(entry []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, append([]byte(nil), entry...))
	return nil
}

// Checkpoint counts checkpoint calls.
func (m *MemoryStorage) Checkpoint() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints++
	return nil
}

// Close marks the storage closed.
func (m *MemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// CheckpointCount returns how many times Checkpoint was called.
func (m *MemoryStorage) CheckpointCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkpoints
}

// Closed reports whether Close was called.
func (m *MemoryStorage) Closed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// Contents renders the stored session as JSONL bytes (header + entries).
func (m *MemoryStorage) Contents() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	var buf bytes.Buffer
	if m.header != nil {
		buf.Write(m.header)
		buf.WriteByte('\n')
	}
	for _, entry := range m.entries {
		buf.Write(entry)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// EntryCount returns the number of appended entries.
func (m *MemoryStorage) EntryCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}
