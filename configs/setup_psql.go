package configs

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var PGDB *gorm.DB

func ConnectPSQLDatabase() {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		EnvDBHost(),
		EnvDBUser(),
		"synEyeC#1",
		EnvDBName(),
		EnvDBPort(),
	)
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		panic("Failed to connect to database")
	}

	PGDB = database
	fmt.Println("Connected to database : " + PGDB.Name())
}
