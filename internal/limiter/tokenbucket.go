package limiter

import (
	"context"
	_ "embed" // importing the lua script
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ebedding the lua script
//
//go:embed scripts/token_bucket.lua
var tokenBucketScript string

// the limiter's struct
type TokenBucket struct {
	rdb      *redis.Client
	script   *redis.Script
	capacity float64
	rate     float64
}

var _ Limiter = (*TokenBucket)(nil)

//constructor

func NewTokenBucket(rdb *redis.Client, capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		rdb:      rdb,
		script:   redis.NewScript(tokenBucketScript),
		capacity: capacity,
		rate:     rate,
	}
}

func (t *TokenBucket) Check(ctx context.Context, tenant string, cost float64) (Decision, error) {
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := t.script.Run(ctx, t.rdb,
		[]string{bucketKey(tenant)},
		t.capacity, t.rate, now, cost,
	).Result()
	if err != nil {
		return Decision{}, err
	}

	return parseReply(res)
}

func bucketKey(tenant string) string {
	return "rl:" + tenant
}

//parse reply ie modifying redis ans to our struct

func parseReply(res any) (Decision, error) {
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 2 {
		return Decision{}, fmt.Errorf("unexpected reply shape: %#v", res)
	}

	allowed, ok := arr[0].(int64)
	if !ok {
		return Decision{}, fmt.Errorf("bad allowed field: %#v", arr[0])
	}

	tokensStr, ok := arr[1].(string)
	if !ok {
		return Decision{}, fmt.Errorf("bad tokens field: %#v", arr[1])
	}
	tokens, err := strconv.ParseFloat(tokensStr, 64)
	if err != nil {
		return Decision{}, fmt.Errorf("parsing tokens: %w", err)
	}

	return Decision{
		Allowed:   allowed == 1,
		Remaining: tokens,
	}, nil
}
