// Package server exposes a ttlcache Store over HTTP using a Go 1.22+
// pattern-based ServeMux.
package server

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/geoggrigori/ttlcache/cache"
)

// Server wraps a cache.Store and serves it over HTTP.
type Server struct {
	store      *cache.Store
	defaultTTL time.Duration
}

// New returns a Server backed by store, using defaultTTL when a request does
// not specify a TTL.
func New(store *cache.Store, defaultTTL time.Duration) *Server {
	return &Server{store: store, defaultTTL: defaultTTL}
}

// Handler returns the HTTP handler exposing the cache API:
//
//	PUT    /kv/{key}  store a value (TTL from ?ttl= or X-TTL, else default)
//	GET    /kv/{key}  fetch a value (404 if missing or expired)
//	DELETE /kv/{key}  remove a value (always 204)
//	GET    /keys      JSON array of the currently non-expired keys
//	DELETE /kv        flush the entire cache (always 204)
//	GET    /stats     JSON snapshot of items, hits, misses
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /kv/{key}", s.handlePut)
	mux.HandleFunc("GET /kv/{key}", s.handleGet)
	mux.HandleFunc("DELETE /kv/{key}", s.handleDelete)
	mux.HandleFunc("GET /keys", s.handleKeys)
	mux.HandleFunc("DELETE /kv", s.handleFlush)
	mux.HandleFunc("GET /stats", s.handleStats)
	return mux
}

// resolveTTL determines the TTL for a request from the ?ttl= query parameter,
// then the X-TTL header, falling back to the server default. An invalid
// duration is reported via ok=false.
func (s *Server) resolveTTL(r *http.Request) (ttl time.Duration, ok bool) {
	raw := r.URL.Query().Get("ttl")
	if raw == "" {
		raw = r.Header.Get("X-TTL")
	}
	if raw == "" {
		return s.defaultTTL, true
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	ttl, ok := s.resolveTTL(r)
	if !ok {
		http.Error(w, "invalid ttl", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	s.store.Set(key, string(body), ttl)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	value, ok := s.store.Get(key)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.WriteString(w, value)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	s.store.Delete(r.PathValue("key"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.store.Keys()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(keys)
}

func (s *Server) handleFlush(w http.ResponseWriter, r *http.Request) {
	s.store.Flush()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.store.Stats())
}
