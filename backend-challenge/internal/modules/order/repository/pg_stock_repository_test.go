package orderrepository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/apperrors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/testutil"

	orderrepo "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/repository"
)

var (
	testDBOrder *sql.DB
	pgOrderRepo orderrepo.OrderRepository // Use the interface
	loggerOrder *logging.Logger
)

func TestMain(m *testing.M) {
	var cleanup func()
	testDBOrder, loggerOrder, cleanup = testutil.SetupIntegrationTest(m, "pg_order_repository_test")
	pgOrderRepo = orderrepo.NewOrderRepository(testDBOrder) // Instantiate your concrete type

	exitCode := m.Run()
	cleanup()
	os.Exit(exitCode)
}

// Helper function to seed a user if your orders table has a user_id FK
func seedUserForOrderTests(t *testing.T, ctx context.Context, userID uuid.UUID, userName string) {
	// Assuming a 'users' table: id (uuid), name (text), api_key (text), role (text)
	// Add other necessary fields for your users table
	_, err := testDBOrder.ExecContext(ctx,
		"INSERT INTO users (id, name, api_key, role) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
		userID, userName, fmt.Sprintf("apikey-%s", userName), "customer",
	)
	require.NoError(t, err, "Failed to seed user for order tests")
}

// Helper function to seed products for FK constraints if order_items.product_id needs it
func seedProductsForOrderTests(t *testing.T, ctx context.Context, productIDs ...uuid.UUID) {
	categoryID := uuid.New() // Create a dummy category
	_, err := testDBOrder.ExecContext(ctx, "INSERT INTO categories (id, name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING", categoryID, "Test Category for Orders")
	require.NoError(t, err)

	for _, productID := range productIDs {
		_, err := testDBOrder.ExecContext(ctx,
			"INSERT INTO products (id, name, price, category_id) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
			productID, fmt.Sprintf("Product %s", productID.String()[:4]), 10.00, categoryID,
		)
		require.NoError(t, err, "Failed to seed product for order tests")
	}
}

