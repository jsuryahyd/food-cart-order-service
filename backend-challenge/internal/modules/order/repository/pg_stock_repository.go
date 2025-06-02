package orderrepository

import (
	"context"
	"errors"
	"fmt"

	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

type PgStockRepository struct {
	db     *sql.DB
	logger *logging.Logger
	qb     *db.QueryBuilder
}

// NewPgStockRepository creates a new PgStockRepository.
func NewPgStockRepository(dbConn *sql.DB) *PgStockRepository {
	return &PgStockRepository{
		db:     dbConn,
		logger: logging.GetLogger(),
		qb:     db.NewQueryBuilder(dbConn),
	}
}

/*
*
- Can Run as part of a transaction and also as independant query
*/
func (r *PgStockRepository) GetProductStock(ctx context.Context, tx *sql.Tx, productID uuid.UUID) (int, error) {
	var quantity int
	queryBuilder := r.qb.Squirrel.Select("quantity").
		From("product_stock").
		Where(squirrel.Eq{"product_id": productID})

		//todo: use wrapper around squirrel
	var row squirrel.RowScanner
	if tx != nil {
		// "FOR UPDATE" is used only when within a transaction to lock the row.
		row = queryBuilder.Suffix("FOR UPDATE").RunWith(tx).QueryRowContext(ctx)
	} else {
		row = queryBuilder.RunWith(r.db).QueryRowContext(ctx)
	}

	err := row.Scan(&quantity) // using row.Scan instead of struct mapping, as we do not have a DAO for stock.
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Warnf("No stock record found for product %s", productID)
			return 0, fmt.Errorf("stock record for product %s not found: %w", productID, apperrors.ErrNotFound)
		}
		r.logger.Errorf("Failed to get stock for product %s: %v", productID, err)
		return 0, fmt.Errorf("failed to retrieve product stock: %w", err)
	}
	return quantity, nil
}

/*
*
This operation must be performed strictly within a transaction.
reduces the stock of a product, also ensuring it doesn't go below zero.
*/
func (r *PgStockRepository) ReduceProductStock(ctx context.Context, tx *sql.Tx, productID uuid.UUID, quantity int) error {
	if tx == nil {
		return fmt.Errorf("transaction is required for stock reduction")
	}

	updateQuery := r.qb.Squirrel.Update("product_stock").
		Set("quantity", squirrel.Expr("quantity - ?", quantity)).
		Where(squirrel.Eq{"product_id": productID}).
		Where(squirrel.Expr("quantity >= ?", quantity)) // Ensure non-negative after update

	result, err := updateQuery.RunWith(tx).ExecContext(ctx)
	if err != nil {
		r.logger.Errorf("Failed to reduce stock for product %s by %d: %v", productID, quantity, err)
		return fmt.Errorf("failed to reduce product stock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Errorf("Failed to get rows affected for stock reduction: %v", err)
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// This means either product_id was not found or quantity was less than requested quantity
		return fmt.Errorf("either product_id was not found: %s or quantity was less than requested quantity: %d %w", productID, quantity, apperrors.ErrInvalidInput)
	}
	return nil
}
