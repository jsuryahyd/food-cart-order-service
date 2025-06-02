package userrepository

import (
	"context"
)

type UserRepository interface {
	GetFirstUser(ctx context.Context) (*User, error)
}
