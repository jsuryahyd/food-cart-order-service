package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

type Application struct {
	Server *http.Server
	DB     *sql.DB
	Logger *logging.Logger
	Router *gin.Engine
}

func NewApplication(ctx context.Context, config *config.Config) (*Application, error) {
	logger := logging.GetLogger()
	defer logger.Sync()
	//DB connection
	//todo: when we initialize repositories, we pass db(_) as an arg to them.
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.Database.User, config.Database.Password, config.Database.Host, config.Database.Port, config.Database.DbName)

	dbConn, dbErr := db.GetConnection(dbURL, logger)
	if dbErr != nil {
		return nil, dbErr
	}

	//todo: move path to env variable
	if migrationErr := db.RunMigrations(dbURL, "file://db/migrations", logger); migrationErr != nil {
		return nil, migrationErr
	}
	// if config.Environment == "development" || config.Environment == "test" {
	// 	logger.Info("Running seed data for development environment...")
	// 	if err := db.TruncateTables(ctx, dbConn, logger); err != nil {
	// 		return nil, fmt.Errorf("failed to truncate tables before seeding: %w", err)
	// 	}
	// 	if err := db.SeedData(ctx, dbConn, logger); err != nil {
	// 		logger.Fatal("Failed to seed database", err)
	// 	}
	// }

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
	//todo: initiate redis

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

	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server force shutdown")
	}
	app.Logger.Info("Server shutdown gracefully")
	return nil
}
