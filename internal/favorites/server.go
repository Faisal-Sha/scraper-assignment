package favorites

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"scraper/internal/db"
	"scraper/internal/kafka"

	"github.com/sirupsen/logrus"
)

func Start() {
	dbConn := db.Setup()
	producer := kafka.SetupProducer()

	e := echo.New()
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	port := os.Getenv("FAVORITE_PORT")
	if port == "" {
		port = "8084"
	}

	logrus.WithField("port", port).Info("Starting Favorite Product Service")
	go func() {
		if err := e.Start("0.0.0.0:" + port); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Favorite service shutdown")
		}
	}()

	// Start scheduler
	startScheduler(dbConn, producer)

	// Setup Kafka consumer
	favoritesTopic := os.Getenv("KAFKA_FAVORITES_TOPIC")
	if favoritesTopic == "" {
		favoritesTopic = "FAVORITE_PRODUCTS"
	}
	kafka.SetupConsumer(favoritesTopic, handleFavorites(dbConn, producer))
}