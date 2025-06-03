package orderhandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	session "github.com/jsuryahyd/food-cart-order-service/internal/common/auth"
	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	oe "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/entities"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/order/http/dto"
	promoEntities "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/entities"
	promohandler "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/http"
)

type OrderService interface {
	PlaceOrder(ctx context.Context, payload *oe.PlaceOrderPayload) (*oe.Order, error)
	GetOrderById(ctx context.Context, id uuid.UUID) (*oe.Order, error)
}

type OrderHandler struct {
	orderService OrderService
	logger       *logging.Logger
	promoService promohandler.PromoService
}

func NewOrderHandler(orderService OrderService, promoService promohandler.PromoService) *OrderHandler {
	return &OrderHandler{orderService: orderService, promoService: promoService, logger: logging.GetLogger()}
}

// POST /order endpoint
func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	var req dto.PlaceOrderRequest
	if err := BindStrictJSON(c, &req); err != nil {
		h.logger.Errorf("Invalid request payload: %v" + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   true,
			"message": "Invalid Input. Please check the order payload",
		})
		return
	}

	if err := ValidatePlaceOrderRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   true,
			"message": "Validation error: " + err.Error(),
		})
		return
	}

	items := make([]oe.OrderItem, len(req.Items))
	for i, itemReq := range req.Items {
		productID, _ := uuid.Parse(itemReq.ProductId)
		items[i] = oe.OrderItem{
			ProductId: productID,
			Quantity:  itemReq.Quantity,
		}
	}

	userSession, ok := c.Get(session.UserSessionKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   true,
			"message": "Unauthorized: user session not found",
		})
		return
	}

	if req.CouponCode != "" {

		couponCodeLen := len(req.CouponCode)
		if couponCodeLen > 0 && (couponCodeLen < 8 || couponCodeLen > 10) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(apperrors.ErrInvalidCoupon),
			})
			return
		}

		isValid, err := h.promoService.ValidateCoupon(promoEntities.Coupon(req.CouponCode))
		if err != nil {
			h.logger.Errorf("error while validating coupon %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(apperrors.ErrInternalServerForOrders), //custom message for the module
			})
			return
		}
		if !isValid {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(apperrors.ErrInvalidCoupon),
			})
			return
		}
	}

	payload := &oe.PlaceOrderPayload{
		CouponCode: req.CouponCode,
		Items:      items,
		UserId:     userSession.(session.UserSession).User.Id,
	}

	order, err := h.orderService.PlaceOrder(c.Request.Context(), payload)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(err),
			})
			return
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(apperrors.ErrProductsNotFound),
			})
			return
		}
		if errors.Is(err, apperrors.ErrInsufficientStock) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(err),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   true,
			"message": apperrors.UserMessage(apperrors.ErrInternalServerForOrders), //custom message for the module
		})
		return
	}

	response := dto.NewOrderResponseFromEntity(order)
	c.JSON(http.StatusOK, response)
}
