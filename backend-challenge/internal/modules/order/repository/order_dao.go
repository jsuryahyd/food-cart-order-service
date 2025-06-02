package orderrepository

import (
	"time"

	"github.com/google/uuid"
	oe "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/entities"
)

type OrderDAO struct {
	ID         uuid.UUID `db:"id"`
	Total      float64   `db:"total"`
	UserId     uuid.UUID `db:"user_id"`
	UserName   string    `db:"user_name"`
	Status     string    `db:"status"`
	CouponCode *string   `db:"coupon_code"` // *string to allow NULL in DB
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type OrderItemDAO struct {
	OrderID         uuid.UUID `db:"order_id"`
	ProductID       uuid.UUID `db:"product_id"`
	Quantity        int       `db:"quantity"`
	PriceAtPurchase float64   `db:"price_at_order_time"`
}

func (o *OrderDAO) ToDomainModel(itemDAOs []*OrderItemDAO) *oe.Order {
	if o == nil {
		return nil
	}

	orderItems := make([]oe.OrderItem, len(itemDAOs))
	for i, item := range itemDAOs {
		orderItems[i] = oe.OrderItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		}
	}

	couponCode := ""
	if o.CouponCode != nil {
		couponCode = *o.CouponCode
	}

	return &oe.Order{
		Id:    o.ID,
		Total: o.Total,
		Items: orderItems,
		User: oe.User{
			Id:   o.UserId,
			Name: o.UserName,
		},
		Status:     o.Status,
		CouponCode: couponCode,
		CreatedAt:  o.CreatedAt,
		UpdatedAt:  o.UpdatedAt,
		Products:   nil, // loaded by the service layer, not directly loaded from DAO
	}
}

func FromOrderEntity(order *oe.Order) (*OrderDAO, []*OrderItemDAO) {
	if order == nil {
		return nil, nil
	}

	var couponCode *string
	if order.CouponCode != "" {
		cc := order.CouponCode
		couponCode = &cc
	}

	orderDAO := &OrderDAO{
		ID:         order.Id,
		Total:      order.Total,
		UserId:     order.User.Id,
		CouponCode: couponCode,
		Status:     order.Status,
		CreatedAt:  order.CreatedAt,
		UpdatedAt:  order.UpdatedAt,
	}

	itemDAOs := make([]*OrderItemDAO, len(order.Items))
	for i, item := range order.Items {
		itemDAOs[i] = &OrderItemDAO{
			PriceAtPurchase: item.PriceAtPurchase,
			OrderID:         order.Id,
			ProductID:       item.ProductId,
			Quantity:        item.Quantity,
		}
	}
	return orderDAO, itemDAOs
}
