// internal/modules/product/service/product_service_test.go
package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
	pr "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/product/service"
)

// mock implementation of pr.ProductRepository
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) GetProductByID(ctx context.Context, id uuid.UUID, options pr.Options) (*pr.ProductDAO, error) {
	args := m.Called(ctx, id, options)

	//do not type-assert the first return value as DAO, if it is nil (happens in the cases when we mock the call to return error)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pr.ProductDAO), args.Error(1)
}

func (m *MockProductRepository) GetListOfProducts(ctx context.Context, params *pe.ProductListQueryParams) ([]*pr.ProductDAO, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*pr.ProductDAO), args.Error(1)
}

func TestProductService_GetProductByID(t *testing.T) {
	mockRepo := new(MockProductRepository)
	productService := service.NewProductService(mockRepo)
	ctx := context.Background() // Use a background context for simplicity in tests

	// ==== Test Data ====
	// Define common data for mock responses
	sampleCategoryID := uuid.New()
	sampleCategoryName := "Beverages"

	// An active product (not soft deleted)
	activeProductDAO := &pr.ProductDAO{
		ID:           uuid.New(),
		Name:         "Fresh Coffee",
		Price:        3.50,
		CategoryId:   sampleCategoryID,
		CategoryName: sampleCategoryName,
		CreatedAt:    time.Now().AddDate(0, 0, -3),
		UpdatedAt:    time.Now().AddDate(0, 0, -1),
		DeletedAt:    nil,
	}
	activeProductEntity := activeProductDAO.ToDomainModel()

	// A product that is soft-deleted
	deletedAt := time.Now().AddDate(0, 0, -2)
	deletedProductDAO := &pr.ProductDAO{
		ID:           uuid.New(),
		Name:         "Old Smoothie",
		Price:        2.00,
		CategoryId:   sampleCategoryID,
		CategoryName: sampleCategoryName,
		CreatedAt:    time.Now().AddDate(0, 0, -5),
		UpdatedAt:    time.Now().AddDate(0, 0, -3),
		DeletedAt:    &deletedAt, // Pointer to time for non-null DeletedAt
	}
	// Convert to entity for comparison
	deletedProductEntity := deletedProductDAO.ToDomainModel()

	t.Run("should return active product if found and not deleted", func(t *testing.T) {
		options := pr.Options{IncludeDeleted: false} // Default option to not include deleted
		mockRepo.On("GetProductByID", ctx, activeProductDAO.ID, options).Return(activeProductDAO, nil).Once()

		product, err := productService.GetProductByID(ctx, activeProductDAO.ID, options)
		require.NoError(t, err)
		assert.NotNil(t, product)
		assert.Equal(t, activeProductEntity.Id, product.Id)
		assert.Equal(t, activeProductEntity.Name, product.Name)
		assert.Equal(t, activeProductEntity.Price, product.Price)
		assert.Equal(t, activeProductEntity.Category.Id, product.Category.Id)
		assert.Equal(t, activeProductEntity.Category.Name, product.Category.Name)
		assert.True(t, product.IsActive) // Should be active

		/**
		assertions:
		- the method passed to mockRepo.on(method, args...) was called
		- called with the args passed
		*/
		mockRepo.AssertExpectations(t)
	})

	t.Run("should return deleted product if found and IncludeDeleted is true", func(t *testing.T) {
		options := pr.Options{IncludeDeleted: true} // Explicitly include deleted products
		mockRepo.On("GetProductByID", ctx, deletedProductDAO.ID, options).Return(deletedProductDAO, nil).Once()

		product, err := productService.GetProductByID(ctx, deletedProductDAO.ID, options)
		require.NoError(t, err)
		assert.NotNil(t, product)
		assert.Equal(t, deletedProductEntity.Id, product.Id)
		assert.False(t, product.IsActive) // Should be inactive
		mockRepo.AssertExpectations(t)
	})

	t.Run("should return ErrNotFound if product does not exist (sql.ErrNoRows)", func(t *testing.T) {
		nonExistentID := uuid.New()
		options := pr.Options{IncludeDeleted: false}
		mockRepo.On("GetProductByID", ctx, nonExistentID, options).Return(nil, sql.ErrNoRows).Once()

		product, err := productService.GetProductByID(ctx, nonExistentID, options)
		assert.Nil(t, product)
		assert.ErrorIs(t, err, apperrors.ErrNotFound) // Service should translate sql.ErrNoRows
		mockRepo.AssertExpectations(t)
	})

	t.Run("should farword other unhandled errors from repository", func(t *testing.T) {
		repoErrorID := uuid.New()
		options := pr.Options{IncludeDeleted: false}
		expectedRepoError := errors.New("database connection failed unexpectedly")
		mockRepo.On("GetProductByID", ctx, repoErrorID, options).Return(nil, expectedRepoError).Once()

		product, err := productService.GetProductByID(ctx, repoErrorID, options)
		assert.Nil(t, product)
		assert.ErrorIs(t, err, expectedRepoError) // should fwd original error
		mockRepo.AssertExpectations(t)
	})

	t.Run("should return ErrInvalidInput for zero UUID and not call repo method", func(t *testing.T) {
		options := pr.Options{IncludeDeleted: false}
		// Expect that GetProductByID is NOT called on the mock repo for invalid input
		mockRepo.On("GetProductByID", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("should not be called")).Maybe()

		product, err := productService.GetProductByID(ctx, uuid.Nil, options)
		assert.Nil(t, product)
		assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
		mockRepo.AssertNotCalled(t, "GetProductByID") // assert repo method was not called
	})
}

