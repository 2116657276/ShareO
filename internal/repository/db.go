package repository

import (
	"fmt"
	"log"
	"time"

	"github.com/zhoujianlin/ShareO/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg config.DatabaseConfig, mode string) error {
	var err error
	logLevel := logger.Warn
	if mode == "debug" {
		logLevel = logger.Info
	}
	DB, err = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect mysql: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	log.Println("MySQL connected successfully")

	// Detect FULLTEXT index availability
	DetectFulltext()

	return nil
}
