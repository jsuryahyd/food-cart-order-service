package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// implements product_model.PgProductRepository interface
type PgProductRepository struct {
	db *sql.DB
}

func NewProductRepository(dbConn *sql.DB) *PgProductRepository {
	return &PgProductRepository{
		db: dbConn,
	}
}

func (p *PgProductRepository) GetProductByID(ctx context.Context, id uuid.UUID) (*ProductRepository, error) {
	return nil, nil
}
