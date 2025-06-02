package db

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

// TruncateTables truncates all tables in the correct order.
func TruncateTables(ctx context.Context, db *sql.DB, logger *logging.Logger) error {
	logger.Info("Truncating tables before seeding...")
	tables := []string{"order_items", "orders", "product_stock", "products", "categories", "users"}
	for _, table := range tables {
		_, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}
	logger.Info("Tables truncated successfully.")
	return nil
}

// SeedUsers inserts users and returns their IDs.
func SeedUsers(ctx context.Context, tx *sql.Tx, logger *logging.Logger) ([]uuid.UUID, error) {
	userIDs := []uuid.UUID{}
	usersToSeed := []struct {
		Email        string
		PasswordHash string
	}{
		{"user1@example.com", "hashed_pass_user1"},
		{"user2@example.com", "hashed_pass_user2"},
		{"test@example.com", "testpass"},
	}
	for _, user := range usersToSeed {
		userID := uuid.New()
		userIDs = append(userIDs, userID)
		builder := sq.Insert("users").
			Columns("id", "email", "password_hash").
			Values(userID, user.Email, user.PasswordHash)
		sqlStr, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to build user insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sqlStr, args...); err != nil {
			return nil, fmt.Errorf("failed to insert user %s: %w", user.Email, err)
		}
	}
	logger.Infof("Seeded %d users.", len(userIDs))
	return userIDs, nil
}

// SeedCategories inserts categories and returns their IDs.
func SeedCategories(ctx context.Context, tx *sql.Tx, logger *logging.Logger) ([]uuid.UUID, []string, error) {
	categories := []string{"Main Course", "Pizza", "Burgers", "Sides", "Beverages", "Desserts", "Salads", "Appetizers", "Seafood", "Noodles"}
	categoryIDs := []uuid.UUID{}
	for _, name := range categories {
		catId := uuid.New()
		categoryIDs = append(categoryIDs, catId)
		builder := sq.Insert("categories").Columns("id", "name").Values(catId, name)
		sqlStr, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build categories insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sqlStr, args...); err != nil {
			return nil, nil, fmt.Errorf("failed to insert category %s: %w", name, err)
		}
	}
	logger.Infof("Seeded %d categories.", len(categoryIDs))
	return categoryIDs, categories, nil
}

// SeedProductsAndStock inserts products and their stock, returns product IDs.
func SeedProductsAndStock(ctx context.Context, tx *sql.Tx, logger *logging.Logger, categoryIDs []uuid.UUID, categories []string) ([]uuid.UUID, error) {
	productIDs := []uuid.UUID{}
	productsToSeed := 35
	for i := 0; i < productsToSeed; i++ {
		productID := uuid.New()
		productIDs = append(productIDs, productID)
		name := fmt.Sprintf("Product %d %s", i+1, categories[rand.Intn(len(categories))])
		price := float64(rand.Intn(400)+50) + rand.Float64()
		categoryId := categoryIDs[rand.Intn(len(categoryIDs))]
		var deletedAt *time.Time
		if i%8 == 0 {
			t := time.Now().Add(-5 * time.Second)
			deletedAt = &t
		}
		builder := sq.Insert("products").
			Columns("id", "name", "price", "category_id", "deleted_at").
			Values(productID, name, price, categoryId, deletedAt)
		sqlStr, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to build product insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sqlStr, args...); err != nil {
			return nil, fmt.Errorf("failed to insert product %s: %w", name, err)
		}
		stockQuantity := rand.Intn(100) + 10
		stockBuilder := sq.Insert("product_stock").
			Columns("product_id", "quantity").
			Values(productID, stockQuantity)
		stockSql, stockArgs, err := stockBuilder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return nil, fmt.Errorf("failed to build stock insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, stockSql, stockArgs...); err != nil {
			return nil, fmt.Errorf("failed to insert stock for product %s: %w", productID.String(), err)
		}
	}
	logger.Infof("Seeded %d products and their stock.", len(productIDs))
	return productIDs, nil
}

// SeedOrdersAndItems inserts example orders and order items.
func SeedOrdersAndItems(ctx context.Context, tx *sql.Tx, logger *logging.Logger, userIDs, productIDs []uuid.UUID) error {
	if len(userIDs) == 0 || len(productIDs) == 0 {
		return nil
	}
	for i := 0; i < 5; i++ {
		orderID := uuid.New()
		userID := userIDs[rand.Intn(len(userIDs))]
		totalAmount := 0.0
		status := "pending"
		if i%2 == 0 {
			status = "completed"
		}
		numItems := rand.Intn(3) + 1
		perm := rand.Perm(len(productIDs))
		orderItemsBuilder := sq.Insert("order_items").
			Columns("order_id", "product_id", "quantity", "price_at_order_time")
		for j := 0; j < numItems; j++ {
			productIdx := perm[j]
			productID := productIDs[productIdx]
			quantity := rand.Intn(3) + 1
			priceAtOrderTime := float64(rand.Intn(300) + 100)
			totalAmount += priceAtOrderTime * float64(quantity)
			orderItemsBuilder = orderItemsBuilder.Values(orderID, productID, quantity, priceAtOrderTime)
		}
		orderBuilder := sq.Insert("orders").
			Columns("id", "user_id", "total_amount", "status").
			Values(orderID, userID, totalAmount, status)
		orderSql, orderArgs, err := orderBuilder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("failed to build order insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, orderSql, orderArgs...); err != nil {
			return fmt.Errorf("failed to insert order %s: %w", orderID.String(), err)
		}
		orderItemsSql, orderItemsArgs, err := orderItemsBuilder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("failed to build order items insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, orderItemsSql, orderItemsArgs...); err != nil {
			return fmt.Errorf("failed to insert order items for order %s: %w", orderID.String(), err)
		}
	}
	logger.Infof("Seeded 5 example orders.")
	return nil
}

// SeedData runs all seeders in a single transaction.
func SeedData(ctx context.Context, db *sql.DB, logger *logging.Logger) error {
	logger.Info("Starting database seeding...")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for seeding: %w", err)
	}
	defer tx.Rollback()
	userIDs, err := SeedUsers(ctx, tx, logger)
	if err != nil {
		return err
	}
	categoryIDs, categories, err := SeedCategories(ctx, tx, logger)
	if err != nil {
		return err
	}
	productIDs, err := SeedProductsAndStock(ctx, tx, logger, categoryIDs, categories)
	if err != nil {
		return err
	}
	if err := SeedOrdersAndItems(ctx, tx, logger, userIDs, productIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit seed data transaction: %w", err)
	}
	logger.Info("Database seeding complete.")
	return nil
}
