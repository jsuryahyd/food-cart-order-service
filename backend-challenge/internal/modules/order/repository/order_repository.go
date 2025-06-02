package orderrepository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	oe "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/entities"
)

type OrderRepository interface {
	CreateOrder(ctx context.Context, tx *sql.Tx, order *OrderDAO, items []*OrderItemDAO) error
	GetOrderById(ctx context.Context, id uuid.UUID) (*oe.Order, error)
}
