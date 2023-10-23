package core

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(postgresDsn string) (*gorm.DB, error) {
	// use postgres
	var database *gorm.DB
	var err error

	database, err = gorm.Open(sqlite.Open(postgresDsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	sqlDB, err := database.DB()

	//database.AutoMigrate(&Validator{}, &Staker{}, &DataValidatorIndexerRun{}, &DataStakerIndexerRun{}, &Block{})

	if err != nil {
		return nil, err
	}
	//conection pooling settings
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(90)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Attempt to enable the uuid-ossp extension
	err = database.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error
	if err != nil {
		return nil, fmt.Errorf("failed to enable uuid-ossp extension: %w", err)
	}

	if err != nil {
		return nil, err
	}
	return database, nil
}
