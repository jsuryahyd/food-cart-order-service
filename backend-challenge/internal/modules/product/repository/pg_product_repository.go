package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
)

// implements product_model.PgProductRepository interface
type PgProductRepository struct {
	db     *sql.DB
	logger *logging.Logger
	qb     *db.QueryBuilder
}

func NewProductRepository(dbConn *sql.DB) *PgProductRepository {
	return &PgProductRepository{
		db:     dbConn,
		logger: logging.GetLogger(),
		qb:     db.NewQueryBuilder(dbConn),
	}
}

func (p *PgProductRepository) GetProductByID(ctx context.Context, id uuid.UUID, options Options) (*ProductDAO, error) {

	var productDetails = ProductDAO{}

	conditions := []db.Sqlizer{
		db.Eq{"p.id": id},
	}
	if !options.IncludeDeleted {
		conditions = append(conditions, db.Eq{"p.deleted_at": nil})
	}

	queryBuilder := p.qb.Squirrel.Select(
		"p.id", "p.name", "p.price", "p.created_at", "p.updated_at", "p.deleted_at",
		"c.id AS category_id", "c.name AS category_name",
	).From("products p").
		Join("categories c ON p.category_id = c.id").Where(db.And(conditions))

	err := p.qb.Get(ctx, &productDetails, queryBuilder)
	if err != nil {
		return nil, err
	}
	return &productDetails, nil
}

func (p *PgProductRepository) GetListOfProducts(ctx context.Context, params *pe.ProductListQueryParams) ([]*ProductDAO, error) {
	var productDAOs []*ProductDAO

	conditions := []db.Sqlizer{}
	if !params.IncludeDeleted {
		conditions = append(conditions, db.Eq{"p.deleted_at": nil})
	}
	if params.Name != "" {
		conditions = append(conditions, db.ILike{"p.name": "%" + params.Name + "%"})
	}
	if params.CategoryIDs != nil {
		conditions = append(conditions, db.Eq{"p.category_id": params.CategoryIDs})
	}

	queryBuilder := p.qb.Squirrel.Select(
		"p.id", "p.name", "p.price", "p.created_at", "p.updated_at", "p.deleted_at",
		"c.id AS category_id", "c.name AS category_name",
	).From("products p").
		Join("categories c ON p.category_id = c.id")

	if len(conditions) > 0 {
		queryBuilder = queryBuilder.Where(db.And(conditions))
	}

	// Sorting
	if params.SortBy != "" {
		order := "ASC"
		if params.SortOrder == "desc" {
			order = "DESC"
		}
		queryBuilder = queryBuilder.OrderBy("p." + params.SortBy + " " + order)
	}

	// Pagination
	if params.Limit > 0 {
		queryBuilder = queryBuilder.Limit(uint64(params.Limit))
		if params.Offset > 0 {
			queryBuilder = queryBuilder.Offset(uint64(params.Offset))
		}
	}

	err := p.qb.Select(ctx, &productDAOs, queryBuilder)
	if err != nil {
		return nil, err
	}

	return productDAOs, nil
}
