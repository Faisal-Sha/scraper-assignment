package crawler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"

	"scraper/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func registerHandlers(e *echo.Echo, db *gorm.DB, producer sarama.SyncProducer) {
	validate := validator.New()

	e.GET("/fetch", func(c echo.Context) error {
		var req struct {
			Flag bool `json:"flag"`
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid fetch request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		if req.Flag {
			client := &http.Client{}
			start := 94
			end := 200
			file, err := os.Create("data.json")
			if err != nil {
				logrus.WithError(err).Error("Failed to create data.json")
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create JSON file"})
			}
			defer file.Close()
			file.WriteString("[\n")
			first := true
			for wc := start; wc <= end; wc++ {
				logrus.WithField("wc", wc).Info("Fetching products")
				url := fmt.Sprintf("https://apigw.trendyol.com/discovery-sfint-browsing-service/api/search-feed/products?source=sr?wc=%d&size=60", wc)
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					logrus.WithError(err).Error("Failed to create HTTP request")
					continue
				}
				// Set headers (same as in original main.go)
				req.Header.Set("accept", "application/json")
				// ... (add all headers from original fetch logic)
				resp, err := client.Do(req)
				if err != nil {
					logrus.WithError(err).Error("Failed to fetch products")
					continue
				}
				defer resp.Body.Close()
				bodyText, err := io.ReadAll(resp.Body)
				if err != nil {
					logrus.WithError(err).Error("Failed to read response body")
					continue
				}
				var result models.Root
				if err := json.Unmarshal(bodyText, &result); err != nil {
					logrus.WithError(err).Error("Failed to unmarshal response")
					continue
				}
				if len(result.Data.Contents) == 0 {
					logrus.Info("No more products found")
					continue
				}
				for _, p := range result.Data.Contents {
					time.Sleep(4 * time.Second) // Rate limiting
					logrus.WithField("product_id", p.ID).Info("Fetching product details")
					detailedProduct := FetchProductDetails(p.ID)
					encoder := json.NewEncoder(file)
					if !first {
						file.WriteString(",\n")
					}
					first = false
					if err := encoder.Encode(detailedProduct); err != nil {
						logrus.WithError(err).WithField("product_id", p.ID).Error("Failed to write product to file")
						continue
					}
				}
			}
			file.WriteString("\n]")
		}

		mockProducts, err := readMockData()
		if err != nil {
			logrus.WithError(err).Error("Failed to read mock data")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to read mock data: %v", err)})
		}
		productsJSON, err := json.Marshal(mockProducts)
		if err != nil {
			logrus.WithError(err).Error("Failed to marshal products")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to marshal products"})
		}

		msg := &sarama.ProducerMessage{
			Topic: "PRODUCTS",
			Value: sarama.ByteEncoder(productsJSON),
		}
		if _, _, err := producer.SendMessage(msg); err != nil {
			logrus.WithError(err).Error("Failed to send message to Kafka")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to send message to Kafka"})
		}

		logrus.Info("Products fetched and sent to Kafka")
		return c.JSON(http.StatusOK, map[string]string{"status": "Products fetched and sent to Kafka"})
	})

	e.POST("/favorites", func(c echo.Context) error {
		var req struct {
			UserID    uint `json:"user_id" validate:"required"`
			ProductID uint `json:"product_id" validate:"required"`
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid favorites request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		if err := validate.Struct(&req); err != nil {
			logrus.WithError(err).Error("Validation failed for favorites request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		if err := AddFavorite(db, req.UserID, req.ProductID); err != nil {
			logrus.WithError(err).Error("Failed to add favorite")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to add favorite"})
		}

		logrus.WithFields(logrus.Fields{"user_id": req.UserID, "product_id": req.ProductID}).Info("Product added to favorites")
		return c.JSON(http.StatusOK, map[string]string{"status": "Product added to favorites"})
	})

	e.DELETE("/favorites", func(c echo.Context) error {
		var req struct {
			UserID    uint `json:"user_id" validate:"required"`
			ProductID uint `json:"product_id" validate:"required"`
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid favorites deletion request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		if err := validate.Struct(&req); err != nil {
			logrus.WithError(err).Error("Validation failed for favorites deletion")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		if err := RemoveFavorite(db, req.UserID, req.ProductID); err != nil {
			logrus.WithError(err).Error("Failed to remove favorite")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to remove favorite"})
		}

		logrus.WithFields(logrus.Fields{"user_id": req.UserID, "product_id": req.ProductID}).Info("Product removed from favorites")
		return c.JSON(http.StatusOK, map[string]string{"status": "Product removed from favorites"})
	})

	e.GET("/favorites/:user_id", func(c echo.Context) error {
		userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
		if err != nil {
			logrus.WithError(err).Error("Invalid user ID")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		}

		favorites, err := GetUserFavorites(db, uint(userID))
		if err != nil {
			logrus.WithError(err).Error("Failed to get favorites")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get favorites"})
		}

		logrus.WithField("user_id", userID).Info("Fetched user favorites")
		return c.JSON(http.StatusOK, favorites)
	})

	e.POST("/users", func(c echo.Context) error {
		var req struct {
			Email    string `json:"email" validate:"required,email"`
			Username string `json:"username" validate:"required"`
			Password string `json:"password" validate:"required,min=6"`
			Name     string `json:"name" validate:"required"`
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid user creation request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		if err := validate.Struct(&req); err != nil {
			logrus.WithError(err).Error("Validation failed for user creation")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		var count int64
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			logrus.WithField("email", req.Email).Error("User with this email already exists")
			return c.JSON(http.StatusConflict, map[string]string{"error": "User with this email already exists"})
		}
		db.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
		if count > 0 {
			logrus.WithField("username", req.Username).Error("Username is already taken")
			return c.JSON(http.StatusConflict, map[string]string{"error": "Username is already taken"})
		}

		user := models.User{
			Email:       req.Email,
			Username:    req.Username,
			Password:    req.Password, // TODO: Hash password in production
			Name:        req.Name,
			IsActive:    true,
			LastLoginAt: time.Now(),
		}
		result := db.Create(&user)
		if result.Error != nil {
			logrus.WithError(result.Error).Error("Failed to create user")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
		}

		user.Password = ""
		logrus.WithField("user_id", user.ID).Info("User created successfully")
		return c.JSON(http.StatusCreated, user)
	})

	e.GET("/users/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			logrus.WithError(err).Error("Invalid user ID")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		}

		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			logrus.WithError(err).Error("User not found")
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}

		user.Password = ""
		logrus.WithField("user_id", id).Info("Fetched user details")
		return c.JSON(http.StatusOK, user)
	})
}