func TestProductService_GetListOfProducts(t *testing.T) {
	mockRepo := new(MockProductRepository)
	productService := service.NewProductService(mockRepo)
	ctx := context.Background()

	// ==== Test Data ====
	catID1 := uuid.New()
	catID2 := uuid.New()

	productA_DAO := &pr.ProductDAO{
		ID: uuid.New(), Name: "Item A", Price: 10.00,
		CategoryId: catID1, CategoryName: "CatA", CreatedAt: time.Now().Add(-5 * time.Hour).Truncate(time.Microsecond), UpdatedAt: time.Now().Truncate(time.Microsecond), DeletedAt: nil,
	}
	productB_DAO := &pr.ProductDAO{
		ID: uuid.New(), Name: "Item B", Price: 20.00,
		CategoryId: catID2, CategoryName: "CatB", CreatedAt: time.Now().Add(-10 * time.Hour).Truncate(time.Microsecond), UpdatedAt: time.Now().Add(-1 * time.Hour).Truncate(time.Microsecond), DeletedAt: nil,
	}
	deletedAtC := time.Now().Add(-3 * time.Hour).Truncate(time.Microsecond)
	productC_DAO_Deleted := &pr.ProductDAO{
		ID: uuid.New(), Name: "Item C", Price: 15.00,
		CategoryId: catID1, CategoryName: "CatA", CreatedAt: time.Now().Add(-15 * time.Hour).Truncate(time.Microsecond), UpdatedAt: time.Now().Add(-2 * time.Hour).Truncate(time.Microsecond), DeletedAt: &deletedAtC,
	}

	activeDAOs := []*pr.ProductDAO{productA_DAO, productB_DAO}
	allDAOs := []*pr.ProductDAO{productA_DAO, productB_DAO, productC_DAO_Deleted}

	t.Run("should return a list of active products by default (IncludeDeleted: false)", func(t *testing.T) {
		params := &pe.ProductListQueryParams{IncludeDeleted: false}
		mockRepo.On("GetListOfProducts", ctx, params).Return(activeDAOs, nil).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		require.NoError(t, err)
		assert.Len(t, products, 2)
		assert.Equal(t, productA_DAO.ToDomainModel().Id, products[0].Id)
		assert.Equal(t, productB_DAO.ToDomainModel().Id, products[1].Id)
		assert.True(t, products[0].IsActive)
		assert.True(t, products[1].IsActive)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should return all products including deleted if IncludeDeleted is true", func(t *testing.T) {
		params := &pe.ProductListQueryParams{IncludeDeleted: true}
		mockRepo.On("GetListOfProducts", ctx, params).Return(allDAOs, nil).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		require.NoError(t, err)
		assert.Len(t, products, 3)
		assert.True(t, products[0].IsActive)
		assert.True(t, products[1].IsActive)
		// The deleted product should be inactive
		assert.False(t, products[2].IsActive)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should return empty list if no products found by repository", func(t *testing.T) {
		params := &pe.ProductListQueryParams{}
		mockRepo.On("GetListOfProducts", ctx, params).Return([]*pr.ProductDAO{}, nil).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		require.NoError(t, err)
		assert.Empty(t, products)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should propagate repository errors for list operation", func(t *testing.T) {
		params := &pe.ProductListQueryParams{}
		expectedRepoError := errors.New("timeout while fetching products")
		mockRepo.On("GetListOfProducts", ctx, params).Return(nil, expectedRepoError).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		assert.Nil(t, products)
		assert.ErrorIs(t, err, expectedRepoError)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should filter by name and categoryIDs", func(t *testing.T) {
		params := &pe.ProductListQueryParams{
			Name:           "Item A",
			CategoryIDs:    []uuid.UUID{catID1},
			IncludeDeleted: false, // Ensure this is explicitly set for mock matching
		}
		mockRepo.On("GetListOfProducts", ctx, params).Return([]*pr.ProductDAO{productA_DAO}, nil).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		require.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, productA_DAO.ToDomainModel().Id, products[0].Id)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should handle pagination (limit and offset)", func(t *testing.T) {
		params := &pe.ProductListQueryParams{
			Limit:          1,
			Offset:         1,
			IncludeDeleted: false,
		}
		// Mock the repository to return the expected sub-slice based on pagination
		mockRepo.On("GetListOfProducts", ctx, params).Return([]*pr.ProductDAO{productB_DAO}, nil).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		require.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, productB_DAO.ToDomainModel().Id, products[0].Id)
		mockRepo.AssertExpectations(t)
	})

	t.Run("should handle sorting", func(t *testing.T) {
		params := &pe.ProductListQueryParams{
			SortBy:         "price",
			SortOrder:      "asc",
			IncludeDeleted: false,
		}
		// Repository is mocked to return already sorted DAOs
		sortedByPriceDAOs := []*pr.ProductDAO{productA_DAO, productB_DAO} // 10.00, 20.00
		mockRepo.On("GetListOfProducts", ctx, params).Return(sortedByPriceDAOs, nil).Once()

		products, err := productService.GetListOfProducts(ctx, params)
		require.NoError(t, err)
		assert.Len(t, products, 2)
		assert.Equal(t, productA_DAO.ToDomainModel().Id, products[0].Id)
		assert.Equal(t, productB_DAO.ToDomainModel().Id, products[1].Id)
		mockRepo.AssertExpectations(t)
	})

	// ==== Validation Tests for ProductListQueryParams ====
	t.Run("should return ErrInvalidInput for negative Limit", func(t *testing.T) {
		params := &pe.ProductListQueryParams{Limit: -1}
		// Expect repo method NOT to be called
		mockRepo.On("GetListOfProducts", mock.Anything, mock.Anything).Return(nil, errors.New("should not be called")).Maybe()

		products, err := productService.GetListOfProducts(ctx, params)
		assert.Nil(t, products)
		assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
		mockRepo.AssertNotCalled(t, "GetListOfProducts") // Ensure the mock was not engaged
	})

	t.Run("should return ErrInvalidInput for negative Offset", func(t *testing.T) {
		params := &pe.ProductListQueryParams{Offset: -1}
		mockRepo.On("GetListOfProducts", mock.Anything, mock.Anything).Return(nil, errors.New("should not be called")).Maybe()

		products, err := productService.GetListOfProducts(ctx, params)
		assert.Nil(t, products)
		assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
		mockRepo.AssertNotCalled(t, "GetListOfProducts")
	})

	t.Run("should return ErrInvalidInput for invalid SortBy column", func(t *testing.T) {
		params := &pe.ProductListQueryParams{SortBy: "non_existent_column"}
		mockRepo.On("GetListOfProducts", mock.Anything, mock.Anything).Return(nil, errors.New("should not be called")).Maybe()

		products, err := productService.GetListOfProducts(ctx, params)
		assert.Nil(t, products)
		assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
		mockRepo.AssertNotCalled(t, "GetListOfProducts")
	})

	t.Run("should return ErrInvalidInput for invalid SortOrder", func(t *testing.T) {
		params := &pe.ProductListQueryParams{SortOrder: "invalid_order"}
		mockRepo.On("GetListOfProducts", mock.Anything, mock.Anything).Return(nil, errors.New("should not be called")).Maybe()

		products, err := productService.GetListOfProducts(ctx, params)
		assert.Nil(t, products)
		assert.ErrorIs(t, err, apperrors.ErrInvalidInput)
		mockRepo.AssertNotCalled(t, "GetListOfProducts")
	})
}
