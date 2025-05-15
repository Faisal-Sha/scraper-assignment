package favorites

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"scraper/internal/models"
	"scraper/internal/proto"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func handleFavorites(db *gorm.DB, producer sarama.SyncProducer) func([]byte) {
	return func(data []byte) {
		logrus.WithField("data", string(data)).Info("Received favorited product update")

		notificationGrpcPort := os.Getenv("NOTIFICATION_GRPC_PORT")
		if notificationGrpcPort == "" {
			notificationGrpcPort = "8083"
		}

		conn, err := grpc.DialContext(context.Background(),
			fmt.Sprintf("0.0.0.0:%s", notificationGrpcPort),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logrus.WithError(err).Fatal("Failed to connect to notification service")
			return
		}
		defer conn.Close()
		notificationClient := proto.NewNotificationServiceClient(conn)

		var products []models.Product
		if err := json.Unmarshal(data, &products); err != nil {
			logrus.WithError(err).Error("Error unmarshaling favorited products")
			return
		}

		for _, updatedProduct := range products {
			var existingProduct models.Product
			if err := db.First(&existingProduct, updatedProduct.ID).Error; err != nil {
				logrus.WithError(err).WithField("product_id", updatedProduct.ID).Error("Error finding existing product")
				continue
			}

			logrus.WithFields(logrus.Fields{
				"name": existingProduct.Name,
				"id":   existingProduct.ID,
			}).Info("Priority update for product")

			var oldPriceInfo, newPriceInfo map[string]interface{}
			var oldStockInfo, newStockInfo map[string]interface{}

			if err := json.Unmarshal(existingProduct.PriceInfo, &oldPriceInfo); err != nil {
				logrus.WithError(err).Error("Error unmarshaling old price")
				continue
			}
			if err := json.Unmarshal(updatedProduct.PriceInfo, &newPriceInfo); err != nil {
				logrus.WithError(err).Error("Error unmarshaling new price")
				continue
			}
			if err := json.Unmarshal(existingProduct.StockInfo, &oldStockInfo); err != nil {
				logrus.WithError(err).Error("Error unmarshaling old stock")
				continue
			}
			if err := json.Unmarshal(updatedProduct.StockInfo, &newStockInfo); err != nil {
				logrus.WithError(err).Error("Error unmarshaling new stock")
				continue
			}

			oldPrice, oldPriceOk := oldPriceInfo["originalPrice"].(float64)
			newPrice, newPriceOk := newPriceInfo["originalPrice"].(float64)
			oldStock, oldStockOk := oldStockInfo["stock"].(float64)
			newStock, newStockOk := newStockInfo["stock"].(float64)

			if !oldPriceOk || !newPriceOk || !oldStockOk || !newStockOk {
				logrus.Error("Could not extract price or stock values as float64")
				continue
			}

			if oldPrice != newPrice || oldStock != newStock {
				logrus.WithFields(logrus.Fields{
					"old_price": oldPrice,
					"new_price": newPrice,
					"old_stock": oldStock,
					"new_stock": newStock,
				}).Info("Change detected")

				priceLog := models.PriceStockLog{
					ProductID:  existingProduct.ID,
					OldPrice:   fmt.Sprintf("%.2f", oldPrice),
					NewPrice:   fmt.Sprintf("%.2f", newPrice),
					OldStock:   fmt.Sprintf("%.0f", oldStock),
					NewStock:   fmt.Sprintf("%.0f", newStock),
					ChangeTime: time.Now(),
				}
				db.Create(&priceLog)

				db.Model(&existingProduct).Updates(map[string]interface{}{
					"price_info": updatedProduct.PriceInfo,
					"stock_info": updatedProduct.StockInfo,
				})

				if newPrice < oldPrice {
					logrus.WithField("name", existingProduct.Name).Info("Price drop detected")

					var favorites []models.UserFavorite
					if err := db.Where("product_id = ?", existingProduct.ID).Find(&favorites).Error; err != nil {
						logrus.WithError(err).WithField("product_id", existingProduct.ID).Error("Error finding favorites")
						continue
					}

					logrus.WithField("count", len(favorites)).Info("Found users to notify")
					for _, favorite := range favorites {
						var user models.User
						if err := db.First(&user, favorite.UserID).Error; err != nil {
							logrus.WithError(err).WithField("user_id", favorite.UserID).Error("Error finding user")
							continue
						}

						logrus.WithField("email", user.Email).Info("Sending notification about price drop")
						_, err := notificationClient.SendNotification(context.Background(),
							&proto.NotificationRequest{
								UserId:    fmt.Sprintf("%d", user.ID),
								ProductId: uint32(existingProduct.ID),
								Message:   fmt.Sprintf("Price dropped from %.2f to %.2f for %s!", oldPrice, newPrice, existingProduct.Name),
							})
						if err != nil {
							logrus.WithError(err).Error("Error sending notification")
						} else {
							logrus.WithField("user_id", user.ID).Info("Successfully sent notification")
						}
					}
				}
			} else {
				logrus.WithField("name", existingProduct.Name).Info("No changes detected")
			}
		}
	}
}