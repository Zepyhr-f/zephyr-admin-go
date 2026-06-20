package gatewaysign

import (
	"context"
	"sync"
	"time"
)

// MemoryNonceStore is an in-memory NonceStore primarily used for tests.
type MemoryNonceStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

// NewMemoryNonceStore creates an empty in-memory store.
func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{entries: make(map[string]time.Time)}
}

// Reserve atomically inserts the nonce and returns true when previously absent.
func (m *MemoryNonceStore) Reserve(_ context.Context, nonce string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if exp, ok := m.entries[nonce]; ok && exp.After(now) {
		return false, nil
	}
	m.entries[nonce] = now.Add(ttl)
	return true, nil
}
