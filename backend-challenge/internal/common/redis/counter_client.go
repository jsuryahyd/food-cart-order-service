package redisclient

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

var (
	redisCounterClient   *redis.Client
	oncePerCounterClient sync.Once
	counterCtx           = context.Background()
	counterLogger        = logging.GetLogger().With("module", "redis-counter-client")
)

// returns a singleton instance of the Redis client.
func GetRedisCounterClient(cfg *config.RedisCounterConfig) *redis.Client {
	oncePerCounterClient.Do(func() {
		counterLogger.Info("Initializing Redis counter client...")
		redisCounterClient = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password:     cfg.Password,
			DB:           cfg.DB,
			WriteTimeout: 60 * time.Second,
			ReadTimeout:  60 * time.Second,
		})

		// Ping Redis to check connectivity
		cmd := redisCounterClient.Ping(counterCtx)
		if cmd.Err() != nil {
			log.Fatalf("Could not connect to Redis: %v", cmd.Err())
		}
		counterLogger.Info("Successfully connected to Redis.")
	})
	return redisCounterClient
}

/*
*
- closes the Redis client connection.
- should be called during application shutdown.
*/
func CloseRedisCounterClient() {
	if redisCounterClient != nil {
		err := redisCounterClient.Close()
		if err != nil {
			log.Printf("Error closing Redis counter client: %v", err)
		} else {
			counterLogger.Info("Redis counter client closed.")
		}
	}
}
