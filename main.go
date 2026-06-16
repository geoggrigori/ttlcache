// Command ttlcache runs a concurrency-safe in-memory key-value cache with
// per-key TTL, exposed over HTTP.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/geoggrigori/ttlcache/cache"
	"github.com/geoggrigori/ttlcache/server"
)

func main() {
	port := flag.String("port", env("PORT", "8080"), "TCP port to listen on")
	defaultTTL := flag.Duration("ttl", envDuration("TTL", time.Minute),
		"default TTL applied when a request omits one")
	interval := flag.Duration("janitor-interval", envDuration("JANITOR_INTERVAL", 30*time.Second),
		"how often the janitor evicts expired keys")
	flag.Parse()

	store := cache.New(*interval)
	defer store.Close()

	srv := server.New(store, *defaultTTL)
	addr := ":" + *port

	log.Printf("ttlcache listening on %s (default ttl %s, janitor interval %s)",
		addr, *defaultTTL, *interval)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

// env returns the value of the environment variable named key, or def if unset.
func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// envDuration returns the duration parsed from the environment variable named
// key, or def if unset or unparseable.
func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
