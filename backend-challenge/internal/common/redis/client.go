package redisclient

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

type RedisClient = redis.Client

var (
	redisClient *redis.Client
	once        sync.Once
	ctx         = context.Background()
	logger      = logging.GetLogger().With("module", "redis-client")
)

// returns a singleton instance of the Redis client.
func GetRedisClient(cfg *config.Config) *redis.Client {
	once.Do(func() {
		logger.Info("Initializing Redis client...")
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		// Ping Redis to check connectivity
		cmd := redisClient.Ping(ctx)
		if cmd.Err() != nil {
			log.Fatalf("Could not connect to Redis: %v", cmd.Err())
		}
		logger.Info("Successfully connected to Redis.")
	})
	return redisClient
}

/*
*
- closes the Redis client connection.
- should be called during application shutdown.
*/
func CloseRedisClient() {
	if redisClient != nil {
		err := redisClient.Close()
		if err != nil {
			log.Printf("Error closing Redis client: %v", err)
		} else {
			logger.Info("Redis client closed.")
		}
	}
}
