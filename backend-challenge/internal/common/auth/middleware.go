package session

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	apperrors "github.com/jsuryahyd/food-cart-order-service/internal/common/errors"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	userrepository "github.com/jsuryahyd/food-cart-order-service/internal/modules/user/repository"
)

const UserSessionKey = "user_session"

func AuthMiddleware(config *config.Config, userRepo userrepository.UserRepository, logger *logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("api_key")
		if apiKey != config.ApiKeys.AdminKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(apperrors.ErrUnauthorized),
			})
			c.Abort()
			return
		}

		user, err := userRepo.GetFirstUser(c.Request.Context())
		if err != nil || user == nil {
			logger.Warnw("Cannot get the first user %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   true,
				"message": apperrors.UserMessage(apperrors.ErrUnauthorized),
			})
			c.Abort()
			return
		}

		// dummy session object
		session := UserSession{
			User:      *user,
			SessionId: "generated-session-id",
		}
		c.Set(UserSessionKey, session)
		c.Next()
	}
}
