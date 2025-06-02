package dto

import (
	"time"

	"github.com/google/uuid"
	oe "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/entities"
	pdto "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/http/dto"
)

type OrderRequestItem struct {
	ProductId string `json:"productId" binding:"required,uuid"` // Product ID as string, will be parsed to UUID
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

type PlaceOrderRequest struct {
	CouponCode string             `json:"couponCode"`
	Items      []OrderRequestItem `json:"items" binding:"required,min=1"`
}

type OrderItemResponse struct {
	ProductId uuid.UUID `json:"productId"`
	Quantity  int       `json:"quantity"`
}

type OrderResponse struct {
	Id        uuid.UUID               `json:"id"`
	Total     float64                 `json:"total"`
	Items     []OrderItemResponse     `json:"items"`
	Products  []*pdto.ProductResponse `json:"products"`
	CreatedAt time.Time               `json:"createdAt"`
	UpdatedAt time.Time               `json:"updatedAt"`
}

func NewOrderResponseFromEntity(entity *oe.Order) *OrderResponse {
	if entity == nil {
		return nil
	}

	itemResponses := make([]OrderItemResponse, len(entity.Items))
	for i, item := range entity.Items {
		itemResponses[i] = OrderItemResponse{
			ProductId: item.ProductId,
			Quantity:  item.Quantity,
		}
	}

	return &OrderResponse{
		Id:        entity.Id,
		Total:     entity.Total,
		Items:     itemResponses,
		Products:  pdto.NewProductResponseListFromEntities(entity.Products),
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
}
