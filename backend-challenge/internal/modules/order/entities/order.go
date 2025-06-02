package entities

import (
	"time"

	"github.com/google/uuid"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
)

type OrderItem struct {
	ProductId       uuid.UUID `json:"productId"`
	Quantity        int       `json:"quantity"`
	PriceAtPurchase float64   `json:"price"`
	OrderId         uuid.UUID `json:"orderId"`
}

type User struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Order struct {
	Id         uuid.UUID     `json:"id"`
	Total      float64       `json:"total"`
	User       User          `json:"user"`
	Items      []OrderItem   `json:"items"`
	Products   []*pe.Product `json:"products"`
	CouponCode string        `json:"couponCode"`
	Status     string        `json:"status"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

type PlaceOrderPayload struct {
	CouponCode string      `json:"couponCode"`
	Items      []OrderItem `json:"items"`
	UserId     uuid.UUID   `json:"userId"`
}
