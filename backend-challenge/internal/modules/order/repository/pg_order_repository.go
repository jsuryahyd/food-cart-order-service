package orderrepository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	oe "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/entities"
)

type PgOrderRepository struct {
	db     *sql.DB
	logger *logging.Logger
	qb     *db.QueryBuilder
}

func NewOrderRepository(dbConn *sql.DB) *PgOrderRepository {
	return &PgOrderRepository{
		db:     dbConn,
		logger: logging.GetLogger(),
		qb:     db.NewQueryBuilder(dbConn),
	}
}

// CreateOrder inserts a new order and order items using the provided transaction. i.e this is a part of a transaction
// The transaction (tx) must be managed by the caller (e.g., a service).
func (r *PgOrderRepository) CreateOrder(ctx context.Context, tx *sql.Tx, orderDAO *OrderDAO, itemDAOs []*OrderItemDAO) error {
	if tx == nil {
		r.logger.Error("CreateOrder called with nil transaction")
		return fmt.Errorf("transaction is required for creating an order")
	}

	// Ensure order ID is set (if not already)
	if orderDAO.ID == uuid.Nil {
		orderDAO.ID = uuid.New() //we assign IDs at the application level, not DB
	}

	// 1. Insert into orders table
	orderInsertQuery := r.qb.Squirrel.Insert("orders").Columns(
		"id", "total_amount", "coupon_code", "status", "created_at", "updated_at", "user_id", // Added user_id
	).Values(
		orderDAO.ID, orderDAO.Total, orderDAO.CouponCode, orderDAO.Status, orderDAO.CreatedAt, orderDAO.UpdatedAt, orderDAO.UserId,
	).Suffix("RETURNING id").RunWith(tx)

	var insertedOrderID uuid.UUID
	err := orderInsertQuery.QueryRowContext(ctx).Scan(&insertedOrderID)
	if err != nil {
		r.logger.Errorf("Failed to insert order with tx: %v", err)
		return fmt.Errorf("failed to insert order: %w", err)
	}

	// 2. Insert into order_items table
	if len(itemDAOs) > 0 {
		itemInsertBuilder := r.qb.Squirrel.Insert("order_items").Columns(
			"order_id", "product_id", "quantity", "price_at_order_time",
		)
		for _, item := range itemDAOs {

			itemInsertBuilder = itemInsertBuilder.Values(
				orderDAO.ID, item.ProductID, item.Quantity, item.PriceAtPurchase,
			)
		}
		_, err = itemInsertBuilder.RunWith(tx).ExecContext(ctx)
		if err != nil {
			r.logger.Errorf("Failed to insert order items with tx: %v", err)
			return fmt.Errorf("failed to insert order items: %w", err)
		}
	}

	// Commit or Rollback is handled by the service layer.
	return nil
}

// GetOrderById fetches an order and its items by ID.
func (r *PgOrderRepository) GetOrderById(ctx context.Context, id uuid.UUID) (*oe.Order, error) {
	var orderDAO OrderDAO
	var itemDAOs []*OrderItemDAO

	// Fetch order details
	orderQueryBuilder := r.qb.Squirrel.Select(
		"id", "total_amount", "coupon_code", "created_at", "updated_at",
	).From("orders").Where(squirrel.Eq{"id": id})

	err := r.qb.Get(ctx, &orderDAO, orderQueryBuilder)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		r.logger.Errorf("Failed to get order by ID %s: %v", id, err)
		return nil, fmt.Errorf("failed to get order by ID: %w", err)
	}

	// Fetch order items
	itemQueryBuilder := r.qb.Squirrel.Select(
		"id", "order_id", "product_id", "quantity", "price_at_order_time",
	).From("order_items").Where(squirrel.Eq{"order_id": id})

	err = r.qb.Select(ctx, &itemDAOs, itemQueryBuilder)
	if err != nil {
		r.logger.Errorf("Failed to get order items for order %s: %v", id, err)
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}

	return orderDAO.ToDomainModel(itemDAOs), nil
}
