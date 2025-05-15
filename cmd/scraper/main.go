package main

import (
	"scraper/internal/analysis"
	"scraper/internal/crawler"
	"scraper/internal/favorites"
	"scraper/internal/notification"
	"scraper/pkg/config"
	"scraper/pkg/logger"

	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger.Init()

	// Load configuration
	if err := config.Load(); err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// Start services in separate goroutines
	go crawler.Start()
	go analysis.Start()
	go favorites.Start()
	go notification.Start()

	logrus.Info("Application started")

	// Keep the application running
	select {}
}