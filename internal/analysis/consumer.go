// Package analysis implements the product analysis service that processes and categorizes products
package analysis

import (
	"encoding/json"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"scraper/internal/models"
)

// handleProducts creates a message handler for processing product updates.
// It analyzes incoming product data and determines whether products are:
// 1. New products to be created
// 2. Existing products that need updates
// 3. Favorited products that need special handling
// 4. Out-of-stock products that should be marked inactive
//
// The handler performs the following steps for each product:
// 1. Checks if the product exists in the database
// 2. For new products:
//   - Creates them in the database
//   - Checks if they are favorited by any users
// 3. For existing products:
//   - Checks stock status and marks inactive if out of stock
//   - Identifies if product is favorited and needs special handling
//   - Updates product details in the database
//
// Parameters:
//   - db: Database connection for product operations
//   - producer: Kafka producer for sending updates about favorited products
//
// Returns:
//   - func([]byte): Message handler function that processes product data
func handleProducts(db *gorm.DB, producer sarama.SyncProducer) func([]byte) {
	return func(data []byte) {
		logrus.Info("Product Analysis Service received product data")

		// Unmarshal incoming product data
		var products []models.Product
		if err := json.Unmarshal(data, &products); err != nil {
			logrus.WithError(err).Error("Error unmarshaling products")
			return
		}
		logrus.WithField("data", string(data)).Info("Received product data")

		// Track products that are favorited for special handling
		var favoritedProducts []models.Product

		// Process each product
		for _, p := range products {
			var existing models.Product
			result := db.First(&existing, p.ID)

			// Handle new products
			if result.Error != nil {
				if result.Error == gorm.ErrRecordNotFound {
					logrus.WithFields(logrus.Fields{
						"name": p.Name,
						"id":   p.ID,
					}).Info("New product detected")

					// Create new product
					db.Create(&p)

					// Check if new product is favorited
					var favoriteCount int64
					db.Model(&models.UserFavorite{}).Where("product_id = ?", p.ID).Count(&favoriteCount)
					if favoriteCount > 0 && p.IsActive && p.IsFavorite {
						favoritedProducts = append(favoritedProducts, p)
					}
				} else {
					logrus.WithError(result.Error).Error("Error checking existing product")
				}
			} else {
				// Handle existing products
				logrus.WithFields(logrus.Fields{
					"name": p.Name,
					"id":   p.ID,
				}).Info("Existing product detected")

				// Check stock status
				var stockInfo map[string]interface{}
				if err := json.Unmarshal(p.StockInfo, &stockInfo); err == nil {
					if stock, ok := stockInfo["stock"].(float64); ok && stock == 0 {
						logrus.WithFields(logrus.Fields{
							"name": p.Name,
							"id":   p.ID,
						}).Info("Product out of stock, marking inactive")
						p.IsActive = false
					}
				}

				// Check if product is favorited
				isFavorited := false
				if p.IsFavorite && p.IsActive {
					var favoriteCount int64
					db.Model(&models.UserFavorite{}).Where("product_id = ?", p.ID).Count(&favoriteCount)
					if favoriteCount > 0 {
						isFavorited = true
						logrus.WithFields(logrus.Fields{
							"name": p.Name,
							"id":   p.ID,
						}).Info("Favorited product, forwarding to Favorite Service")
						favoritedProducts = append(favoritedProducts, p)
					}
				}

				// Update regular products
				if !isFavorited {
					logrus.WithFields(logrus.Fields{
						"name": p.Name,
						"id":   p.ID,
					}).Info("Regular product, updating in DB")
					db.Model(&existing).Updates(map[string]interface{}{
						"name":                p.Name,
						"category_path":       p.CategoryPath,
						"images":              p.Images,
						"seller":              p.Seller,
						"brand":               p.Brand,
						"rating_score":        p.RatingScore,
						"favorites_count":     p.FavoritesCount,
						"views":               p.Views,
						"orders":              p.Orders,
						"stock_info":          p.StockInfo,
						"price_info":          p.PriceInfo,
						"attributes":          p.Attributes,
						"is_active":           p.IsActive,
						"is_favorite":         p.IsFavorite,
						"comments_count":      p.CommentsCount,
						"add_to_cart_events":  p.AddToCartEvents,
						"size_recommendation": p.SizeRecommendation,
						"estimated_delivery":  p.EstimatedDelivery,
						"other_sellers":       p.OtherSellers,
					})
				}
			}
		}

		if len(favoritedProducts) > 0 {
			logrus.WithField("count", len(favoritedProducts)).Info("Forwarding favorited products to Favorite Service")
			productsJSON, err := json.Marshal(favoritedProducts)
			if err != nil {
				logrus.WithError(err).Error("Error marshaling favorited products")
			} else {
				msg := &sarama.ProducerMessage{
					Topic: "FAVORITE_PRODUCTS",
					Value: sarama.ByteEncoder(productsJSON),
				}
				_, _, err := producer.SendMessage(msg)
				if err != nil {
					logrus.WithError(err).Error("Error sending favorited products to Kafka")
				} else {
					logrus.Info("Successfully forwarded favorited products")
				}
			}
		}
	}
}