package messaging

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type Publisher interface {
	Publish(ctx context.Context, channel, message string) error
}

// implements the Publisher interface using Redis Pub/Sub.
type RedisPublisher struct {
	client *redis.Client
}

func NewRedisPublisher(client *redis.Client) *RedisPublisher {
	return &RedisPublisher{client: client}
}

func (r *RedisPublisher) Publish(ctx context.Context, channel, message string) error {
	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Add a timeout for publish
	defer cancel()

	cmd := r.client.Publish(publishCtx, channel, message)
	if cmd.Err() != nil {
		return fmt.Errorf("failed to publish message to Redis channel %s: %w", channel, cmd.Err())
	}
	log.Printf("Published message to Redis channel '%s': '%s'", channel, message)
	return nil
}
