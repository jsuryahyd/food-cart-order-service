package repository

import (
	"time"

	"github.com/google/uuid"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
)

// type Category struct {
// 	Id   uuid.UUID `db:"id"`
// 	Name string    `db:"name"`
// }

type ProductDAO struct {
	ID    uuid.UUID `db:"id"`
	Name  string    `db:"name"`
	Price float64   `db:"price"`
	// Category  Category   `db:"-"` //todo: figure out mapping joins to nested structs
	CategoryId   uuid.UUID  `db:"category_id"`
	CategoryName string     `db:"category_name"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"` //ptr used to allow null (nil) values
}

func (p *ProductDAO) ToDomainModel() *pe.Product {
	if p == nil {
		return nil
	}
	isActive := true
	if p.DeletedAt != nil {
		isActive = p.DeletedAt.IsZero()
	}
	return &pe.Product{
		Id:    p.ID,
		Name:  p.Name,
		Price: p.Price,
		Category: struct {
			Id   uuid.UUID
			Name string
		}{Id: p.CategoryId, Name: p.CategoryName},
		CreatedAt: p.CreatedAt,
		IsActive:  isActive,
	}
}

// Not needed for the task
func FromDomain(p *pe.Product) *ProductDAO {
	return nil
}
