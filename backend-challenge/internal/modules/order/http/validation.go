package orderhandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/order/http/dto"
)

func ValidatePlaceOrderRequest(req *dto.PlaceOrderRequest) error {
	if req == nil {
		return errors.New("request cannot be nil")
	}
	if len(req.Items) == 0 {
		return errors.New("at least one item is required")
	}

	productIDSet := make(map[string]struct{})
	for i, item := range req.Items {

		if _, err := uuid.Parse(item.ProductId); err != nil {
			return fmt.Errorf("item %d: invalid productId: %v", i, err)
		}

		if item.Quantity < 1 {
			return fmt.Errorf("item %d: quantity must be at least 1", i)
		}
		// Check for duplicate ProductId
		if _, exists := productIDSet[item.ProductId]; exists {
			return fmt.Errorf("item %d: duplicate productId: %s", i, item.ProductId)
		}
		productIDSet[item.ProductId] = struct{}{}
	}
	return nil
}

func BindStrictJSON(c *gin.Context, obj interface{}) error {
	// 1. Read the request body into a buffer. This is necessary because
	//    `c.Request.Body` is an `io.ReadCloser` and can only be read once.
	//    We need to read it once for our custom JSON decode and then
	//    potentially restore it for Gin's internal validators or other middlewares.
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// 2. Restore the request body in the Gin context. This makes the body
	//    available for any subsequent handlers or Gin's own internal binding
	//    mechanisms if they were to be used later (though we are handling binding here).
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 3. Create a new JSON decoder to handle the "DisallowUnknownFields" logic.
	decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
	decoder.DisallowUnknownFields() // Crucial: This setting makes the decoder strict.

	// 4. Attempt to decode the JSON into the provided object.
	if err := decoder.Decode(obj); err != nil {
		// Check for specific error types to provide more detailed messages.
		if unmarshalErr, ok := err.(*json.UnmarshalTypeError); ok {
			// Example: "quantity" expects an integer but got a string.
			return fmt.Errorf("invalid type for field '%s', expected %s", unmarshalErr.Field, unmarshalErr.Type)
		}
		if strings.Contains(err.Error(), "unknown field") {
			// Example: Request payload included a field not defined in the struct.
			return fmt.Errorf("request contains unknown fields: %w", err)
		}
		// Handle other generic JSON decoding errors (e.g., malformed JSON syntax).
		return fmt.Errorf("invalid request payload: %w", err)
	}

	// 5. Perform struct tag validations using Gin's underlying validator.
	//    `json.Decoder.Decode` only unmarshals; it doesn't apply `binding` tags.
	//    We get the validator engine that Gin typically uses and run `Struct()` on it.
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.Struct(obj); err != nil {
			// This error comes from your `binding:"required,min=1"` tags.
			return fmt.Errorf("validation error: %w", err)
		}
	} else {
		// This fallback is unlikely in a standard Gin setup, but good for robustness.
		return fmt.Errorf("could not access Gin's validator engine for struct validation")
	}

	return nil // Return nil if everything is successful
}
