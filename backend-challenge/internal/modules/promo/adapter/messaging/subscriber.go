package messaging

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

type Subscriber interface {
	Subscribe(ctx context.Context, channel string) (<-chan *redis.Message, error)
	Close() error
}

type RedisSubscriber struct {
	client  *redis.Client
	pubsub  *redis.PubSub // Store the PubSub client
	channel string
}

func NewRedisSubscriber(client *redis.Client, channel string) *RedisSubscriber {
	return &RedisSubscriber{client: client, channel: channel}
}

func (r *RedisSubscriber) Subscribe(ctx context.Context, channel string) (<-chan *redis.Message, error) {
	r.channel = channel
	r.pubsub = r.client.Subscribe(ctx, channel)
	_, err := r.pubsub.Receive(ctx) // Wait for confirmation of subscription
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to Redis channel %s: %w", channel, err)
	}
	log.Printf("Subscribed to Redis channel: %s", channel)
	return r.pubsub.Channel(), nil
}

func (r *RedisSubscriber) Close() error {
	if r.pubsub != nil {
		err := r.pubsub.Close()
		if err != nil {
			return fmt.Errorf("failed to close Redis PubSub connection: %w", err)
		}
		log.Printf("Redis PubSub connection to channel %s closed.", r.channel)
	}
	return nil
}
