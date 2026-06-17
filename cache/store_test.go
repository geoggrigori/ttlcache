package cache

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a thread-safe clock for tests. Multiple goroutines (e.g. the
// janitor) may read it while the test advances it.
type fakeClock struct {
	nanos atomic.Int64
}

func newFakeClock(t time.Time) *fakeClock {
	c := &fakeClock{}
	c.nanos.Store(t.UnixNano())
	return c
}

func (c *fakeClock) now() time.Time { return time.Unix(0, c.nanos.Load()) }

func (c *fakeClock) advance(d time.Duration) { c.nanos.Add(int64(d)) }

func TestSetGet(t *testing.T) {
	s := New(0)
	defer s.Close()

	s.Set("a", "alpha", time.Minute)
	got, ok := s.Get("a")
	if !ok || got != "alpha" {
		t.Fatalf("Get(a) = %q, %v; want %q, true", got, ok, "alpha")
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatalf("Get(missing) returned ok=true; want false")
	}
}

func TestExpiry(t *testing.T) {
	clock := newFakeClock(time.Now())
	s := New(0, WithClock(clock.now))
	defer s.Close()

	s.Set("a", "alpha", 10*time.Second)
	if _, ok := s.Get("a"); !ok {
		t.Fatalf("Get(a) before expiry: want present")
	}

	clock.advance(11 * time.Second) // advance past TTL
	if _, ok := s.Get("a"); ok {
		t.Fatalf("Get(a) after expiry: want missing")
	}
}

func TestDelete(t *testing.T) {
	s := New(0)
	defer s.Close()

	s.Set("a", "alpha", time.Minute)
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatalf("Get(a) after Delete: want missing")
	}
	s.Delete("a") // no-op on absent key
}

func TestJanitorEviction(t *testing.T) {
	clock := newFakeClock(time.Now())
	s := New(5*time.Millisecond, WithClock(clock.now))
	defer s.Close()

	s.Set("a", "alpha", time.Second)
	clock.advance(2 * time.Second) // entry is now expired

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.Len() == 0 {
			return // janitor evicted the expired entry
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("janitor did not evict expired entry; Len = %d", s.Len())
}

func TestStats(t *testing.T) {
	s := New(0)
	defer s.Close()

	s.Set("a", "alpha", time.Minute)
	s.Get("a")       // hit
	s.Get("missing") // miss

	st := s.Stats()
	if st.Items != 1 || st.Hits != 1 || st.Misses != 1 {
		t.Fatalf("Stats = %+v; want items=1 hits=1 misses=1", st)
	}
}

func TestKeys(t *testing.T) {
	clock := newFakeClock(time.Now())
	s := New(0, WithClock(clock.now))
	defer s.Close()

	if got := s.Keys(); len(got) != 0 {
		t.Fatalf("Keys() on empty store = %v; want empty", got)
	}

	s.Set("a", "alpha", time.Minute)
	s.Set("b", "beta", time.Minute)
	s.Set("short", "gone", 10*time.Second)

	clock.advance(11 * time.Second) // expire only "short"

	got := s.Keys()
	want := map[string]bool{"a": true, "b": true}
	if len(got) != len(want) {
		t.Fatalf("Keys() = %v; want keys %v", got, want)
	}
	for _, k := range got {
		if !want[k] {
			t.Fatalf("Keys() returned unexpected key %q (got %v)", k, got)
		}
	}
}

func TestFlush(t *testing.T) {
	s := New(0)
	defer s.Close()

	s.Set("a", "alpha", time.Minute)
	s.Set("b", "beta", time.Minute)
	s.Get("a") // record a hit so we can verify counters survive Flush

	s.Flush()

	if s.Len() != 0 {
		t.Fatalf("Len() after Flush = %d; want 0", s.Len())
	}
	if _, ok := s.Get("a"); ok {
		t.Fatalf("Get(a) after Flush: want missing")
	}
	if st := s.Stats(); st.Hits != 1 {
		t.Fatalf("Stats after Flush = %+v; want hits=1 preserved", st)
	}
}

func TestConcurrentSetGet(t *testing.T) {
	s := New(time.Millisecond)
	defer s.Close()

	const workers = 16
	const ops = 1000
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				s.Set("k"+strconv.Itoa(id), strconv.Itoa(i), time.Millisecond)
			}
		}(w)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				s.Get("k" + strconv.Itoa(id))
			}
		}(w)
	}
	wg.Wait()
}
