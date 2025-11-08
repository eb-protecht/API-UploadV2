package configs

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

// ConnectREDISDB connects to Redis
func ConnectREDISDB() error {
	redisURL := RedisURL()
	
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	redisClient = redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

// GetRedisClient returns the Redis client
func GetRedisClient() *redis.Client {
	return redisClient
}