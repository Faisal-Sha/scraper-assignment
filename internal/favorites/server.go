// Package favorites implements the favorite product management service
package favorites

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"scraper/internal/db"
	"scraper/internal/kafka"
)

// Start initializes and runs the favorite product service.
// This service is responsible for:
// 1. Running a periodic scheduler that checks favorite products for price updates
// 2. Consuming Kafka messages about price changes and notifying users
// 3. Providing a health check endpoint for monitoring
//
// The service performs the following setup:
// - Initializes database connection
// - Sets up Kafka producer for sending price updates
// - Creates HTTP server with health check endpoint
// - Starts the product update scheduler
// - Sets up Kafka consumer for processing price changes
//
// Environment Variables:
//   - FAVORITE_PORT: Port for the HTTP server (default: 8084)
//   - KAFKA_FAVORITES_TOPIC: Kafka topic for favorite product updates (default: FAVORITE_PRODUCTS)
func Start() {
	// Initialize database and Kafka producer
	dbConn := db.Setup()
	producer := kafka.SetupProducer()

	// Setup HTTP server with health check
	e := echo.New()
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Get service port from environment
	port := os.Getenv("FAVORITE_PORT")
	if port == "" {
		port = "8084" // Default port
	}

	// Start HTTP server in a goroutine
	logrus.WithField("port", port).Info("Starting Favorite Product Service")
	go func() {
		if err := e.Start("0.0.0.0:" + port); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Favorite service shutdown")
		}
	}()

	// Start the scheduler that periodically checks favorite products
	startScheduler(dbConn, producer)

	// Setup Kafka consumer for processing price updates
	favoritesTopic := os.Getenv("KAFKA_FAVORITES_TOPIC")
	if favoritesTopic == "" {
		favoritesTopic = "FAVORITE_PRODUCTS" // Default topic
	}
	kafka.SetupConsumer(favoritesTopic, handleFavorites(dbConn, producer))
}