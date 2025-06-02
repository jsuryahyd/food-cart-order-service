package orderrepository

import (
	"context"

	"database/sql"

	"github.com/google/uuid"
)

type StockRepository interface {
	GetProductStock(ctx context.Context, tx *sql.Tx, productID uuid.UUID) (int, error)
	ReduceProductStock(ctx context.Context, tx *sql.Tx, productID uuid.UUID, quantity int) error
}
