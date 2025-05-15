package favorites

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"scraper/internal/crawler"
	"scraper/internal/models"

	"github.com/sirupsen/logrus"
)

var jobIDs = make(map[string]cron.EntryID)

func startScheduler(db *gorm.DB, producer sarama.SyncProducer) {
	c := cron.New()
	id, err := c.AddFunc("* * * * *", func() {
		logrus.Info("Running scheduled task")
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
	jobIDs["productDetails"] = id
	c.Start()
	logrus.Info("Scheduler started for product details fetching")
}

func runTask(producer sarama.SyncProducer, productIDs []int) {
	logrus.WithField("time", time.Now()).Info("Running scheduled task")
	var newProducts []map[string]interface{}

	logrus.WithField("count", len(productIDs)).Info("Fetching details for products")
	for _, productID := range productIDs {
		logrus.WithField("product_id", productID).Info("Fetching product")
		time.Sleep(2 * time.Second)
		detail := crawler.FetchProductDetails(productID)
		if detail != nil {
			newProducts = append(newProducts, detail)
		}
	}

	if len(newProducts) == 0 {
		logrus.Info("No new products fetched")
		return
	}

	filePath := "data.json"
	var existing []map[string]interface{}
	if file, err := os.ReadFile(filePath); err == nil {
		json.Unmarshal(file, &existing)
	}
	existing = append(existing, newProducts...)

	file, err := os.Create(filePath)
	if err != nil {
		logrus.WithError(err).WithField("file", filePath).Error("Failed to open file")
		return
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", " ")
	if err := enc.Encode(existing); err != nil {
		logrus.WithError(err).Error("Failed to encode products to file")
		return
	}

	logrus.Info("Product details saved to data.json")

	trendyolResp := make([]models.TrendyolResponse, len(newProducts))
	for i, p := range newProducts {
		data, _ := json.Marshal(p)
		var resp models.TrendyolResponse
		json.Unmarshal(data, &resp)
		trendyolResp[i] = resp
	}
	products := crawler.ConvertTrendyolToProduct(&trendyolResp)
	productsJSON, err := json.Marshal(products)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal products for Kafka")
		return
	}

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

func fetchProductIDsFromDB(db *gorm.DB) ([]int, error) {
	var productIDs []int
	result := db.Model(&models.Product{}).
		Select("DISTINCT products.id").
		Joins("JOIN user_favorites ON products.id = user_favorites.product_id").
		Where("products.is_active = ? AND products.is_favorite = ?", true, true).
		Pluck("products.id", &productIDs)

	if result.Error != nil {
		logrus.WithError(result.Error).Error("Failed to fetch product IDs")
		return nil, result.Error
	}
	if len(productIDs) == 0 {
		logrus.Info("No active favorited products found")
		return nil, fmt.Errorf("no active favorited products")
	}
	return productIDs, nil
}