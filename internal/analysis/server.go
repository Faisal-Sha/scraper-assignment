package analysis

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

	port := os.Getenv("ANALYZER_PORT")
	if port == "" {
		port = "8085"
	}

	logrus.WithField("port", port).Info("Starting Product Analysis Service")
	go func() {
		if err := e.Start("0.0.0.0:" + port); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("Analyzer service shutdown")
		}
	}()

	productsTopic := os.Getenv("KAFKA_PRODUCTS_TOPIC")
	if productsTopic == "" {
		productsTopic = "PRODUCTS"
	}

	kafka.SetupConsumer(productsTopic, handleProducts(dbConn, producer))
}