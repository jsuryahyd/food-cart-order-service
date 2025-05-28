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
)

func RunServer(ctx context.Context, config *config.Config) error {

	//todo: initiate db

	//todo: initiate redis

	router := gin.Default()
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	//static
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

	go func() {
		if err := server.ListenAndServe(); err != nil {
			//todo: use logger
			fmt.Printf("server failed to start and listen %w", err) //todo: improper formatting in terminal, after server is shutdown
		}
	}()
	fmt.Printf("server info %v", server)
	//graceful shutdown
	exit := make(chan os.Signal)

	signal.Notify(exit, syscall.SIGINT /*CTRL+C*/, syscall.SIGTERM /*Docker stop signals*/)
	<-exit //block until signal is received

	fmt.Println("Stop signal received, shutting down the server...")

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxWithTimeout); err != nil {
		return fmt.Errorf("Server force shutdown")
	}
	fmt.Println("Server shutdown gracefully")

	return nil
}
