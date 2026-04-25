package cache

import (
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Store is a minimal cache interface. It is intentionally small so that
// swapping in a Redis-backed implementation only requires a new struct.
type Store interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(keys ...string)
	// DeleteByPrefix removes every cached key that starts with prefix.
	// Used for namespace-level invalidation (e.g. all "nav:" entries).
	DeleteByPrefix(prefix string)
}

// InMemory wraps github.com/patrickmn/go-cache for single-process caching.
type InMemory struct{ c *gocache.Cache }

// NewInMemory creates an in-memory store. cleanupInterval controls how often
// expired entries are purged from memory (separate from TTL per item).
func NewInMemory(cleanupInterval time.Duration) *InMemory {
	return &InMemory{c: gocache.New(gocache.NoExpiration, cleanupInterval)}
}

func (m *InMemory) Get(key string) (interface{}, bool) { return m.c.Get(key) }

func (m *InMemory) Set(key string, value interface{}, ttl time.Duration) {
	m.c.Set(key, value, ttl)
}

func (m *InMemory) Delete(keys ...string) {
	for _, k := range keys {
		m.c.Delete(k)
	}
}

func (m *InMemory) DeleteByPrefix(prefix string) {
	for k := range m.c.Items() {
		if strings.HasPrefix(k, prefix) {
			m.c.Delete(k)
		}
	}
}

// Noop is a no-op Store. Use it in tests or when caching is intentionally disabled.
type Noop struct{}

func (Noop) Get(_ string) (interface{}, bool)             { return nil, false }
func (Noop) Set(_ string, _ interface{}, _ time.Duration) {}
func (Noop) Delete(_ ...string)                           {}
func (Noop) DeleteByPrefix(_ string)                      {}
