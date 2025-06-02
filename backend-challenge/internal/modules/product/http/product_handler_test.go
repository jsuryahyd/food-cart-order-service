package producthandler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
	producthandler "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/http"
	pdto "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/http/dto"
	pr "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
)

// ==== Mock service implementation ====
type MockProductService struct {
	mock.Mock
}

func (m *MockProductService) GetProductByID(ctx context.Context, id uuid.UUID, options pr.Options) (*pe.Product, error) {
	args := m.Called(ctx, id, options)
	// nil check before type assertion
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pe.Product), args.Error(1)
}

func (m *MockProductService) GetListOfProducts(ctx context.Context, params *pe.ProductListQueryParams) ([]*pe.Product, error) {
	args := m.Called(ctx, params)
	// nil check before type assertion
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*pe.Product), args.Error(1)
}

// ==== Mock Service Implementation ====

func TestProductHandler_GetProductByID(t *testing.T) {
	// suppress verbose debug output during tests.
	gin.SetMode(gin.TestMode)

	mockService := new(MockProductService)

	handler := producthandler.NewProductHandler(mockService)

	router := gin.New()

	router.GET("/product/:productId", handler.GetProductByID)

	// --- Sample Data for Tests ---
	sampleProductUUID := uuid.New()
	sampleProduct := &pe.Product{
		Id:    sampleProductUUID,
		Name:  "Espresso",
		Price: 3.00,
		Category: struct {
			Id   uuid.UUID
			Name string
		}{Id: uuid.New(), Name: "Coffee"},
		IsActive: true,
	}
	// Default options expected by the handler when calling the service.
	defaultOptions := pr.Options{IncludeDeleted: false}

	t.Run("should return 200 OK and product for valid ID", func(t *testing.T) {
		// Assertions: service call with specific ID and options, returning sampleProduct and no error.
		mockService.On("GetProductByID", mock.Anything, sampleProductUUID, defaultOptions).Return(sampleProduct, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product/"+sampleProductUUID.String(), nil)

		// capture the HTTP response.
		w := httptest.NewRecorder()
		// Serve the HTTP request.
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var responseProduct pdto.ProductResponse
		err := json.Unmarshal(w.Body.Bytes(), &responseProduct)
		require.NoError(t, err)

		assert.Equal(t, sampleProduct.Id, responseProduct.Id)
		assert.Equal(t, sampleProduct.Name, responseProduct.Name)
		assert.Equal(t, sampleProduct.Price, responseProduct.Price)

		// Verify that mock service call meets all assert expectations.
		mockService.AssertExpectations(t)
	})

	t.Run("should return 400 Bad Request for invalid UUID format", func(t *testing.T) {
		// No service call expected as the handler validates UUID format first.
		req, _ := http.NewRequest(http.MethodGet, "/product/invalid-uuid-string", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid product ID format")

		mockService.AssertNotCalled(t, "GetProductByID")
		mockService.AssertExpectations(t) // Verify no unexpected service calls.
	})

	t.Run("should return 404 Not Found if service returns ErrNotFound", func(t *testing.T) {
		nonExistentUUID := uuid.New()

		mockService.On("GetProductByID", mock.Anything, nonExistentUUID, defaultOptions).Return(nil, apperrors.ErrNotFound).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product/"+nonExistentUUID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Product not found.")
		mockService.AssertExpectations(t)
	})

	t.Run("should return 400 Bad Request if service returns ErrInvalidInput", func(t *testing.T) {

		zeroUUID := uuid.Nil
		mockService.On("GetProductByID", mock.Anything, zeroUUID, defaultOptions).Return(nil, apperrors.ErrInvalidInput).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product/"+zeroUUID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid product ID provided.")
		mockService.AssertExpectations(t)
	})

	t.Run("should return 500 Internal Server Error for other unexpected service errors", func(t *testing.T) {
		serviceErrorUUID := uuid.New()

		mockService.On("GetProductByID", mock.Anything, serviceErrorUUID, defaultOptions).Return(nil, errors.New("database connection lost")).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product/"+serviceErrorUUID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "An unexpected error occurred")
		mockService.AssertExpectations(t)
	})
}

