// Package db provides database connection and management functionality
package db

import (
	"fmt"

	// viper for configuration management
	"github.com/spf13/viper"
	// gorm and postgres driver for database operations
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	// internal models for database schema
	"scraper/internal/models"

	// logrus for structured logging
	"github.com/sirupsen/logrus"
)

// Setup initializes the PostgreSQL database connection and runs migrations.
// It reads configuration from environment variables using viper, sets up the
// connection, performs auto-migrations, and ensures a default admin user exists.
// Returns a configured *gorm.DB instance or panics on fatal errors.
func Setup() *gorm.DB {
	// Read database configuration from environment variables
	host := viper.GetString("DB_HOST")
	user := viper.GetString("DB_USER")
	password := viper.GetString("DB_PASSWORD")
	dbname := viper.GetString("DB_NAME")
	port := viper.GetString("DB_PORT")

	// Set default values if environment variables are not provided
	if host == "" {
		host = "localhost" // Default to local PostgreSQL instance
	}
	if port == "" {
		port = "5432" // Default PostgreSQL port
	}
	if dbname == "" {
		dbname = "scraper" // Default database name
	}

	// Construct PostgreSQL connection string
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, password, dbname, port)

	// Initialize GORM with PostgreSQL driver
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}

	// Auto-migrate database schema for all models
	// This creates tables if they don't exist and updates existing ones
	db.AutoMigrate(
		&models.Product{},      // Product information table
		&models.PriceStockLog{}, // Price and stock history
		&models.User{},         // User accounts
		&models.UserFavorite{}, // User's favorite products
	)

	// Ensure at least one admin user exists in the system
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		// Create default admin user for first-time setup
		db.Create(&models.User{
			Email:    "faisal712000@gmail.com",
			Username: "admin",
			Password: "admin123", // TODO: Hash password before storing in production
			Name:     "Admin User",
		})
		logrus.Info("Created default admin user")
	}

	logrus.Info("Database initialized successfully")
	return db // Return configured database connection
}