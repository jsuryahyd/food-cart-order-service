package producthandler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
	productdto "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/http/dto"
	pr "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
)

// Consumer defines interfaces
type ProductService interface {
	GetProductByID(ctx context.Context, id uuid.UUID, options pr.Options) (*pe.Product, error)
	GetListOfProducts(ctx context.Context, params *pe.ProductListQueryParams) ([]*pe.Product, error)
}

type ProductHandler struct {
	productService ProductService
	logger         *logging.Logger
}

func NewProductHandler(productService ProductService) *ProductHandler {
	return &ProductHandler{productService: productService, logger: logging.GetLogger()}
}

// GET /product/{productId} endpoint
func (h *ProductHandler) GetProductByID(c *gin.Context) {
	productIDStr := c.Param("productId")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   true,
			"message": "Invalid product ID format. Must be a valid UUID.", // for security, the message can be made vague.
		})
		return
	}

	options := pr.Options{}
	// validation
	includeDeletedStr := c.Query("include_deleted")
	includeDeleted := false
	if includeDeletedStr != "" {
		parsed, err := ParseBoolParam(includeDeletedStr, "include_deleted")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": err.Error(),
			})
			return
		}
		includeDeleted = parsed
		if includeDeleted {
			options.IncludeDeleted = includeDeleted
		}
	}

	product, err := h.productService.GetProductByID(c.Request.Context(), productID, options)
	if err != nil {
		h.logger.Error("Failed to get product by ID ", err, "productID", productID.String())

		// Translate service errors to appropriate HTTP responses and user-friendly messages
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   true,
				"message": "Product not found.",
			})
			return
		}

		if errors.Is(err, apperrors.ErrInvalidInput) {
			// validation errors from the service
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": "Invalid product ID provided.",
			})
			return
		}

		// Unknown error
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   true,
			"message": "An unexpected error occurred while fetching product.",
		})
		return
	}

	c.JSON(http.StatusOK, productdto.NewProductResponseFromEntity(product))
}

/*
*
// GET /product endpoint. returns list of products
- Accepts Filters, Pagination and Sort
*/
func (h *ProductHandler) ListProducts(c *gin.Context) {
	params := &pe.ProductListQueryParams{}

	params.Name = c.Query("name")

	if c.Query("category_ids") != "" {
		categoryIDs := strings.Split(c.Query("category_ids"), ",")
		h.logger.Infof("category_ids %v", categoryIDs)
		if len(categoryIDs) > 0 {
			catIDs, err := ParseUUIDsFromQueryArray(categoryIDs, "Category IDs")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   true,
					"message": err.Error(),
				})
				return
			}
			params.CategoryIDs = catIDs
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := ParseIntParam(limitStr, "limit")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": err.Error(),
			})
			return
		}
		params.Limit = limit
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := ParseIntParam(offsetStr, "offset")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": err.Error(),
			})
			return
		}
		params.Offset = offset
	}

	params.SortBy = c.Query("sort_by")
	params.SortOrder = c.Query("sort_order")

	if includeDeletedStr := c.Query("include_deleted"); includeDeletedStr != "" {
		includeDeleted, err := ParseBoolParam(includeDeletedStr, "include_deleted")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": err.Error(),
			})
			return
		}
		if params.IncludeDeleted { //add only if true
			params.IncludeDeleted = includeDeleted
		}
	}

	products, err := h.productService.GetListOfProducts(c.Request.Context(), params)
	if err != nil {
		h.logger.Error("Failed to list products", err, "params", params)

		// Translate service errors to HTTP responses
		if errors.Is(err, apperrors.ErrInvalidInput) {

			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": "Invalid query parameters: " + err.Error(), // Expose specific service validation error message
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   true,
			"message": "An unexpected error occurred while listing products.",
		})
		return
	}

	if products == nil {
		products = []*pe.Product{}
	}

	c.JSON(http.StatusOK, productdto.NewProductResponseListFromEntities(products))
}
