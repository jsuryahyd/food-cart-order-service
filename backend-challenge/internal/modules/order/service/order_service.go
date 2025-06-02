package orderservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	oe "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/entities"
	or "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/repository"
	pe "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/entities"
	pr "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
)

type OrderService interface {
	PlaceOrder(ctx context.Context, payload *oe.PlaceOrderPayload) (*oe.Order, error)
	GetOrderById(ctx context.Context, id uuid.UUID) (*oe.Order, error)
}

type OrderServiceImpl struct {
	orderRepo   or.OrderRepository
	productRepo pr.ProductRepository
	logger      *logging.Logger
	db          *sql.DB //todo: use unitOfWork pattern to avoid direct db interaction from the service.
	stockRepo   or.StockRepository
	//couponService
}

func NewOrderService(orderRepo or.OrderRepository, productRepo pr.ProductRepository, stockRepo or.StockRepository, db *sql.DB) *OrderServiceImpl {
	return &OrderServiceImpl{
		orderRepo:   orderRepo,
		productRepo: productRepo,
		logger:      logging.GetLogger(),
		stockRepo:   stockRepo,
		db:          db,
	}
}

/*
*
- Orchestrates a transaction involving multiple repository methods(read and write operations) to create an order,
aquire lock on product rows, reduce the quantity of products, return errors for insufficient stock etc.
*/
func (s *OrderServiceImpl) PlaceOrder(ctx context.Context, payload *oe.PlaceOrderPayload) (*oe.Order, error) {
	if payload == nil || len(payload.Items) == 0 {
		return nil, fmt.Errorf("%w: order payload is missing items %v", payload.Items, apperrors.ErrInvalidInput)
	}

	if payload.UserId == uuid.Nil {
		return nil, fmt.Errorf("%w: order payload is missing User ID %v", payload.UserId, apperrors.ErrInvalidInput)
	}

	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("%w: order must contain at least one item", apperrors.ErrInvalidInput)
	}
	for _, item := range payload.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("%w: item quantity for product %s must be positive", item.ProductId, apperrors.ErrInvalidInput)
		}
	}

	// Start a database transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Errorf("Failed to begin transaction: %v", err)
		return nil, fmt.Errorf("could not start order process: %w", err)
	}
	var finalOrder *oe.Order
	// Defer a function to handle commit or rollback
	defer func() {
		if p := recover(); p != nil {
			s.logger.Error("Panic occurred, rolling back transaction")
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Errorf("Panic recovery: Failed to rollback transaction: %v", rbErr)
			}
			panic(p) // Re-throw panic after attempting rollback
		} else if err != nil {
			s.logger.Warnf("Error occurred (%v), rolling back transaction", err)
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Errorf("Error handling: Failed to rollback transaction: %v", rbErr)
				// err is the primary error we want to return, but log rollback failure
			}
		} else {
			// No error, attempt to commit
			if cmErr := tx.Commit(); cmErr != nil {
				s.logger.Errorf("Failed to commit transaction: %v", cmErr)
				err = fmt.Errorf("failed to finalize order: %w", cmErr) // Set err so it's returned
				finalOrder = nil                                        // Ensure order is not returned on commit failure
			} else {
				s.logger.Infof("Order %s transaction committed successfully", finalOrder.Id)
			}
		}
	}()

	// --- Business Logic within the Transaction ---

	// 1. Collect Product IDs to fetch details in batch
	productIDs := make([]uuid.UUID, 0, len(payload.Items))
	for _, item := range payload.Items {
		productIDs = append(productIDs, item.ProductId)
	}

	// 2. Fetch product details (e.g., price, name)
	// It's good to fetch product details within the transaction if their state (like price)
	// needs to be consistent with the stock check.
	productDetails, err := s.productRepo.GetListOfProductDetails(ctx, tx, productIDs)
	if err != nil {
		s.logger.Errorf("Failed to fetch product details in batch: %v", err)
		// err will be handled by defer, trigger rollback
		return nil, fmt.Errorf("could not retrieve product information: %w", err)
	}
	productDetailsMap := map[uuid.UUID]pr.ProductDAO{}
	for _, p := range productDetails {
		productDetailsMap[p.ID] = *p
	}

	var orderItems []oe.OrderItem
	var totalAmount float64 = 0.0 //todo: check if this is needed

	// 3. Process each item: check stock, reduce stock, calculate price
	for _, itemInput := range payload.Items {
		product, ok := productDetailsMap[itemInput.ProductId]
		if !ok {
			err = fmt.Errorf("product details not found for ID %s: %w", itemInput.ProductId, apperrors.ErrNotFound)
			return nil, err // err will be handled by defer
		}

		// 3a. Get current stock with FOR UPDATE lock (within the transaction)
		currentStock, stockErr := s.stockRepo.GetProductStock(ctx, tx, itemInput.ProductId)
		if stockErr != nil {
			if errors.Is(stockErr, apperrors.ErrNotFound) {
				err = fmt.Errorf("stock information not found for product %s (%s): %w", product.Name, itemInput.ProductId, stockErr)
			} else {
				err = fmt.Errorf("failed to get stock for product %s (%s): %w", product.Name, itemInput.ProductId, stockErr)
			}
			return nil, err // err will be handled by defer
		}

		if currentStock < itemInput.Quantity {
			err = fmt.Errorf("insufficient stock for product %s (available: %d, requested: %d): %w",
				product.Name, currentStock, itemInput.Quantity, apperrors.ErrInsufficientStock)
			return nil, err // err will be handled by defer
		}

		// 3b. Reduce stock (within the transaction)
		err = s.stockRepo.ReduceProductStock(ctx, tx, itemInput.ProductId, itemInput.Quantity)
		if err != nil {

			if errors.Is(err, apperrors.ErrInsufficientStock) {
				s.logger.Warnf("Failed to reduce stock for %s due to insufficient quantity after lock: %v", product.Name, err)
				err = fmt.Errorf("unable to secure stock for product %s: %w", product.Name, apperrors.ErrInsufficientStock)
			} else {
				s.logger.Errorf("Failed to reduce stock for product %s: %v", product.Name, err)
				err = fmt.Errorf("could not update stock for product %s: %w", product.Name, err)
			}
			return nil, err // err will be handled by defer
		}

		orderItems = append(orderItems, oe.OrderItem{
			ProductId:       itemInput.ProductId,
			Quantity:        itemInput.Quantity,
			PriceAtPurchase: product.Price, // Price at the time of order creation
		})
		totalAmount += product.Price * float64(itemInput.Quantity)
	}

	// 4. TODO: Apply Coupon/Promo Code
	// discount := s.couponService.CalculateDiscount(input.CouponCode, totalAmount)
	// finalAmount := totalAmount - discount
	finalAmount := totalAmount // Placeholder

	// 5. Construct the final Order entity
	orderToCreate := &oe.Order{
		Id:         uuid.New(), // Generate a new UUID for the order
		User:       oe.User{Id: payload.UserId},
		Items:      orderItems,
		Total:      finalAmount,
		CouponCode: payload.CouponCode,
		Status:     "pending",        // Define order statuses in your entities
		CreatedAt:  time.Now().UTC(), // Helper for time.Now().UTC() or similar
		UpdatedAt:  time.Now().UTC(),
	}

	orderDAO, orderItemDAOs := or.FromOrderEntity(orderToCreate)

	// 6. Save the Order to the database (within the transaction)
	err = s.orderRepo.CreateOrder(ctx, tx, orderDAO, orderItemDAOs)
	if err != nil {
		s.logger.Errorf("Failed to save order: %v", err)
		// err will be handled by defer to rollback
		return nil, fmt.Errorf("could not create order record: %w", err)
	}

	finalOrder = orderToCreate // Set the order to be returned if commit is successful
	// Commit is handled by the defer function if err is nil at this point
	return finalOrder, nil // If err is nil here, defer will commit. Otherwise, it will rollback.
}

