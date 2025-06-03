package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	session "github.com/jsuryahyd/food-cart-order-service/internal/common/auth"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	redisclient "github.com/jsuryahyd/food-cart-order-service/internal/common/redis"
	orderhandler "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/http"
	orderrepository "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/repository"
	orderservice "github.com/jsuryahyd/food-cart-order-service/internal/modules/order/service"
	producthandler "github.com/jsuryahyd/food-cart-order-service/internal/modules/product/http"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/product/repository"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/product/service"
	promocache "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/cache"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/messaging"
	promostore "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/store"
	promohandler "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/http"
	promoservice "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/service"
	userrepository "github.com/jsuryahyd/food-cart-order-service/internal/modules/user/repository"
)

type Application struct {
	Server *http.Server
	DB     *sql.DB
	Logger *logging.Logger
	Router *gin.Engine
}

var redisClient *redis.Client

func NewApplication(ctx context.Context, config *config.Config) (*Application, error) {
	logger := logging.GetLogger()
	defer logger.Sync()

	//DB connection
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.Database.User, config.Database.Password, config.Database.Host, config.Database.Port, config.Database.DbName)

	dbConn, dbErr := db.GetConnection(dbURL, logger)
	if dbErr != nil {
		return nil, dbErr
	}

	// Initialize Redis client for the main app
	redisClient = redisclient.GetRedisClient(config)
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Fatalf("Could not connect to Redis from main app: %v", err)
	}
	logger.Info("Main app successfully connected to Redis.")

	//todo: move path to env variable
	if migrationErr := db.RunMigrations(dbURL, "file://db/migrations", logger); migrationErr != nil {
		return nil, migrationErr
	}
	if config.Misc.ShouldSeedData || config.Environment == "development" || config.Environment == "test" {
		logger.Info("Running seed data for development environment...")
		if err := db.TruncateTables(ctx, dbConn, logger); err != nil {
			return nil, fmt.Errorf("failed to truncate tables before seeding: %w", err)
		}
		if err := db.SeedData(ctx, dbConn, logger); err != nil {
			logger.Fatalf("Failed to seed database %+v", err)
		}
	}

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

	// ==== Promo service starts ====
	//todo: the code was moved up here to make promoService available to orderHandler. This has to be refactored.
	processedCouponsFilePath := config.CouponProcessor.OUTPUT_FILE_PATH
	processedCouponsIndexFilePath := config.CouponProcessor.ProcessedCouponsIndexFilePath
	if processedCouponsFilePath == "" || processedCouponsIndexFilePath == "" {
		logger.Fatal("PROCESSED_COUPONS_FILE_PATH or ProcessedCouponsIndexFilePath is not set in config for app.")
	}
	fileStore := promostore.NewCouponFileStore(processedCouponsFilePath, processedCouponsIndexFilePath)

	// Ping Redis to check connectivity
	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Fatalf("Could not connect to Redis: %v", err)
	}
	logger.Info("Successfully connected to Redis.")
	couponCache := promocache.NewCouponCache(redisClient)
	promoService := promoservice.NewPromoService(couponCache, fileStore, config)
	//==== Promo Service to be contd. ====

	apiGroup := router.Group("/api")

	{
		// Product
		repository := repository.NewProductRepository(dbConn)
		productService := service.NewProductService(repository)
		productHandler := producthandler.NewProductHandler(productService)
		apiGroup.GET("/product/:productId", productHandler.GetProductByID)
		apiGroup.GET("/product", productHandler.ListProducts)
	}
	{
		// Order setup
		ordRepository := orderrepository.NewOrderRepository(dbConn)
		ordService := orderservice.NewOrderService(ordRepository, repository.NewProductRepository(dbConn), orderrepository.NewPgStockRepository(dbConn), dbConn)
		ordHandler := orderhandler.NewOrderHandler(ordService, promoService)
		apiGroup.POST("/order", session.AuthMiddleware(config, userrepository.NewUserRepository(dbConn), logging.GetLogger().With("middleware", "AuthMiddleware")), ordHandler.PlaceOrder)
	}

	{
		//==== promo service contd.

		messagePublisher := messaging.NewRedisPublisher(redisClient)
		promoHandler := promohandler.NewPromoHandler(promoService, config, messagePublisher)
		//sends messages to queue on api request
		apiGroup.PUT("/admin/update-coupon-cache", session.AuthMiddleware(config, userrepository.NewUserRepository(dbConn), logger), promoHandler.TriggerCouponFileProcessing)

		//receives messages from queue and refreshes cache from files
		promoHandler.StartCouponUpdateListener(ctx, redisClient, config.Redis.CouponUpdateChannel)
	}

	server := &http.Server{
		Addr:         config.Server.Port,
		Handler:      router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
	}

	return &Application{
		Router: router,
		Server: server,
		DB:     dbConn,
		Logger: logger,
	}, nil

}

func (app *Application) RunServer() error {

	if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		app.Logger.Errorw("server failed to start and listen", "error", err)
		return err
	}

	return nil
}

func (app *Application) Shutdown(shutdownCtx context.Context) error {
	redisclient.CloseRedisClient()

	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server force shutdown")
	}
	app.Logger.Info("Server shutdown gracefully")
	return nil
}
