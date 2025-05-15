// Package favorites handles processing of favorite products and price notifications
package favorites

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	// Kafka client for message processing
	"github.com/IBM/sarama"
	// gRPC for notification service communication
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Internal packages
	"scraper/internal/models"
	"scraper/internal/proto"

	// Logging and database
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// handleFavorites creates a message handler for processing favorite product updates.
// It takes a database connection and Kafka producer as input and returns a function
// that processes incoming messages about price changes for favorited products.
//
// The handler performs the following steps:
// 1. Connects to the notification service via gRPC
// 2. Unmarshals the price update data
// 3. Retrieves product details from the database
// 4. Sends a notification to the user about the price change
// 5. Records the price change in the price history log
func handleFavorites(db *gorm.DB, producer sarama.SyncProducer) func([]byte) {
	return func(data []byte) {
		// Log received data for debugging
		logrus.WithField("data", string(data)).Info("Received favorited product update")

		// Get notification service port from environment or use default
		notificationGrpcPort := os.Getenv("NOTIFICATION_GRPC_PORT")
		if notificationGrpcPort == "" {
			notificationGrpcPort = "8083" // Default notification service port
		}

		// Establish gRPC connection to notification service
		conn, err := grpc.DialContext(context.Background(),
			fmt.Sprintf("0.0.0.0:%s", notificationGrpcPort),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logrus.WithError(err).Fatal("Failed to connect to notification service")
			return
		}
		defer conn.Close()
		notificationClient := proto.NewNotificationServiceClient(conn)

		// Define price update structure and unmarshal data
		var priceUpdate struct {
			UserID    uint    `json:"user_id"`    // ID of the user who favorited the product
			ProductID uint    `json:"product_id"` // ID of the product with price change
			OldPrice  float64 `json:"old_price"`  // Previous price of the product
			NewPrice  float64 `json:"new_price"`  // New price of the product
		}
		if err := json.Unmarshal(data, &priceUpdate); err != nil {
			logrus.WithError(err).Error("Error unmarshaling price update")
			return
		}

		// Retrieve product details from database
		var product models.Product
		if err := db.First(&product, priceUpdate.ProductID).Error; err != nil {
			logrus.WithError(err).Error("Failed to find product")
			return
		}

		// Send price drop notification to user via gRPC
		_, err = notificationClient.SendNotification(context.Background(), &proto.NotificationRequest{
			UserId:    fmt.Sprintf("%d", priceUpdate.UserID),
			ProductId: uint32(priceUpdate.ProductID),
			Message:   fmt.Sprintf("Price dropped from %.2f to %.2f for %s", priceUpdate.OldPrice, priceUpdate.NewPrice, product.Name),
		})
		if err != nil {
			logrus.WithError(err).Error("Failed to send notification")
			return
		}

		// Record price change in history log
		priceLog := models.PriceStockLog{
			ProductID:  priceUpdate.ProductID,
			OldPrice:   fmt.Sprintf("%.2f", priceUpdate.OldPrice),
			NewPrice:   fmt.Sprintf("%.2f", priceUpdate.NewPrice),
			ChangeTime: time.Now(), // Record exact time of price change
		}

		// Save price change to database
		if err := db.Create(&priceLog).Error; err != nil {
			logrus.WithError(err).Error("Failed to create price log")
		}
	}
}