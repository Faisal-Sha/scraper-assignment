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

		var priceUpdate struct {
			UserID    uint    `json:"user_id"`
			ProductID uint    `json:"product_id"`
			OldPrice  float64 `json:"old_price"`
			NewPrice  float64 `json:"new_price"`
		}
		if err := json.Unmarshal(data, &priceUpdate); err != nil {
			logrus.WithError(err).Error("Error unmarshaling price update")
			return
		}

		// Get product details
		var product models.Product
		if err := db.First(&product, priceUpdate.ProductID).Error; err != nil {
			logrus.WithError(err).Error("Failed to find product")
			return
		}

		// Send notification to user
		_, err = notificationClient.SendNotification(context.Background(), &proto.NotificationRequest{
			UserId:    fmt.Sprintf("%d", priceUpdate.UserID),
			ProductId: uint32(priceUpdate.ProductID),
			Message:   fmt.Sprintf("Price dropped from %.2f to %.2f for %s", priceUpdate.OldPrice, priceUpdate.NewPrice, product.Name),
		})
		if err != nil {
			logrus.WithError(err).Error("Failed to send notification")
			return
		}

		// Record price history
		priceLog := models.PriceStockLog{
			ProductID:  priceUpdate.ProductID,
			OldPrice:   fmt.Sprintf("%.2f", priceUpdate.OldPrice),
			NewPrice:   fmt.Sprintf("%.2f", priceUpdate.NewPrice),
			ChangeTime: time.Now(),
		}

		if err := db.Create(&priceLog).Error; err != nil {
			logrus.WithError(err).Error("Failed to create price log")
		}
	}
}