package configs

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var PGDB *gorm.DB

func ConnectPSQLDatabase() error {
	if PGDB != nil {
		return nil // Already connected
	}

	logger := LogWithContext("database", "postgresql-connect")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		EnvDBHost(),
		EnvDBUser(),
		EnvDBPassword(),
		"eyecdb",
		EnvDBPort(),
	)
	logger.Debug("Connecting to PostgreSQL", "host", EnvDBHost(), "port", EnvDBPort(), "database", EnvDBName())

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to PostgreSQL database", "error", err, "host", EnvDBHost(), "database", EnvDBName())
		return err
	}

	// Test the connection
	sqlDB, err := database.DB()
	if err != nil {
		logger.Error("Failed to get underlying SQL DB", "error", err)
		return err
	}

	if err := sqlDB.Ping(); err != nil {
		logger.Error("Failed to ping PostgreSQL database", "error", err)
		return err
	}

	PGDB = database
	logger.Info("Connected to PostgreSQL database successfully", "database", PGDB.Name())
	return nil
}
