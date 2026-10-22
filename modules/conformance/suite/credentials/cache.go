package credentials

import (
	"context"
	"sync"
	"time"
)

// RefreshBefore is how long before expiry a cached token is replaced.
const RefreshBefore = 2 * time.Minute

type cacheKey struct{ user, client string }

// Cache keeps each client's credentials until shortly before they expire.
type Cache struct {
	source  Source
	now     func() time.Time
	mu      sync.Mutex
	entries map[cacheKey]Credentials
}

// NewCache returns a Cache in front of source.
func NewCache(source Source) *Cache {
	return &Cache{source: source, now: time.Now, entries: map[cacheKey]Credentials{}}
}

// Fetch implements Source.
func (c *Cache) Fetch(ctx context.Context, user, client string) (Credentials, error) {
	key := cacheKey{user, client}
	c.mu.Lock()
	cached, ok := c.entries[key]
	c.mu.Unlock()
	if ok && c.now().Add(RefreshBefore).Before(cached.Expiry) {
		return cached, nil
	}
	fresh, err := c.source.Fetch(ctx, user, client)
	if err != nil {
		return Credentials{}, err
	}
	c.mu.Lock()
	c.entries[key] = fresh
	c.mu.Unlock()
	return fresh, nil
}
