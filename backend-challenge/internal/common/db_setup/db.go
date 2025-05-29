package db_setup

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

func GetConnection(dbURL string, logger *zap.SugaredLogger) (*sql.DB, error) {
	const maxAttempts = 3
	var db *sql.DB
	var err error

	for i := 0; i < maxAttempts; i++ {
		logger.Infof("Connection Attempt to DB %s", dbURL)
		if db, err = sql.Open("postgres", dbURL); err != nil {
			logger.Errorf("Failed to get db connection %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err = db.Ping(); err != nil {
			multiplier := 2 * (i + 1)
			logger.Warnf("Database Ping failed. Attempt (%d/%d). %v. Retrying in %d seconds... ", i+1, maxAttempts, err, multiplier)
			time.Sleep(time.Duration(multiplier) * time.Second)
			continue
		}

		logger.Info("Successfully connected to Database")
		return db, nil

	}

	return nil, fmt.Errorf("Failed to connect to db after %d attempts. %v", maxAttempts, err)
}
