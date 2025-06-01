package repository

import (
	"time"

	"github.com/google/uuid"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
)

type CategoryDetails struct {
	Id   uuid.UUID `db:"id"`
	Name string    `db:"name"`
}

type ProductDAO struct {
	ID        uuid.UUID       `db:"id"`
	Name      string          `db:"name"`
	Price     float64         `db:"price"`
	Category  CategoryDetails `db:"category"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
	DeletedAt time.Time       `db:"deleted_at"`
}

func (p *ProductDAO) ToDomainModel() *pe.Product {
	if p == nil {
		return nil
	}

	return &pe.Product{
		Id:    p.ID,
		Name:  p.Name,
		Price: p.Price,
		Category: struct {
			Id   uuid.UUID
			Name string
		}{p.Category.Id, p.Category.Name},
		CreatedAt: p.CreatedAt,
		IsActive:  p.DeletedAt.IsZero(),
	}
}

// Not needed for the task
func FromDomain(p *pe.Product) *ProductDAO {
	return nil
}
