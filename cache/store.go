// Package cache provides a concurrency-safe in-memory key-value store
// with per-key time-to-live (TTL) expiration and a background janitor
// that periodically evicts expired entries.
package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// item is a single stored value together with its expiration time.
type item struct {
	value     string
	expiresAt time.Time
}

// expired reports whether the item is expired relative to now.
func (it item) expired(now time.Time) bool {
	return now.After(it.expiresAt)
}

// Store is a thread-safe map of string keys to string values, each with an
// independent TTL. A background janitor goroutine evicts expired entries on a
// configurable interval. Hit and miss counts are tracked atomically.
//
// The zero value is not usable; create a Store with New.
type Store struct {
	mu    sync.RWMutex
	items map[string]item

	hits   atomic.Uint64
	misses atomic.Uint64

	now    func() time.Time // injectable clock, for testing
	stop   chan struct{}
	closed sync.Once
}

// Option configures a Store.
type Option func(*Store)

// WithClock overrides the clock used to evaluate expiry. It is intended for
// tests; production code should leave the default (time.Now).
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

// New creates a Store and starts a janitor goroutine that evicts expired
// entries every interval. If interval is zero or negative, no janitor is
// started. Call Close to stop the janitor and release resources.
func New(interval time.Duration, opts ...Option) *Store {
	s := &Store{
		items: make(map[string]item),
		now:   time.Now,
		stop:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	if interval > 0 {
		go s.janitor(interval)
	}
	return s
}

// janitor periodically removes expired entries until the Store is closed.
func (s *Store) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.evictExpired()
		case <-s.stop:
			return
		}
	}
}

// evictExpired removes all entries that have expired.
func (s *Store) evictExpired() {
	now := s.now()
	s.mu.Lock()
	for k, it := range s.items {
		if it.expired(now) {
			delete(s.items, k)
		}
	}
	s.mu.Unlock()
}

// Set stores value under key with the given ttl. A non-positive ttl stores the
// value with an already-passed expiration, so it is treated as missing.
func (s *Store) Set(key, value string, ttl time.Duration) {
	exp := s.now().Add(ttl)
	s.mu.Lock()
	s.items[key] = item{value: value, expiresAt: exp}
	s.mu.Unlock()
}

// Get returns the value stored under key and whether it was present and not
// expired. Expired entries are treated as missing. Get updates the hit/miss
// counters.
func (s *Store) Get(key string) (string, bool) {
	now := s.now()
	s.mu.RLock()
	it, ok := s.items[key]
	s.mu.RUnlock()
	if !ok || it.expired(now) {
		s.misses.Add(1)
		return "", false
	}
	s.hits.Add(1)
	return it.value, true
}

// Delete removes key from the Store. It is a no-op if the key is absent.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

// Len returns the number of entries currently stored, including any that have
// expired but not yet been evicted.
func (s *Store) Len() int {
	s.mu.RLock()
	n := len(s.items)
	s.mu.RUnlock()
	return n
}

// Keys returns the keys of all entries that are currently present and not
// expired. Expired entries that have not yet been evicted are excluded. The
// order of the returned slice is unspecified.
func (s *Store) Keys() []string {
	now := s.now()
	s.mu.RLock()
	keys := make([]string, 0, len(s.items))
	for k, it := range s.items {
		if !it.expired(now) {
			keys = append(keys, k)
		}
	}
	s.mu.RUnlock()
	return keys
}

// Flush removes all entries from the Store. Hit and miss counters are left
// unchanged.
func (s *Store) Flush() {
	s.mu.Lock()
	clear(s.items)
	s.mu.Unlock()
}

// Stats is a snapshot of cache counters.
type Stats struct {
	Items  int    `json:"items"`
	Hits   uint64 `json:"hits"`
	Misses uint64 `json:"misses"`
}

// Stats returns a snapshot of the current item count and hit/miss totals.
func (s *Store) Stats() Stats {
	return Stats{
		Items:  s.Len(),
		Hits:   s.hits.Load(),
		Misses: s.misses.Load(),
	}
}

// Close stops the janitor goroutine. It is safe to call Close multiple times.
func (s *Store) Close() {
	s.closed.Do(func() { close(s.stop) })
}
