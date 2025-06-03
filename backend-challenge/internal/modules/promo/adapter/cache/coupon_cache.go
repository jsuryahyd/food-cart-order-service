package cache

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/entities"
)

type CouponCache struct {
	redisClient *redis.Client
	ctx         context.Context
}

func NewCouponCache(client *redis.Client) *CouponCache {
	return &CouponCache{
		redisClient: client,
		ctx:         context.Background(),
	}
}

func (c *CouponCache) IsCouponValid(couponCode entities.Coupon) (bool, error) {
	// Use SISMEMBER for a set, or EXISTS for individual keys.
	cmd := c.redisClient.SIsMember(c.ctx, "valid_coupons", couponCode)
	if cmd.Err() != nil {
		return false, fmt.Errorf("redis SISMEMBER error: %w", cmd.Err())
	}
	return cmd.Val(), nil
}

/*
*
- replaces the entire set of valid coupons in Redis. It clears existing coupons and adds new ones.
*/
func (c *CouponCache) SetValidCoupons(coupons []entities.Coupon) error {
	pipe := c.redisClient.TxPipeline() // Use a transaction pipeline for atomicity

	// Clear existing coupons
	pipe.Del(c.ctx, "valid_coupons")

	// Add new coupons to the set
	if len(coupons) > 0 {
		members := make([]interface{}, len(coupons))
		for i, coupon := range coupons {
			members[i] = coupon
		}
		pipe.SAdd(c.ctx, "valid_coupons", members...)
	}

	_, err := pipe.Exec(c.ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline execution error: %w", err)
	}
	log.Printf("Successfully set %d valid coupons in Redis cache.", len(coupons))
	return nil
}

/*
*
- Adds a single coupon to the Redis set of valid coupons.
- This is used when promoting a coupon from the file cold store to the hot cache.
- Redis's maxmemory policy will handle LRU eviction if the cache is full.
*/
func (c *CouponCache) AddCouponToCache(couponCode entities.Coupon) error {
	cmd := c.redisClient.SAdd(c.ctx, "valid_coupons", couponCode)
	if cmd.Err() != nil {
		return fmt.Errorf("redis SADD error for %s: %w", couponCode, cmd.Err())
	}
	log.Printf("Coupon %s added/promoted to Redis hot cache.", couponCode)
	return nil
}
