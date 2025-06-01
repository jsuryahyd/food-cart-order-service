// internal/common/testutil/testmain.go
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/db"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

func SetupIntegrationTest(m *testing.M, testName string) (*sql.DB, *logging.Logger, func()) {
	_ = godotenv.Load()
	cfg, err := config.LoadConfig(ConfigPath())
	if err != nil {
		logging.GetLogger().Panicw("could not load config", "error", err)
		os.Exit(1)
	}
	testDbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.TestDB.User, cfg.TestDB.Password, cfg.TestDB.Host, cfg.TestDB.Port, cfg.TestDB.DbName)
	logger := logging.GetLogger().With("test", testName)
	testDBConn, err := db.GetConnection(testDbURL, logger)
	if err != nil {
		logger.Panicw("could not get database connection", "error", err)
		os.Exit(1)
	}
	if err := db.RunMigrations(testDbURL, MigrationsPath(), logger); err != nil {
		logger.Panicw("could not run migrations", "error", err)
		os.Exit(1)
	}

	cleanup := func() {
		ctx := context.Background()
		_ = db.TruncateTables(ctx, testDBConn, logger)
	}
	return testDBConn, logger, cleanup
}
