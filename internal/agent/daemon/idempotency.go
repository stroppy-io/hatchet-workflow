package daemon

import (
	"sync"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

const defaultCacheSize = 256

// reportCache is an in-memory LRU-like cache of command-id → Report
// capped to maxSize entries (evicts oldest insertion order).
type reportCache struct {
	mu      sync.Mutex
	maxSize int
	order   []string
	data    map[string]*agentpb.Report
}

func newReportCache(maxSize int) *reportCache {
	if maxSize <= 0 {
		maxSize = defaultCacheSize
	}
	return &reportCache{
		maxSize: maxSize,
		data:    make(map[string]*agentpb.Report, maxSize),
	}
}

// Lookup returns a cached Report for commandID, or nil.
func (c *reportCache) Lookup(commandID string) *agentpb.Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[commandID]
}

// Store saves a Report and evicts the oldest entry if at capacity.
func (c *reportCache) Store(commandID string, r *agentpb.Report) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.data[commandID]; !exists {
		if len(c.order) >= c.maxSize {
			evict := c.order[0]
			c.order = c.order[1:]
			delete(c.data, evict)
		}
		c.order = append(c.order, commandID)
	}
	c.data[commandID] = r
}
