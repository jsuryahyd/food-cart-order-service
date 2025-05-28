package main

import (
	"context"
	"log"
	"os"

	"github.com/jsuryahyd/food-cart-order-service/internal/app"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
)

func main() {

	context := context.Background()

	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatal("Failed to load config on server starup %w", err)
		return
	}

	if err := app.RunServer(context, cfg); err != nil {
		log.Fatalf("Server failed to start/shutdown %w", err)
		os.Exit(1)
	}

}
