// internal/modules/product/service/product_service.go (Updated)
package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
	pr "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
)

type ProductService struct {
	repo pr.ProductRepository
}

func NewProductService(repo pr.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) GetProductByID(ctx context.Context, id uuid.UUID, options pr.Options) (*pe.Product, error) {

	if id == uuid.Nil {
		return nil, apperrors.ErrInvalidInput
	}

	dao, err := s.repo.GetProductByID(ctx, id, options)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	if dao == nil {
		return nil, apperrors.ErrNotFound
	}
	return dao.ToDomainModel(), nil
}

func (s *ProductService) GetListOfProducts(ctx context.Context, params *pe.ProductListQueryParams) ([]*pe.Product, error) {

	if params.Limit < 0 || params.Offset < 0 {
		return nil, apperrors.ErrInvalidInput
	}

	validSortBys := map[string]bool{"id": true, "name": true, "price": true, "created_at": true, "updated_at": true}
	if params.SortBy != "" && !validSortBys[params.SortBy] {
		return nil, apperrors.ErrInvalidInput
	}

	if params.SortOrder != "" && params.SortOrder != "asc" && params.SortOrder != "desc" {
		return nil, apperrors.ErrInvalidInput
	}

	daos, err := s.repo.GetListOfProducts(ctx, params)
	if err != nil {
		return nil, err
	}
	products := make([]*pe.Product, 0, len(daos))
	for _, dao := range daos {
		products = append(products, dao.ToDomainModel())
	}
	return products, nil
}
