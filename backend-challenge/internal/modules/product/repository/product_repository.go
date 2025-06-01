package repository

import (
	"context"

	"github.com/google/uuid"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
)

type Options struct {
	IncludeDeleted bool
}

type ProductRepository interface {
	GetProductByID(ctx context.Context, id uuid.UUID, options Options) (*pe.Product, error)

	GetListOfProducts(ctx context.Context, params *pe.ProductListQueryParams) ([]*pe.Product, error)

	// Not needed for the task
	// CreateProduct(ctx context.Context, product *model.Product) error

	// UpdateProduct(ctx context.Context, product *model.Product) error

	// DeleteProduct(ctx context.Context, id string) error
}
