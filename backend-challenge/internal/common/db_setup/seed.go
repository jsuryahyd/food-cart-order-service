package db_setup

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	sq "github.com/Masterminds/squirrel" // Alias for convenience
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// In internal/common/db_setup/seed.go or a new utility file
func TruncateTables(ctx context.Context, db *sql.DB, logger *zap.SugaredLogger) error {
	logger.Info("Truncating tables before seeding...")
	// Order matters due to foreign key constraints (truncate children first)
	tables := []string{"order_items", "orders", "stock_inventory", "products", "categories", "users"}
	for _, table := range tables {
		// TRUNCATE ... RESTART IDENTITY would reset sequence if using SERIAL,
		// but we're using UUIDs, so simple TRUNCATE is fine.
		// CASCADE is needed if there are direct foreign key relationships that would be violated
		// when truncating tables that other tables reference.
		_, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}
	logger.Info("Tables truncated successfully.")
	return nil
}

// SeedData inserts initial data into the database.
// This function is for development/testing purposes.
func SeedData(ctx context.Context, db *sql.DB, logger *zap.SugaredLogger) error {
	logger.Info("Starting database seeding...")

	// Use a transaction for atomic seeding
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for seeding: %w", err)
	}
	defer tx.Rollback() // Rollback if any error occurs

	// --- 1. Seed Users ---
	userIDs := []uuid.UUID{}
	usersToSeed := []struct {
		Email        string
		PasswordHash string
	}{
		{"user1@example.com", "hashed_pass_user1"},
		{"user2@example.com", "hashed_pass_user2"},
		{"test@example.com", "testpass"}, // For easy manual testing
	}

	for _, user := range usersToSeed {
		userID := uuid.New()
		userIDs = append(userIDs, userID)
		builder := sq.Insert("users").
			Columns("id", "email", "password_hash").
			Values(userID, user.Email, user.PasswordHash)
		sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("failed to build user insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sql, args...); err != nil {
			return fmt.Errorf("failed to insert user %s: %w", user.Email, err)
		}
	}
	logger.Infof("Seeded %d users.", len(userIDs))

	// --- 2. Seed Products (30+ products, >7 categories) ---
	categories := []string{"Main Course", "Pizza", "Burgers", "Sides", "Beverages", "Desserts", "Salads", "Appetizers", "Seafood", "Noodles"}
	categoryIDs := []uuid.UUID{}
	for _, name := range categories {
		catId := uuid.New()
		categoryIDs = append(categoryIDs, catId)

		builder := sq.Insert("categories").Columns("id", "name").Values(catId, name)
		sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()

		if err != nil {
			return fmt.Errorf("failed to build categories insert query: %w", err)
		}

		if _, err := tx.ExecContext(ctx, sql, args...); err != nil {
			return fmt.Errorf("failed to insert category %s: %w", name, err)
		}
	}

	productIDs := []uuid.UUID{}
	productsToSeed := 35

	// rand.Seed(time.Now().UnixNano()) // Initialize random source

	for i := 0; i < productsToSeed; i++ {
		productID := uuid.New()
		productIDs = append(productIDs, productID)
		name := fmt.Sprintf("Product %d %s", i+1, categories[rand.Intn(len(categories))])
		price := float64(rand.Intn(400)+50) + rand.Float64() // Random price between 50 and 450
		categoryId := categoryIDs[rand.Intn(len(categories))]

		builder := sq.Insert("products").
			Columns("id", "name", "price", "category_id").
			Values(productID, name, price, categoryId)
		sql, args, err := builder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("failed to build product insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sql, args...); err != nil {
			return fmt.Errorf("failed to insert product %s: %w", name, err)
		}

		// Also seed stock inventory for each product
		stockQuantity := rand.Intn(100) + 10 // Quantity between 10 and 110
		stockBuilder := sq.Insert("stock_inventory").
			Columns("product_id", "quantity").
			Values(productID, stockQuantity)
		stockSql, stockArgs, err := stockBuilder.PlaceholderFormat(sq.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("failed to build stock insert query: %w", err)
		}
		if _, err := tx.ExecContext(ctx, stockSql, stockArgs...); err != nil {
			return fmt.Errorf("failed to insert stock for product %s: %w", productID.String(), err)
		}
	}
	logger.Infof("Seeded %d products and their stock.", len(productIDs))

	// --- 3. Seed Orders (optional for initial seed, but good for testing) ---
	if len(userIDs) > 0 && len(productIDs) > 0 {
		for i := 0; i < 5; i++ { // Seed 5 example orders
			orderID := uuid.New()
			userID := userIDs[rand.Intn(len(userIDs))]
			totalAmount := 0.0
			status := "pending"
			if i%2 == 0 {
				status = "completed"
			} // Alternate status

			// Create order items for this order
			numItems := rand.Intn(3) + 1 // 1 to 3 items per order
			orderItemsBuilder := sq.Insert("order_items").
				Columns("order_id", "product_id", "quantity", "price_at_order_time")

			for j := 0; j < numItems; j++ {
				productIdx := rand.Intn(len(productIDs))
				productID := productIDs[productIdx]
				quantity := rand.Intn(3) + 1 // 1 to 3 of each item

				// We need to fetch the actual price here if we were doing this properly
				// For seed data, we can mock it or retrieve from our temporary product list
				// For simplicity, let's assume average product price is 200
				priceAtOrderTime := float64(rand.Intn(300) + 100) // Example price
				totalAmount += priceAtOrderTime * float64(quantity)

				orderItemsBuilder = orderItemsBuilder.Values(orderID, productID, quantity, priceAtOrderTime)
			}

			// Insert the order first
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

			// Insert order items
			orderItemsSql, orderItemsArgs, err := orderItemsBuilder.PlaceholderFormat(sq.Dollar).ToSql()
			if err != nil {
				return fmt.Errorf("failed to build order items insert query: %w", err)
			}
			if _, err := tx.ExecContext(ctx, orderItemsSql, orderItemsArgs...); err != nil {
				return fmt.Errorf("failed to insert order items for order %s: %w", orderID.String(), err)
			}
		}
		logger.Infof("Seeded 5 example orders.")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit seed data transaction: %w", err)
	}
	logger.Info("Database seeding complete.")
	return nil
}
