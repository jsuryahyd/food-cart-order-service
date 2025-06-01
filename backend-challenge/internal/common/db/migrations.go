package db

import (
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
)

func RunMigrations(dbURL string, fileURL string, logger *logging.Logger) error {
	m, err := migrate.New(fileURL, dbURL)
	if err != nil {
		return fmt.Errorf("failed to init migration %w", err)
	}
	const maxAttempts = 5

	for i := 0; i < maxAttempts; i++ {

		err = m.Up()
		if err == nil {
			logger.Info("Database migrations applied successfully")
			return nil
		}

		if err == migrate.ErrNoChange {
			logger.Info("Database migrations: No new changes to apply")
			return nil
		}

		logger.Warnf("Database migrations: Error while applying changes. Attempt (%d/%d). %v. Retrying in 2 seconds...", i+1, maxAttempts, err)
		time.Sleep(time.Second * 2)
	}

	return fmt.Errorf("failed to run migrations after %d attempts. %w", maxAttempts, err)
}
