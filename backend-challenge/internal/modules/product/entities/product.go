package entities

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	Id       uuid.UUID
	Name     string
	Category struct {
		Id   uuid.UUID
		Name string
	}
	Price     float64
	CreatedAt time.Time
	IsActive  bool
}
