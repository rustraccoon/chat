package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client  *redis.Client
	limit   int           // requests
	window  time.Duration // time window
}

func NewRateLimiter(client *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

func (rl *RateLimiter) Allow(userID string) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("rate_limit:%s", userID)

	// increment the counter
	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// set expiration if this is the first request
	if count == 1 {
		err := rl.client.Expire(ctx, key, rl.window).Err()
		if err != nil {
			return false, err
		}
	}

	return count <= int64(rl.limit), nil
}

