package userrepository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

type User struct {
	Id    uuid.UUID `db:"id"`
	Email string    `db:"email"`
}

type PgUserRepository struct {
	db     *sql.DB
	logger *logging.Logger
}

func NewUserRepository(db *sql.DB) *PgUserRepository {
	return &PgUserRepository{db: db, logger: logging.GetLogger().With("repo", "PgUserRepository")}
}

func (r *PgUserRepository) GetFirstUser(ctx context.Context) (*User, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, email FROM users LIMIT 1")
	var user User
	err := row.Scan(&user.Id, &user.Email)
	if err != nil {
		r.logger.Errorf("GetFirstUser: failed to scan user: %v", err)
		return nil, err
	}
	return &user, nil
}
