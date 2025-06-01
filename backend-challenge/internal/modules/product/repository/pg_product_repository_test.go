package repository_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/testutil"
	pr "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testDBConn  *sql.DB
	productRepo *pr.PgProductRepository
	logger      *logging.Logger
	categories  []map[string]any
	products    []map[string]any
)

func TestMain(m *testing.M) {
	_testDBConn, _logger, cleanup := testutil.SetupIntegrationTest(m, "pg_product_repository_test")
	testDBConn = _testDBConn
	logger = _logger
	productRepo = pr.NewProductRepository(testDBConn)
	exitCode := m.Run()
	cleanup()
	os.Exit(exitCode)
}

func SetupTest(t *testing.T) {
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		err := db.TruncateTables(ctx, testDBConn, logger)
		require.NoError(t, err, "Failed to truncate tables during cleanup")

	})
}

func seedData(t *testing.T, ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := db.TruncateTables(ctx, testDBConn, logger)
	require.NoError(t, err, "Failed to truncate tables during cleanup")
	categories = []map[string]any{
		{
			"id":   uuid.New(),
			"name": "Category X",
		},
		{
			"id":   uuid.New(),
			"name": "Category Y",
		},
	}

	products = []map[string]any{
		{
			"id":         uuid.New(),
			"name":       "Product A",
			"price":      12.50,
			"categoryId": categories[0]["id"],
			"createdAt":  time.Now().AddDate(0, 0, -3), // 3 days ago
		},
		{
			"id":         uuid.New(),
			"name":       "Product B",
			"price":      15.75,
			"categoryId": categories[0]["id"],
			"createdAt":  time.Now().AddDate(0, 0, -2),
		},
		{
			"id":         uuid.New(),
			"name":       "Product C",
			"price":      8.99,
			"categoryId": categories[1]["id"],
			"createdAt":  time.Now().AddDate(0, 0, -1),
		},
	}

	_, err = testDBConn.ExecContext(ctx, "INSERT INTO categories (id, name) VALUES ($1, $2),($3,$4)", categories[0]["id"], categories[0]["name"], categories[1]["id"], categories[1]["name"])
	require.NoError(t, err, "Failed to add seed categories")
	sqlInsert := `
	INSERT INTO products (id, name, price, category_id, created_at) VALUES
		($1, $2, $3, $4, $5),
		($6, $7, $8, $9, $10),
		($11, $12, $13, $14, $15)
	`
	_, err = testDBConn.ExecContext(ctx, sqlInsert,
		products[0]["id"], products[0]["name"], products[0]["price"], products[0]["categoryId"], products[0]["createdAt"],
		products[1]["id"], products[1]["name"], products[1]["price"], products[1]["categoryId"], products[1]["createdAt"],
		products[2]["id"], products[2]["name"], products[2]["price"], products[2]["categoryId"], products[2]["createdAt"],
	)

	require.NoError(t, err, "failed to add seed products")
}

func Test_GetProductByID(t *testing.T) {
	SetupTest(t)
	ctx := context.Background()

	t.Run("should return (correct) product Id if exists", func(t *testing.T) {
		seedData(t, ctx) //todo: alternatively, setup test only once, and use transactions for each sub test
		selectedProduct := products[1]
		selectedCategory := categories[0] //in the above mocks these are associated.
		product, err := productRepo.GetProductByID(ctx, selectedProduct["id"].(uuid.UUID), pr.Options{})
		require.NoError(t, err)
		assert.NotNil(t, product)
		assert.Equal(t, product.ID, selectedProduct["id"])
		assert.Equal(t, product.Name, selectedProduct["name"])
		assert.Equal(t, product.Price, selectedProduct["price"])
		assert.Equal(t, product.CategoryId, selectedProduct["categoryId"])
		assert.Equal(t, product.CategoryName, selectedCategory["name"])
		require.WithinDuration(t, product.CreatedAt.UTC(), selectedProduct["createdAt"].(time.Time).UTC(), time.Second)
	})

	t.Run("Should return sql no rows error if non existing uuid is passed", func(t *testing.T) {
		seedData(t, ctx)
		_, err := productRepo.GetProductByID(ctx, uuid.New(), pr.Options{})
		require.ErrorIs(t, err, sql.ErrNoRows)

	})

	t.Run("Should return sql no rows error if element is deleted", func(t *testing.T) {
		seedData(t, ctx)
		// Soft delete the product by setting deleted_at
		now := time.Now().AddDate(0, 0, -1)
		prodID := products[2]["id"].(uuid.UUID)
		_, err := testDBConn.ExecContext(ctx, "UPDATE products SET deleted_at = $1 WHERE id = $2", now, prodID)
		require.NoError(t, err, "Failed to soft delete product")

		_, err = productRepo.GetProductByID(ctx, prodID, pr.Options{})
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Should return deleted item if options contain includeDeleted", func(t *testing.T) {
		seedData(t, ctx)
		// Soft delete the product by setting deleted_at
		now := time.Now()
		selectedProduct := products[2]
		selectedCategory := categories[1]
		_, err := testDBConn.ExecContext(ctx, "UPDATE products SET deleted_at = $1 WHERE id = $2", now, selectedProduct["id"].(uuid.UUID))
		require.NoError(t, err, "Failed to soft delete product")

		product, err := productRepo.GetProductByID(ctx, selectedProduct["id"].(uuid.UUID), pr.Options{IncludeDeleted: true})
		require.NoError(t, err)
		assert.NotNil(t, product)
		assert.Equal(t, product.ID, selectedProduct["id"])
		assert.Equal(t, product.Name, selectedProduct["name"])
		assert.Equal(t, product.Price, selectedProduct["price"])
		assert.Equal(t, product.CategoryId, selectedProduct["categoryId"])
		assert.Equal(t, product.CategoryName, selectedCategory["name"])
		require.WithinDuration(t, product.CreatedAt.UTC(), selectedProduct["createdAt"].(time.Time).UTC(), time.Second)
	})

	t.Run("Should return error on db error", func(t *testing.T) {
		t.Skip("Skipping the test with todos")
		seedData(t, ctx)
		// Simulate DB error by closing the connection
		testDBConn.Close()                            //todo: this will effect other tests.
		badRepo := pr.NewProductRepository(&sql.DB{}) //todo: this causes nil reference error
		_, err := badRepo.GetProductByID(ctx, products[0]["id"].(uuid.UUID), pr.Options{})
		require.Error(t, err)
	})

	t.Run("Should return error if invalid UUID is passed", func(t *testing.T) {
		seedData(t, ctx)
		//No implementation needed, validation will be handled by service/handler.
		zeroUUID := uuid.UUID{}
		_, err := productRepo.GetProductByID(ctx, zeroUUID, pr.Options{})
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

}
