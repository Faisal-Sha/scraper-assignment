package db

import (
	"fmt"

	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"scraper/internal/models"

	"github.com/sirupsen/logrus"
)

// Setup initializes the PostgreSQL database connection and runs migrations.
func Setup() *gorm.DB {
	host := viper.GetString("DB_HOST")
	user := viper.GetString("DB_USER")
	password := viper.GetString("DB_PASSWORD")
	dbname := viper.GetString("DB_NAME")
	port := viper.GetString("DB_PORT")

	// Set default values if not provided
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if dbname == "" {
		dbname = "scraper"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, password, dbname, port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}

	// Migrate tables
	db.AutoMigrate(&models.Product{}, &models.PriceStockLog{}, &models.User{}, &models.UserFavorite{})

	// Create default user if none exists
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		db.Create(&models.User{
			Email:    "usmaaslam187@gmail.com",
			Username: "admin",
			Password: "admin123", // TODO: Hash in production
			Name:     "Admin User",
		})
		logrus.Info("Created default admin user")
	}

	logrus.Info("Database initialized successfully")
	return db
}