func TestPgOrderRepository_CreateOrder(t *testing.T) {
	ctx := context.Background()

	// Cleanup tables after each test case for isolation
	t.Cleanup(func() {
		err := db.TruncateTables(context.Background(), testDBOrder, loggerOrder)
		require.NoError(t, err)
	})

	userID := uuid.New()
	productID1 := uuid.New()
	productID2 := uuid.New()

	// Seed prerequisites
	seedUserForOrderTests(t, ctx, userID, "Order Test User")
	seedProductsForOrderTests(t, ctx, productID1, productID2)

	t.Run("successfully create an order with items", func(t *testing.T) {
		orderID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond) // Consistent time

		orderDAO := &orderrepo.OrderDAO{
			ID:         orderID,
			Total:      35.75,
			UserId:     userID,
			Status:     "PENDING",
			CouponCode: nil, // Example: No coupon
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		itemDAOs := []*orderrepo.OrderItemDAO{
			{OrderID: orderID, ProductID: productID1, Quantity: 2, PriceAtPurchase: 12.50},
			{OrderID: orderID, ProductID: productID2, Quantity: 1, PriceAtPurchase: 10.75},
		}

		tx, err := testDBOrder.BeginTx(ctx, nil)
		require.NoError(t, err)

		err = pgOrderRepo.CreateOrder(ctx, tx, orderDAO, itemDAOs)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verification
		var dbOrder orderrepo.OrderDAO
		err = testDBOrder.QueryRowContext(ctx, "SELECT id, total, user_id, status, created_at, updated_at FROM orders WHERE id = $1", orderID).Scan(
			&dbOrder.ID, &dbOrder.Total, &dbOrder.UserId, &dbOrder.Status, &dbOrder.CreatedAt, &dbOrder.UpdatedAt,
		)
		require.NoError(t, err)
		assert.Equal(t, orderDAO.ID, dbOrder.ID)
		assert.Equal(t, orderDAO.Total, dbOrder.Total)
		assert.Equal(t, orderDAO.UserId, dbOrder.UserId)
		assert.Equal(t, orderDAO.Status, dbOrder.Status)
		assert.WithinDuration(t, orderDAO.CreatedAt, dbOrder.CreatedAt, time.Second)

		var itemCount int
		err = testDBOrder.QueryRowContext(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", orderID).Scan(&itemCount)
		require.NoError(t, err)
		assert.Equal(t, len(itemDAOs), itemCount)
	})

	t.Run("successfully create an order with a coupon code", func(t *testing.T) {
		orderID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)
		coupon := "SUMMER10"

		orderDAO := &orderrepo.OrderDAO{
			ID:         orderID,
			Total:      22.50,
			UserId:     userID,
			Status:     "PENDING",
			CouponCode: &coupon,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		itemDAOs := []*orderrepo.OrderItemDAO{
			{OrderID: orderID, ProductID: productID1, Quantity: 2, PriceAtPurchase: 12.50},
		}

		tx, err := testDBOrder.BeginTx(ctx, nil)
		require.NoError(t, err)

		err = pgOrderRepo.CreateOrder(ctx, tx, orderDAO, itemDAOs)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		var dbCouponCode sql.NullString
		err = testDBOrder.QueryRowContext(ctx, "SELECT coupon_code FROM orders WHERE id = $1", orderID).Scan(&dbCouponCode)
		require.NoError(t, err)
		require.True(t, dbCouponCode.Valid)
		assert.Equal(t, coupon, dbCouponCode.String)
	})

	t.Run("fail if transaction is nil", func(t *testing.T) {
		orderDAO := &orderrepo.OrderDAO{ID: uuid.New()}
		err := pgOrderRepo.CreateOrder(ctx, nil, orderDAO, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transaction is required")
	})

	t.Run("create order with no items", func(t *testing.T) {
		orderID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)
		orderDAO := &orderrepo.OrderDAO{
			ID: orderID, Total: 0, UserId: userID, Status: "EMPTY", CreatedAt: now, UpdatedAt: now,
		}
		var itemDAOs []*orderrepo.OrderItemDAO // Empty slice

		tx, err := testDBOrder.BeginTx(ctx, nil)
		require.NoError(t, err)

		err = pgOrderRepo.CreateOrder(ctx, tx, orderDAO, itemDAOs)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		var itemCount int
		err = testDBOrder.QueryRowContext(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", orderID).Scan(&itemCount)
		require.NoError(t, err)
		assert.Equal(t, 0, itemCount)
	})
}

func TestPgOrderRepository_GetOrderById(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() {
		err := db.TruncateTables(context.Background(), testDBOrder, loggerOrder)
		require.NoError(t, err)
	})

	userID := uuid.New()
	productID1 := uuid.New()
	seedUserForOrderTests(t, ctx, userID, "OrderGet User")
	seedProductsForOrderTests(t, ctx, productID1)

	t.Run("successfully get an existing order", func(t *testing.T) {
		orderID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)
		coupon := "GETME15"

		// Seed data directly or via CreateOrder
		orderDAO := &orderrepo.OrderDAO{
			ID: orderID, Total: 12.50, UserId: userID, Status: "COMPLETED", CouponCode: &coupon, CreatedAt: now, UpdatedAt: now,
		}
		itemDAOs := []*orderrepo.OrderItemDAO{
			{OrderID: orderID, ProductID: productID1, Quantity: 1, PriceAtPurchase: 12.50},
		}
		tx, _ := testDBOrder.BeginTx(ctx, nil)
		err := pgOrderRepo.CreateOrder(ctx, tx, orderDAO, itemDAOs) // Assuming CreateOrder also seeds UserName
		require.NoError(t, err)
		// Manually update UserName in the DAO as CreateOrder doesn't populate it from user table for the DAO
		orderDAO.UserName = "OrderGet User"
		tx.Commit()

		fetchedOrder, err := pgOrderRepo.GetOrderById(ctx, orderID)
		require.NoError(t, err)
		require.NotNil(t, fetchedOrder)

		assert.Equal(t, orderDAO.ID, fetchedOrder.Id)
		assert.Equal(t, orderDAO.Total, fetchedOrder.Total)
		assert.Equal(t, orderDAO.UserId, fetchedOrder.User.Id)
		// The DAO in pg_order_repository.go GetOrderById doesn't currently join with users table to get UserName.
		// So, fetchedOrder.User.Name might be empty unless ToDomainModel is enhanced or query joins users.
		// Let's assume for now it's not populated by GetOrderById directly.
		// If it *is* populated by your GetOrderById by joining users, then assert it:
		// assert.Equal(t, orderDAO.UserName, fetchedOrder.User.Name)
		assert.Equal(t, orderDAO.Status, fetchedOrder.Status)
		assert.Equal(t, *orderDAO.CouponCode, fetchedOrder.CouponCode)
		assert.WithinDuration(t, orderDAO.CreatedAt, fetchedOrder.CreatedAt, time.Second)
		require.Len(t, fetchedOrder.Items, 1)
		assert.Equal(t, itemDAOs[0].ProductID, fetchedOrder.Items[0].ProductId)
		assert.Equal(t, itemDAOs[0].Quantity, fetchedOrder.Items[0].Quantity)
	})

	t.Run("return ErrNotFound for non-existent order ID", func(t *testing.T) {
		nonExistentID := uuid.New()
		_, err := pgOrderRepo.GetOrderById(ctx, nonExistentID)
		require.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrNotFound)
	})

	t.Run("get order with no items", func(t *testing.T) {
		orderID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)
		orderDAO := &orderrepo.OrderDAO{
			ID: orderID, Total: 0.0, UserId: userID, Status: "CREATED", CreatedAt: now, UpdatedAt: now,
		}
		var emptyItemDAOs []*orderrepo.OrderItemDAO

		tx, _ := testDBOrder.BeginTx(ctx, nil)
		err := pgOrderRepo.CreateOrder(ctx, tx, orderDAO, emptyItemDAOs)
		require.NoError(t, err)
		tx.Commit()

		fetchedOrder, err := pgOrderRepo.GetOrderById(ctx, orderID)
		require.NoError(t, err)
		require.NotNil(t, fetchedOrder)
		assert.Len(t, fetchedOrder.Items, 0)
		assert.Equal(t, orderID, fetchedOrder.Id)

	})
}
