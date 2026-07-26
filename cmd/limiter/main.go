package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/shantanusingh/distributed-rate-limiter/internal/limiter"
)

func main() {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic(err)
	}

	fmt.Println("connected to the redis")

	tb := limiter.NewTokenBucket(rdb, 5, 1)

	for i := 1; i <= 7; i++ {
		d, err := tb.Check(ctx, "acme", 1)
		if err != nil {
			panic(err)
		}
		fmt.Printf("request %d -> allowed = %v remaining = %.1f\n",
			i, d.Allowed, d.Remaining)
	}
}
