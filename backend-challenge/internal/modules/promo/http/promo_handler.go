package promohandler

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	rc "github.com/jsuryahyd/food-cart-order-service/internal/common/redis"
	messaging "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/messaging"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/entities"
)

type PromoService interface {
	RefreshCacheFromFiles() error
	ValidateCoupon(coupon entities.Coupon) (bool, error)
}

type PromoHandler struct {
	promoService     PromoService
	logger           *logging.Logger
	messagePublisher messaging.Publisher
	config           *config.Config
}

func NewPromoHandler(promoService PromoService, config *config.Config, messagePublisher messaging.Publisher) *PromoHandler {
	return &PromoHandler{
		promoService:     promoService,
		logger:           logging.GetLogger().With("handler", "promohandler"),
		messagePublisher: messagePublisher,
		config:           config,
	}
}

// starts a goroutine to listen for coupon update messages from the worker.
// When a message is received, it triggers a cache refresh.
func (h *PromoHandler) StartCouponUpdateListener(ctx context.Context, redisClient *rc.RedisClient, channel string) {
	go func() {
		subscriber := messaging.NewRedisSubscriber(redisClient, channel)
		defer subscriber.Close()

		// Use a cancellable context for the subscription, tied to the application's shutdown context.
		msgChannel, err := subscriber.Subscribe(ctx, channel)
		if err != nil {
			log.Fatalf("Error subscribing to coupon update channel %s: %v", channel, err)
			return // todo: Cannot recover from subscription error without re-init
		}

		h.logger.Info("App server is listening for coupon updates on Redis channel: %s", channel)

		for msg := range msgChannel {
			h.logger.Infof("Received coupon update message from Redis channel %s: %s\n", msg.Channel, msg.Payload)

			refreshErr := h.promoService.RefreshCacheFromFiles()
			if refreshErr != nil {
				h.logger.Errorf("Error during cache refresh triggered by Pub/Sub message: %v", refreshErr)
			} else {
				h.logger.Info("Cache refresh successfully completed after Pub/Sub trigger.")
			}
		}
		h.logger.Info("Coupon update listener stopped.")
	}()
}

// handles the API call from an external system to initiate coupon file processing.
// It publishes a message to Redis Pub/Sub, which the worker listens to.
func (h *PromoHandler) TriggerCouponFileProcessing(c *gin.Context) {
	h.logger.Info("Received request to trigger coupon file processing.")

	channel := h.config.Redis.WorkerTriggerChannel // Get channel from config
	message := "trigger_worker"

	err := h.messagePublisher.Publish(c, channel, message)
	if err != nil {
		log.Printf("Failed to publish trigger message to Redis channel %s: %v", channel, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to trigger coupon processing: internal error."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Coupon processing job triggered successfully via Redis channel '%s'.", channel)})
	h.logger.Infof("Trigger message published to Redis channel '%s'.", channel)
}
