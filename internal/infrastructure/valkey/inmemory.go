package valkey

import (
	"context"
	"sync"
	"time"
)

type memEntry struct {
	val     string
	expires time.Time
}

type memImpl struct {
	mu   sync.Mutex
	data map[string]memEntry
}

// NewInMemory returns a *Client backed by a simple in-process map.
// It is intended for unit/integration tests only.
func NewInMemory() (*Client, error) {
	return &Client{impl: &memImpl{data: make(map[string]memEntry)}}, nil
}

func (m *memImpl) get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok {
		return "", nil
	}
	if !e.expires.IsZero() && time.Now().After(e.expires) {
		delete(m.data, key)
		return "", nil
	}
	return e.val, nil
}

func (m *memImpl) set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.data[key] = memEntry{val: value, expires: exp}
	return nil
}

func (m *memImpl) setNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.data[key]; ok && (e.expires.IsZero() || time.Now().Before(e.expires)) {
		return false, nil
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.data[key] = memEntry{val: value, expires: exp}
	return true, nil
}

func (m *memImpl) del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *memImpl) close() {}
