package limiter

import (
	"context"
	"sync"
	"time"
)

type MemoryLimiter struct {
	capacity float64
	rate     float64

	mu      sync.Mutex
	buckets map[string]*memBucket
}

type memBucket struct {
	tokens float64
	ts     time.Time
}

// compile-time proof it satisfies the Limiter interface
var _ Limiter = (*MemoryLimiter)(nil)

func NewMemoryLimiter(capacity, rate float64) *MemoryLimiter {
	return &MemoryLimiter{
		capacity: capacity,
		rate:     rate,
		buckets:  make(map[string]*memBucket),
	}
}

func (m *MemoryLimiter) Check(ctx context.Context, tenant string, cost float64) (Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	b, ok := m.buckets[tenant]
	if !ok {
		b = &memBucket{tokens: m.capacity, ts: now}
		m.buckets[tenant] = b
	}

	// same lazy refill as your Lua script
	elapsed := now.Sub(b.ts).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens = min(m.capacity, b.tokens+elapsed*m.rate)
	b.ts = now

	allowed := false
	if b.tokens >= cost {
		allowed = true
		b.tokens -= cost
	}

	return Decision{Allowed: allowed, Remaining: b.tokens}, nil
}
