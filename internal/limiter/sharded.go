package limiter

import (
	"context"
	"fmt"
	"math/rand/v2"
)

// ShardedLimiter wraps multiple Limiter instances (shards) and routes
// requests randomly to one of them. This eliminates single-key contention
// in Redis by spreading the load across multiple keys.
type ShardedLimiter struct {
	shards []Limiter
}

var _ Limiter = (*ShardedLimiter)(nil)

// NewShardedLimiter creates a new ShardedLimiter.
// `numShards` determines how many underlying keys will be created.
// `builder` is a factory function that returns a new configured Limiter for a single shard.
func NewShardedLimiter(numShards int, builder func() Limiter) *ShardedLimiter {
	if numShards <= 0 {
		numShards = 1
	}

	shards := make([]Limiter, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = builder()
	}

	return &ShardedLimiter{
		shards: shards,
	}
}
func (s *ShardedLimiter) Check(ctx context.Context, tenant string, cost float64) (Decision, error) {
	// Pick a random shard to evenly distribute traffic
	shardIdx := rand.IntN(len(s.shards))

	// Create a sharded tenant key (e.g., "enterprise-key" -> "enterprise-key:shard:3")
	shardedTenant := fmt.Sprintf("%s:shard:%d", tenant, shardIdx)

	return s.shards[shardIdx].Check(ctx, shardedTenant, cost)
}
