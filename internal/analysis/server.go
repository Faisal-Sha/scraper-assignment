// Package analysis implements the product analysis service which processes product data
// from Kafka, analyzes changes, and triggers notifications for price drops
package analysis

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"scraper/internal/db"
	"scraper/internal/kafka"
)

// Start initializes and runs the product analysis service. It:
// 1. Sets up database connection and Kafka producer
// 2. Initializes HTTP server with health check endpoint
// 3. Starts consuming product messages from Kafka
//
// The service listens on ANALYZER_PORT (default: 8085) and consumes messages
// from KAFKA_PRODUCTS_TOPIC (default: PRODUCTS)
func Start() {
	// Initialize database connection
	dbConn := db.Setup()

	// Set up Kafka producer for sending price drop notifications
	producer := kafka.SetupProducer()

	// Initialize Echo HTTP server
	e := echo.New()

	// Register health check endpoint
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Get service port from environment or use default
	port := os.Getenv("ANALYZER_PORT")
	if port == "" {
		port = "8085" // Default port if not specified
	}

	// Start HTTP server in a goroutine
	logrus.WithField("port", port).Info("Starting Product Analysis Service")
	go func() {
		if err := e.Start("0.0.0.0:" + port); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Analyzer service shutdown")
		}
	}()

	// Get Kafka topic from environment or use default
	productsTopic := os.Getenv("KAFKA_PRODUCTS_TOPIC")
	if productsTopic == "" {
		productsTopic = "PRODUCTS" // Default topic if not specified
	}

	// Start consuming product messages from Kafka
	// handleProducts processes each message for price/stock analysis
	kafka.SetupConsumer(productsTopic, handleProducts(dbConn, producer))
}