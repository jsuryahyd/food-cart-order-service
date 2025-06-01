package productdto

import (
	"time"

	"github.com/google/uuid"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
)

// single product item in response
type ProductResponse struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
}

func NewProductResponseFromEntity(entity *pe.Product) *ProductResponse {
	if entity == nil {
		return nil
	}
	return &ProductResponse{
		Id:        entity.Id,
		Name:      entity.Name,
		Price:     entity.Price,
		Category:  entity.Category.Name,
		CreatedAt: entity.CreatedAt,
		IsActive:  entity.IsActive,
	}
}

func NewProductResponseListFromEntities(entities []*pe.Product) []*ProductResponse {
	if entities == nil {
		return nil
	}
	responses := make([]*ProductResponse, len(entities))
	for i, entity := range entities {
		responses[i] = NewProductResponseFromEntity(entity)
	}
	return responses
}
