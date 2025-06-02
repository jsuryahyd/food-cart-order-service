package session

import userrepository "github.com/jsuryahyd/food-cart-order-service/internal/modules/user/repository"

type UserSession struct {
	User      userrepository.User
	SessionId string
}
