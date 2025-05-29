package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

func RunServer(ctx context.Context, config *config.Config) error {

	//todo: initiate db

	//todo: initiate redis

	router := gin.New()

	logging.InitLogger()
	router.Use(logging.GinLogger())
	router.Use(gin.Recovery())

	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "UP"})
	})
	//scalar api client
	router.Static("/scalar", "./web/scalar")
	router.StaticFile("/openapi.yaml", "./api/openapi.yaml")

	//todo: register routes

	server := &http.Server{
		Addr:         config.Server.Port,
		Handler:      router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
	}
	logger := logging.GetLogger()
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorw("server failed to start and listen", "error", err)
		}
	}()

	//graceful shutdown
	exit := make(chan os.Signal, 1)

	signal.Notify(exit, syscall.SIGINT /*CTRL+C*/, syscall.SIGTERM /*Docker stop signals*/)
	<-exit //block until signal is received

	logger.Info("Stop signal received, shutting down the server...")

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxWithTimeout); err != nil {
		return fmt.Errorf("Server force shutdown")
	}
	logger.Info("Server shutdown gracefully")

	return nil
}
