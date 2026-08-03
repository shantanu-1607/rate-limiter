package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/shantanusingh/distributed-rate-limiter/internal/limiter"
)

// tenant is one customer: a name (used as the Redis bucket key) and its limiter.
type tenant struct {
	name    string
	limiter limiter.Limiter
}

// getenv reads an environment variable, falling back to a default.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newLimiter builds the limiter backend named by mode.
func newLimiter(mode string, rdb *redis.Client, capacity, rate float64) limiter.Limiter {
	switch mode {
	case "lua":
		return limiter.NewTokenBucket(rdb, capacity, rate)
	case "sliding":
		return limiter.NewSlidingWindowLog(rdb, capacity, rate)
	case "memory":
		return limiter.NewMemoryLimiter(capacity, rate)
	default:
		log.Fatalf("unknown LIMITER_MODE %q (want: lua, sliding, memory)", mode)
		return nil
	}
}

func main() {
	ctx := context.Background()

	redisAddr := getenv("REDIS_ADDR", "localhost:6379")
	port := getenv("PORT", "8080")
	instance := getenv("INSTANCE_ID", "limiter-1")
	mode := getenv("LIMITER_MODE", "lua")

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("cannot reach redis at %s: %v", redisAddr, err)
	}

	// Build a reverse proxy pointed at the real backend we protect.
	upstreamAddr := getenv("UPSTREAM_ADDR", "http://localhost:9000")
	upstreamURL, err := url.Parse(upstreamAddr)
	if err != nil {
		log.Fatalf("bad UPSTREAM_ADDR %q: %v", upstreamAddr, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	tenants := map[string]tenant{
		// Tenant 1: Token Bucket algorithm (5 capacity, 1/s refill rate)
		"free-key": {name: "free", limiter: newLimiter("lua", rdb, 5, 1)},

		// Tenant 2: Sliding Window Log algorithm (10 max requests per 10s window)
		"pro-key": {name: "pro", limiter: newLimiter("sliding", rdb, 10, 10)},

		// Tenant 3: Sliding Window Log algorithm (5 max requests per 5s window)
		"sliding-key": {name: "sliding", limiter: newLimiter("sliding", rdb, 5, 5)},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Served-By", instance) // which copy handled this

		key := r.Header.Get("X-API-Key")
		t, ok := tenants[key]
		if !ok {
			http.Error(w, "unknown or missing API key", http.StatusUnauthorized) // 401
			return
		}

		d, err := t.limiter.Check(r.Context(), t.name, 1)
		if err != nil {
			http.Error(w, "limiter error", http.StatusInternalServerError) // 500
			return
		}

		w.Header().Set("X-RateLimit-Remaining", strconv.FormatFloat(d.Remaining, 'f', 1, 64))

		if !d.Allowed {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests) // 429
			return
		}

		// allowed: forward the request to the real upstream
		proxy.ServeHTTP(w, r)
	}

	http.HandleFunc("/", handler)
	log.Printf("[%s] listening on :%s (mode=%s, redis=%s)", instance, port, mode, redisAddr)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
