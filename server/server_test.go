package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoggrigori/ttlcache/cache"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := cache.New(0)
	t.Cleanup(store.Close)
	ts := httptest.NewServer(New(store, time.Minute).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, ts *httptest.Server, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func TestPutThenGet(t *testing.T) {
	ts := newTestServer(t)

	resp := do(t, ts, http.MethodPut, "/kv/greeting", "hello")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d; want 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, ts, http.MethodGet, "/kv/greeting", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d; want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "hello" {
		t.Fatalf("GET body = %q; want %q", got, "hello")
	}
}

func TestGetMissing(t *testing.T) {
	ts := newTestServer(t)
	resp := do(t, ts, http.MethodGet, "/kv/nope", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET missing status = %d; want 404", resp.StatusCode)
	}
}

func TestGetExpired(t *testing.T) {
	ts := newTestServer(t)

	resp := do(t, ts, http.MethodPut, "/kv/quick?ttl=20ms", "bye")
	resp.Body.Close()

	time.Sleep(40 * time.Millisecond)

	resp = do(t, ts, http.MethodGet, "/kv/quick", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET expired status = %d; want 404", resp.StatusCode)
	}
}

func TestDelete(t *testing.T) {
	ts := newTestServer(t)

	resp := do(t, ts, http.MethodPut, "/kv/temp", "x")
	resp.Body.Close()

	resp = do(t, ts, http.MethodDelete, "/kv/temp", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d; want 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, ts, http.MethodGet, "/kv/temp", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after DELETE status = %d; want 404", resp.StatusCode)
	}
}

func TestInvalidTTL(t *testing.T) {
	ts := newTestServer(t)
	resp := do(t, ts, http.MethodPut, "/kv/bad?ttl=notaduration", "x")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("PUT invalid ttl status = %d; want 400", resp.StatusCode)
	}
}

func TestStats(t *testing.T) {
	ts := newTestServer(t)

	resp := do(t, ts, http.MethodPut, "/kv/a", "1")
	resp.Body.Close()
	do(t, ts, http.MethodGet, "/kv/a", "").Body.Close()   // hit
	do(t, ts, http.MethodGet, "/kv/zzz", "").Body.Close() // miss

	resp = do(t, ts, http.MethodGet, "/stats", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /stats status = %d; want 200", resp.StatusCode)
	}
	var st cache.Stats
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if st.Items != 1 || st.Hits != 1 || st.Misses != 1 {
		t.Fatalf("stats = %+v; want items=1 hits=1 misses=1", st)
	}
}

func TestKeys(t *testing.T) {
	ts := newTestServer(t)

	// Empty cache must serialize as an empty JSON array, not null.
	resp := do(t, ts, http.MethodGet, "/keys", "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("GET /keys on empty cache = %q; want []", body)
	}

	do(t, ts, http.MethodPut, "/kv/a", "1").Body.Close()
	do(t, ts, http.MethodPut, "/kv/b", "2").Body.Close()

	resp = do(t, ts, http.MethodGet, "/keys", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /keys status = %d; want 200", resp.StatusCode)
	}
	var keys []string
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		t.Fatalf("decode keys: %v", err)
	}
	want := map[string]bool{"a": true, "b": true}
	if len(keys) != len(want) {
		t.Fatalf("GET /keys = %v; want keys %v", keys, want)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("GET /keys returned unexpected key %q (got %v)", k, keys)
		}
	}
}

func TestFlush(t *testing.T) {
	ts := newTestServer(t)

	do(t, ts, http.MethodPut, "/kv/a", "1").Body.Close()
	do(t, ts, http.MethodPut, "/kv/b", "2").Body.Close()

	resp := do(t, ts, http.MethodDelete, "/kv", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /kv status = %d; want 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = do(t, ts, http.MethodGet, "/kv/a", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /kv/a after flush status = %d; want 404", resp.StatusCode)
	}
}