func TestProductHandler_ListProducts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockProductService)
	handler := producthandler.NewProductHandler(mockService)
	router := gin.New()
	router.GET("/product", handler.ListProducts)
	// Sample Product Entities for list responses
	product1 := &pe.Product{Id: uuid.New(), Name: "Latte", Price: 4.00, IsActive: true}
	product2 := &pe.Product{Id: uuid.New(), Name: "Croissant", Price: 3.50, IsActive: true}
	sampleProducts := []*pe.Product{product1, product2}

	t.Run("should return 200 OK and list of products with no query params", func(t *testing.T) {
		// Expected parameters passed to the service should be default/empty.
		expectedParams := &pe.ProductListQueryParams{}
		mockService.On("GetListOfProducts", mock.Anything, mock.MatchedBy(func(params *pe.ProductListQueryParams) bool {
			return reflect.DeepEqual(params, expectedParams)
		})).Return(sampleProducts, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var responseProducts []*pdto.ProductResponse
		err := json.Unmarshal(w.Body.Bytes(), &responseProducts)
		require.NoError(t, err)
		assert.Len(t, responseProducts, 2)
		assert.Equal(t, product1.Id, responseProducts[0].Id)
		mockService.AssertExpectations(t)
	})

	t.Run("should return 200 OK and empty list if no products found by service", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{}
		mockService.On("GetListOfProducts", mock.Anything, mock.MatchedBy(func(params *pe.ProductListQueryParams) bool {
			return reflect.DeepEqual(params, expectedParams)
		})).Return([]*pe.Product{}, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var responseProducts []*pdto.ProductResponse
		err := json.Unmarshal(w.Body.Bytes(), &responseProducts)
		require.NoError(t, err)
		assert.Empty(t, responseProducts)
		mockService.AssertExpectations(t)
	})

	t.Run("should return 200 OK and products with 'name' filter", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{Name: "Latte"}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return([]*pe.Product{product1}, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?name=Latte", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var responseProducts []*pe.Product
		json.Unmarshal(w.Body.Bytes(), &responseProducts)
		assert.Len(t, responseProducts, 1)
		assert.Equal(t, product1.Id, responseProducts[0].Id)
		mockService.AssertExpectations(t)
	})

	t.Run("should return 200 OK and products with 'limit' and 'offset' pagination", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{Limit: 1, Offset: 1}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return([]*pe.Product{product2}, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?limit=1&offset=1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var responseProducts []*pe.Product
		json.Unmarshal(w.Body.Bytes(), &responseProducts)
		assert.Len(t, responseProducts, 1)
		assert.Equal(t, product2.Id, responseProducts[0].Id)
		mockService.AssertExpectations(t)
	})

	t.Run("should return 400 Bad Request for invalid 'limit' parameter format", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/product?limit=invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid 'limit' parameter")
		mockService.AssertExpectations(t) // No service call expected
	})

	t.Run("should return 400 Bad Request for invalid 'offset' parameter format", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/product?offset=xyz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid 'offset' parameter")
		mockService.AssertExpectations(t)
	})

	t.Run("should return 400 Bad Request if service returns ErrInvalidInput (offset < 0)", func(t *testing.T) {
		// Simulate a case where handler parses a valid integer, but service considers it invalid (e.g., negative).
		expectedParams := &pe.ProductListQueryParams{Limit: 10, Offset: -1}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return(nil, errors.New("offset cannot be negative: "+apperrors.ErrInvalidInput.Error())).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?limit=10&offset=-1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid query parameters: offset cannot be negative")
		mockService.AssertExpectations(t)
	})

	t.Run("should return 500 Internal Server Error for other unexpected service errors", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return(nil, errors.New("unexpected database connection error")).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "An unexpected error occurred")
		mockService.AssertExpectations(t)
	})

	// todo: Failing test
	t.Run("should parse 'include_deleted' correctly as true", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{IncludeDeleted: true}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return(sampleProducts, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?include_deleted=true", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("should parse 'include_deleted' correctly as false", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{IncludeDeleted: false}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return(sampleProducts, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?include_deleted=false", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("should return 400 Bad Request for invalid 'include_deleted' format", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/product?include_deleted=invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid 'include_deleted' parameter")
		mockService.AssertExpectations(t)
	})

	t.Run("should parse 'sort_by' and 'sort_order' correctly", func(t *testing.T) {
		expectedParams := &pe.ProductListQueryParams{SortBy: "price", SortOrder: "desc"}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return(sampleProducts, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?sort_by=price&sort_order=desc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("should handle multiple 'category_id' query parameters", func(t *testing.T) {
		catID1 := uuid.New()
		catID2 := uuid.New()
		expectedParams := &pe.ProductListQueryParams{CategoryIDs: []uuid.UUID{catID1, catID2}}
		mockService.On("GetListOfProducts", mock.Anything, expectedParams).Return(sampleProducts, nil).Once()

		req, _ := http.NewRequest(http.MethodGet, "/product?category_id="+catID1.String()+"&category_id="+catID2.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("should return 400 Bad Request for invalid 'category_id' format", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/product?category_id=invalid-cat-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["error"])
		assert.Contains(t, response["message"], "Invalid category ID format")
		mockService.AssertExpectations(t)
	})
}
