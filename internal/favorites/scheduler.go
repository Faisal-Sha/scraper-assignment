// Package favorites implements the favorite product management service
// which tracks user favorites and schedules regular price checks
package favorites

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"scraper/internal/crawler"
	"scraper/internal/models"
)

// jobIDs maps job names to their cron entry IDs for management
var jobIDs = make(map[string]cron.EntryID)

// startScheduler initializes and starts a cron scheduler that periodically
// checks for price updates on favorited products.
//
// The scheduler runs every minute (* * * * *) and performs the following:
// 1. Fetches all favorited product IDs from the database
// 2. For each product, fetches latest details from Trendyol
// 3. Publishes updates to Kafka for price change analysis
//
// Parameters:
//   - db: Database connection for fetching favorite products
//   - producer: Kafka producer for publishing product updates
func startScheduler(db *gorm.DB, producer sarama.SyncProducer) {
	// Initialize cron scheduler
	c := cron.New()

	// Add job to run every minute
	id, err := c.AddFunc("* * * * *", func() {
		logrus.Info("Running scheduled task")

		// Fetch product IDs that need updating
		productIDs, err := fetchProductIDsFromDB(db)
		if err != nil {
			logrus.WithError(err).Error("Failed to fetch favorited product IDs")
			return
		}

		logrus.WithField("count", len(productIDs)).Info("Found active favorited products to update")
		if len(productIDs) > 0 {
			runTask(producer, productIDs)
		}
	})

	if err != nil {
		logrus.WithError(err).Fatal("Invalid cron expression")
	}

	// Store job ID for management
	jobIDs["productDetails"] = id

	// Start the scheduler
	c.Start()
	logrus.Info("Scheduler started for product details fetching")
}

// runTask executes the main product update workflow:
// 1. Fetches latest product details from Trendyol
// 2. Saves updates to local JSON file for backup
// 3. Converts data to internal models
// 4. Publishes updates to Kafka for processing
//
// Parameters:
//   - producer: Kafka producer for publishing updates
//   - productIDs: List of product IDs to fetch and update
func runTask(producer sarama.SyncProducer, productIDs []int) {
	logrus.WithField("time", time.Now()).Info("Running scheduled task")

	// Store fetched product details
	var newProducts []map[string]interface{}

	// Fetch latest details for each product
	logrus.WithField("count", len(productIDs)).Info("Fetching details for products")
	for _, productID := range productIDs {
		logrus.WithField("product_id", productID).Info("Fetching product")
		// Rate limit requests to avoid overwhelming the API
		time.Sleep(2 * time.Second)
		detail := crawler.FetchProductDetails(productID)
		if detail != nil {
			newProducts = append(newProducts, detail)
		}
	}

	// Skip processing if no products were fetched
	if len(newProducts) == 0 {
		logrus.Info("No new products fetched")
		return
	}

	// Save to local JSON file for backup
	filePath := "data.json"
	var existing []map[string]interface{}
	if file, err := os.ReadFile(filePath); err == nil {
		json.Unmarshal(file, &existing)
	}
	existing = append(existing, newProducts...)

	// Create/update the backup file
	file, err := os.Create(filePath)
	if err != nil {
		logrus.WithError(err).WithField("file", filePath).Error("Failed to open file")
		return
	}
	defer file.Close()

	// Write formatted JSON
	enc := json.NewEncoder(file)
	enc.SetIndent("", " ")
	if err := enc.Encode(existing); err != nil {
		logrus.WithError(err).Error("Failed to encode products to file")
		return
	}

	logrus.Info("Product details saved to data.json")

	// Convert raw data to internal models
	trendyolResp := make([]models.TrendyolResponse, len(newProducts))
	for i, p := range newProducts {
		data, _ := json.Marshal(p)
		var resp models.TrendyolResponse
		json.Unmarshal(data, &resp)
		trendyolResp[i] = resp
	}
	products := crawler.ConvertTrendyolToProduct(&trendyolResp)

	// Prepare data for Kafka
	productsJSON, err := json.Marshal(products)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal products for Kafka")
		return
	}

	// Create Kafka message
	msg := &sarama.ProducerMessage{
		Topic: "FAVORITE_PRODUCTS",
		Value: sarama.ByteEncoder(productsJSON),
	}
	if _, _, err := producer.SendMessage(msg); err != nil {
		logrus.WithError(err).Error("Failed to send message to Kafka")
	} else {
		logrus.Info("Products sent to Kafka topic FAVORITE_PRODUCTS")
	}
}

// fetchProductIDsFromDB retrieves IDs of all active products that are marked as favorites.
// It performs a JOIN between products and user_favorites tables to find products that:
// 1. Are marked as active (is_active = true)
// 2. Are marked as favorites (is_favorite = true)
// 3. Have at least one user who has favorited them
//
// Parameters:
//   - db: Database connection
//
// Returns:
//   - []int: List of product IDs that need price updates
//   - error: Database error if any occurred, or "no active favorited products" error if none found
func fetchProductIDsFromDB(db *gorm.DB) ([]int, error) {
	var productIDs []int

	// Query to find active favorited products
	result := db.Model(&models.Product{}).
		Select("DISTINCT products.id"). // Avoid duplicates if multiple users favorited same product
		Joins("JOIN user_favorites ON products.id = user_favorites.product_id"). // Only get favorited products
		Where("products.is_active = ? AND products.is_favorite = ?", true, true). // Only active and favorite-marked products
		Pluck("products.id", &productIDs)

	// Handle database errors
	if result.Error != nil {
		logrus.WithError(result.Error).Error("Failed to fetch product IDs")
		return nil, result.Error
	}

	// Return error if no products found
	if len(productIDs) == 0 {
		logrus.Info("No active favorited products found")
		return nil, fmt.Errorf("no active favorited products")
	}

	return productIDs, nil
}