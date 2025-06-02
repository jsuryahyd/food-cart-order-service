package apperrors

import "errors"

var (
	ErrNotFound = errors.New("not found")

	ErrInvalidInput = errors.New("invalid input parameter")

	ErrInternalServer = errors.New("internal server error")

	// authentication error
	ErrUnauthorized = errors.New("unauthorized")

	// authorization error
	ErrForbidden = errors.New("forbidden")

	// Idempotency while creating orders
	ErrAlreadyExists = errors.New("already exists")
)

var (
	ErrInsufficientStock       = errors.New("insufficient stock")
	ErrProductsNotFound        = errors.New("One of more products are not found")
	ErrInternalServerForOrders = errors.New("An unexpected error occurred while placing the order.")
)

var userMessages = map[error]string{
	ErrInvalidInput:      "Your order input is invalid. Please check your data.",
	ErrInsufficientStock: "Sorry, there is not enough stock for one or more products.",
	ErrNotFound:          "The requested item was not found.",
	ErrUnauthorized:      "You are not authorized to access this resource.",
}

func UserMessage(err error) string {
	for k, v := range userMessages {
		if errors.Is(err, k) {
			return v
		}
	}
	return "An unexpected error occurred. Please try again."
}