// GetOrderById fetches an order by its ID.
func (s *OrderServiceImpl) GetOrderById(ctx context.Context, id uuid.UUID) (*oe.Order, error) {
	order, err := s.orderRepo.GetOrderById(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrNotFound
		}
		s.logger.Errorf("Failed to get order by ID %s: %v", id, err)
		return nil, fmt.Errorf("failed to retrieve order: %w", err)
	}

	// Populate products for the order (similar to PlaceOrder logic)
	productIDs := make([]uuid.UUID, len(order.Items))
	for i, item := range order.Items {
		productIDs[i] = item.ProductId
	}

	// Fetch all required products
	productsMap := make(map[uuid.UUID]*pe.Product)
	var productEntities []*pe.Product

	productDAOs, err := s.productRepo.GetListOfProducts(ctx, &pe.ProductListQueryParams{IncludeDeleted: false, Ids: productIDs})
	for _, productDAO := range productDAOs {
		if err != nil {
			//todo: Not throwing error if a product data is missing. Decide if this is chosen behaviour
			s.logger.Warnf("Product %s for order %s not found during retrieval: %v", productDAO.ID, id, err)
			continue
		}
		product := &pe.Product{
			Id:       productDAO.ID,
			Name:     productDAO.Name,
			Price:    productDAO.Price,
			IsActive: productDAO.DeletedAt == nil,
		}

		productsMap[product.Id] = product
		productEntities = append(productEntities, product)
	}
	order.Products = productEntities

	return order, nil
}
