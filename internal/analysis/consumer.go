package analysis

import (
	"encoding/json"
	"github.com/IBM/sarama"
	"gorm.io/gorm"
	"scraper/internal/models"
	"github.com/sirupsen/logrus"
)

func handleProducts(db *gorm.DB, producer sarama.SyncProducer) func([]byte) {
	return func(data []byte) {
		logrus.Info("Product Analysis Service received product data")
		var products []models.Product
		if err := json.Unmarshal(data, &products); err != nil {
			logrus.WithError(err).Error("Error unmarshaling products")
			return
		}
		logrus.WithField("data", string(data)).Info("Received product data")
		var favoritedProducts []models.Product

		for _, p := range products {
			var existing models.Product
			result := db.First(&existing, p.ID)

			if result.Error != nil {
				if result.Error == gorm.ErrRecordNotFound {
					logrus.WithFields(logrus.Fields{
						"name": p.Name,
						"id":   p.ID,
					}).Info("New product detected")
					db.Create(&p)

					var favoriteCount int64
					db.Model(&models.UserFavorite{}).Where("product_id = ?", p.ID).Count(&favoriteCount)
					if favoriteCount > 0 && p.IsActive && p.IsFavorite {
						favoritedProducts = append(favoritedProducts, p)
					}
				} else {
					logrus.WithError(result.Error).Error("Error checking existing product")
				}
			} else {
				logrus.WithFields(logrus.Fields{
					"name": p.Name,
					"id":   p.ID,
				}).Info("Existing product detected")

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