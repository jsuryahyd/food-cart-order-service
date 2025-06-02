package orderhandler_test

import (
	"testing"

	"github.com/google/uuid"
	orderhandler "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/http"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/order/http/dto"
	"github.com/stretchr/testify/assert"
)

func TestValidatePlaceOrderRequest(t *testing.T) {
	validProductID := uuid.New().String()

	t.Run("nil request", func(t *testing.T) {
		err := orderhandler.ValidatePlaceOrderRequest(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("empty items", func(t *testing.T) {
		req := &dto.PlaceOrderRequest{Items: []dto.OrderRequestItem{}}
		err := orderhandler.ValidatePlaceOrderRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one item is required")
	})

	t.Run("invalid productId", func(t *testing.T) {
		req := &dto.PlaceOrderRequest{
			Items: []dto.OrderRequestItem{
				{ProductId: "not-a-uuid", Quantity: 1},
			},
		}
		err := orderhandler.ValidatePlaceOrderRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid productId")
	})

	t.Run("quantity less than 1", func(t *testing.T) {
		req := &dto.PlaceOrderRequest{
			Items: []dto.OrderRequestItem{
				{ProductId: validProductID, Quantity: 0},
			},
		}
		err := orderhandler.ValidatePlaceOrderRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "quantity must be at least 1")
	})

	t.Run("duplicate productId", func(t *testing.T) {
		req := &dto.PlaceOrderRequest{
			Items: []dto.OrderRequestItem{
				{ProductId: validProductID, Quantity: 1},
				{ProductId: validProductID, Quantity: 2},
			},
		}
		err := orderhandler.ValidatePlaceOrderRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate productId")
	})

	t.Run("valid request", func(t *testing.T) {
		req := &dto.PlaceOrderRequest{
			Items: []dto.OrderRequestItem{
				{ProductId: uuid.New().String(), Quantity: 1},
				{ProductId: uuid.New().String(), Quantity: 2},
			},
		}
		err := orderhandler.ValidatePlaceOrderRequest(req)
		assert.NoError(t, err)
	})
}
