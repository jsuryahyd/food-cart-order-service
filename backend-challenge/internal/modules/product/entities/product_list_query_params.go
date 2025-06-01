package entities

import "github.com/google/uuid"

type ProductListQueryParams struct {
	Limit  int
	Offset int

	CategoryIDs []uuid.UUID
	Name        string

	SortBy    string
	SortOrder string

	IncludeDeleted bool
}

func NewProductListQueryParams() *ProductListQueryParams {

	return &ProductListQueryParams{
		Limit:  10,
		Offset: 0,
	}
}

func (qp *ProductListQueryParams) AddLimit(limit int) *ProductListQueryParams {
	if limit > 0 {
		qp.Limit = limit
	}
	return qp
}

func (qp *ProductListQueryParams) AddOffset(offset int) *ProductListQueryParams {
	if offset >= 0 {
		qp.Offset = offset
	}
	return qp
}
func (qp *ProductListQueryParams) AddCategoryIDs(categoryIDs []uuid.UUID) *ProductListQueryParams {

	qp.CategoryIDs = append(qp.CategoryIDs, categoryIDs...)

	return qp
}
func (qp *ProductListQueryParams) AddProductName(name string) *ProductListQueryParams {
	if len(name) > 0 {
		qp.Name = name
	}
	return qp
}
func (qp *ProductListQueryParams) AddSortBy(sortBy string) *ProductListQueryParams {

	qp.SortBy = sortBy

	return qp
}
func (qp *ProductListQueryParams) AddSortOrder(sortOrder string) *ProductListQueryParams {
	if sortOrder == "asc" || sortOrder == "desc" {

		qp.SortOrder = sortOrder
	}

	return qp
}
