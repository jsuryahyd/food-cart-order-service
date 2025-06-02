package orderhandler

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/order/http/dto"
)

func ValidatePlaceOrderRequest(req *dto.PlaceOrderRequest) error {
	if req == nil {
		return errors.New("request cannot be nil")
	}
	if len(req.Items) == 0 {
		return errors.New("at least one item is required")
	}

	productIDSet := make(map[string]struct{})
	for i, item := range req.Items {

		if _, err := uuid.Parse(item.ProductId); err != nil {
			return fmt.Errorf("item %d: invalid productId: %v", i, err)
		}

		if item.Quantity < 1 {
			return fmt.Errorf("item %d: quantity must be at least 1", i)
		}
		// Check for duplicate ProductId
		if _, exists := productIDSet[item.ProductId]; exists {
			return fmt.Errorf("item %d: duplicate productId: %s", i, item.ProductId)
		}
		productIDSet[item.ProductId] = struct{}{}
	}
	return nil
}
