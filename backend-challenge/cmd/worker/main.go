// cmd/worker/main.go
package main

import (
	"context" // New import for binary encoding of the index
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	rc "github.com/jsuryahyd/food-cart-order-service/internal/common/redis"
	promoworker "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/worker"
)

// Coupon codes will be stored in a map where the key is the coupon code
// and the value is the count of files it appeared in.
type CouponCounts map[string]int

// CouponIndex stores coupon codes and their byte offsets in the processed_coupons.txt file.
type CouponIndex map[string]int64

var appConfig *config.Config
var redisClient *redis.Client  // Global Redis client instance
var ctx = context.Background() // Context for Redis operations

// init function to load environment variables and config
func init() {
	_ = godotenv.Load() // Loads .env file
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		configPath = "configs/config.yaml" // Default config path
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config on server startup: %v", err)
	}
	appConfig = cfg

	// Initialize Redis client using configuration
	redisClient = rc.GetRedisClient(cfg)
	// Ping Redis to check connectivity
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis.")

	// Log the loaded environment variables for debugging
	log.Printf("Loaded COUPON_FILE1_URL: %s", appConfig.CouponProcessor.COUPON_FILE1_URL)
	log.Printf("Loaded COUPON_FILE2_URL: %s", appConfig.CouponProcessor.COUPON_FILE2_URL)
	log.Printf("Loaded COUPON_FILE3_URL: %s", appConfig.CouponProcessor.COUPON_FILE3_URL)
	log.Printf("Loaded OUTPUT_FILE_PATH: %s", appConfig.CouponProcessor.OUTPUT_FILE_PATH)
	log.Printf("Loaded OUTPUT_INDEX_FILE_PATH: %s", appConfig.CouponProcessor.OUTPUT_INDEX_FILE_PATH) // New log
	log.Printf("Redis Host: %s, Port: %d", appConfig.Redis.Host, appConfig.Redis.Port)
	log.Printf("Redis Coupon Update Channel: %s", appConfig.Redis.CouponUpdateChannel)
}

func main() {
	if appConfig == nil {
		log.Fatal("App config not available or not loaded during init.")
	}

	worker := promoworker.NewWorker(appConfig, redisClient)
	log.Println("Starting Coupon pre-processor worker...")

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := worker.Run(ctx); err != nil {
			log.Fatalf("Worker encountered a fatal error %v", err)
		}
	}()
	<-quit //block execution until a signal is received
	log.Println("Shutting down worker...")
	rc.CloseRedisClient()
	log.Println("Coupon pre-processor worker stopped.")
}
