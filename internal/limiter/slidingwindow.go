package limiter

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/sliding_window.lua
var slidingWindowScript string

type SlidingWindowLog struct {
	rdb        *redis.Client
	script     *redis.Script
	limit      float64 // max requests allowed in the window
	windowSize float64 // window duration in seconds
}

var _ Limiter = (*SlidingWindowLog)(nil)

func NewSlidingWindowLog(rdb *redis.Client, limit, windowSize float64) *SlidingWindowLog {
	return &SlidingWindowLog{
		rdb:        rdb,
		script:     redis.NewScript(slidingWindowScript),
		limit:      limit,
		windowSize: windowSize,
	}
}

func (s *SlidingWindowLog) Check(ctx context.Context, tenant string, cost float64) (Decision, error) {
	now := float64(time.Now().UnixNano()) / 1e9
	// Unique request ID for ZADD member using nanosecond timestamp
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	res, err := s.script.Run(ctx, s.rdb,
		[]string{"rl:sw:" + tenant},
		s.limit, s.windowSize, now, requestID,
	).Result()
	if err != nil {
		return Decision{}, err
	}

	return parseReply(res)
}
