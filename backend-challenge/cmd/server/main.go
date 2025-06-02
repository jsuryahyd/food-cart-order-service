package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/jsuryahyd/food-cart-order-service/internal/app"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
)

func main() {

	appCtx := context.Background()

	_ = godotenv.Load()
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatal("Failed to load config on server starup %w", err)
		return
	}

	application, err := app.NewApplication(appCtx, cfg)
	if err != nil {
		log.Fatalf("Application Setup failed %v \n", err)
		return
	}
	logger := application.Logger

	go application.RunServer()

	//For graceful shutdown
	exit := make(chan os.Signal, 1)

	signal.Notify(exit, syscall.SIGINT /*CTRL+C*/, syscall.SIGTERM /*Docker stop signals*/)
	<-exit //block until signal is received

	logger.Info("Stop signal received, shutting down the server...")

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := application.Shutdown(ctxWithTimeout); err != nil {
		log.Fatalf("Server failed to shutdown %v", err)
		os.Exit(1)
	}

}
