// Package crawler implements the product crawler service which fetches product data
// from Trendyol's API and publishes it to Kafka for further processing
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
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"scraper/internal/models"
)

// registerHandlers sets up all HTTP endpoints for the crawler service.
// It configures routes for:
// - Simulating price drops (for testing)
// - Manually triggering crawls
// - Health checks
// - Product updates
//
// Parameters:
//   - e: Echo instance for HTTP routing
//   - db: Database connection for product operations
//   - producer: Kafka producer for publishing updates
func registerHandlers(e *echo.Echo, db *gorm.DB, producer sarama.SyncProducer) {
	// POST /simulate-price-drop
	// Simulates a price drop for a product to test the notification system
	// Request body: {"product_id": uint, "new_price": float64}
	e.POST("/simulate-price-drop", func(c echo.Context) error {
		// Parse and validate request
		var req struct {
			ProductID uint    `json:"product_id" validate:"required"`
			NewPrice  float64 `json:"new_price" validate:"required,gt=0"`
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid price drop simulation request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		// Get current product price from database
		var product models.Product
		if err := db.First(&product, req.ProductID).Error; err != nil {
			logrus.WithError(err).Error("Failed to find product")
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Product not found"})
		}

		// Store old price for comparison
		oldPrice := product.Price
		
		// Update product price in database
		product.Price = req.NewPrice
		priceInfo := fmt.Sprintf(`{"currency": "TRY", "original": %f}`, req.NewPrice)
		product.PriceInfo = datatypes.JSON([]byte(priceInfo))
		// Save updated product to database
		if err := db.Save(&product).Error; err != nil {
			logrus.WithError(err).Error("Failed to update product price")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update price"})
		}

		// Record price change in price history table
		// This helps track price fluctuations over time
		if err := db.Exec("INSERT INTO price_history (product_id, old_price, new_price) VALUES (?, ?, ?)", 
			req.ProductID, oldPrice, req.NewPrice).Error; err != nil {
			logrus.WithError(err).Error("Failed to record price history")
		}

		// Find all users who have favorited this product
		// These users will receive price drop notifications
		var favorites []models.UserFavorite
		if err := db.Where("product_id = ?", req.ProductID).Find(&favorites).Error; err != nil {
			logrus.WithError(err).Error("Failed to find favorites")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to find favorites"})
		}

		// Send price drop notifications to each user via Kafka
		// The notification service will consume these messages and send emails
		for _, fav := range favorites {
			msg := &sarama.ProducerMessage{
				Topic: "FAVORITE_PRODUCTS",
				Key:   sarama.StringEncoder(fmt.Sprintf("%d", fav.UserID)), // User ID as key for partitioning
				Value: sarama.StringEncoder(fmt.Sprintf(`{"user_id":%d,"product_id":%d,"old_price":%f,"new_price":%f}`, 
					fav.UserID, req.ProductID, oldPrice, req.NewPrice)),
			}
			_, _, err := producer.SendMessage(msg)
			if err != nil {
				logrus.WithError(err).Error("Failed to send notification message")
			}
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "Price updated and notifications sent"})
	})

	// Initialize validator for request validation
	validate := validator.New()

	// GET /fetch
	// Fetches products from Trendyol API and publishes them to Kafka
	// Query parameters:
	//   - flag: If true, fetches live data from API. If false, uses mock data.
	e.GET("/fetch", func(c echo.Context) error {
		// Parse and validate request
		var req struct {
			Flag bool `json:"flag" validate:"required"` // Whether to fetch live data
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid fetch request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		// If flag is true, fetch live data from Trendyol API
		if req.Flag {
			// Initialize HTTP client for API requests
			client := &http.Client{}

			// Define category range to fetch (wc = web category)
			start := 94  // Starting category ID
			end := 200   // Ending category ID

			// Create file to store raw product data
			file, err := os.Create("data.json")
			if err != nil {
				logrus.WithError(err).Error("Failed to create data.json")
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create JSON file"})
			}
			defer file.Close()

			// Initialize JSON array in file
			file.WriteString("[\n")
			first := true // Track first item for JSON formatting

			// Iterate through each category
			for wc := start; wc <= end; wc++ {
				logrus.WithField("wc", wc).Info("Fetching products")

				// Construct API URL for category products
				url := fmt.Sprintf("https://apigw.trendyol.com/discovery-sfint-browsing-service/api/search-feed/products?source=sr?wc=%d&size=60", wc)
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					logrus.WithError(err).Error("Failed to create HTTP request")
					continue
				}

				// Set required headers for API request
				req.Header.Set("accept", "application/json")
				// TODO: Add all required headers from original fetch logic
				// req.Header.Set("User-Agent", "Mozilla/5.0...")
				// req.Header.Set("Referer", "https://www.trendyol.com/")
				// etc.

				// Execute request
				resp, err := client.Do(req)
				if err != nil {
					logrus.WithError(err).Error("Failed to fetch products")
					continue
				}
				defer resp.Body.Close()

				// Read response body
				bodyText, err := io.ReadAll(resp.Body)
				if err != nil {
					logrus.WithError(err).Error("Failed to read response body")
					continue
				}

				// Parse JSON response
				var result models.Root
				if err := json.Unmarshal(bodyText, &result); err != nil {
					logrus.WithError(err).Error("Failed to unmarshal response")
					continue
				}

				// Skip if no products found in category
				if len(result.Data.Contents) == 0 {
					logrus.Info("No more products found")
					continue
				}

				// Process each product in category
				for _, p := range result.Data.Contents {
					// Respect rate limits
					time.Sleep(4 * time.Second)

					// Fetch detailed product information
					logrus.WithField("product_id", p.ID).Info("Fetching product details")
					detailedProduct := FetchProductDetails(p.ID)

					// Write product to file with proper JSON formatting
					encoder := json.NewEncoder(file)
					if !first {
						file.WriteString(",\n")
					}
					first = false

					// Encode and write product data
					if err := encoder.Encode(detailedProduct); err != nil {
						logrus.WithError(err).WithField("product_id", p.ID).Error("Failed to write product to file")
						continue
					}
				}
			}

			// Close JSON array
			file.WriteString("\n]")
		}

		// Read mock product data from file
		mockProducts, err := readMockData()
		if err != nil {
			logrus.WithError(err).Error("Failed to read mock data")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to read mock data: %v", err)})
		}

		// Process products in batches to avoid overwhelming Kafka
		batchSize := 50 // Maximum products per batch
		totalProducts := len(mockProducts)
		
		// Iterate through products in batches
		for i := 0; i < totalProducts; i += batchSize {
			// Calculate end index for current batch
			end := i + batchSize
			if end > totalProducts {
				end = totalProducts
			}
			
			// Get current batch of products
			batch := mockProducts[i:end]

			// Convert batch to JSON for Kafka message
			productsJSON, err := json.Marshal(batch)
			if err != nil {
				logrus.WithError(err).Error("Failed to marshal products batch")
				continue
			}
			
			// Prepare Kafka message
			msg := &sarama.ProducerMessage{
				Topic: "PRODUCTS", // Topic for product updates
				Value: sarama.ByteEncoder(productsJSON),
			}
			
			// Send batch to Kafka
			if _, _, err := producer.SendMessage(msg); err != nil {
				logrus.WithError(err).WithField("batch_start", i).WithField("batch_end", end).Error("Failed to send batch to Kafka")
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to send message to Kafka"})
			}
			
			// Log successful batch processing
			logrus.WithFields(logrus.Fields{
				"batch_start": i,
				"batch_end":   end,
				"batch_size":  len(batch),
			}).Info("Batch sent to Kafka")
			
			// Rate limiting between batches
			time.Sleep(500 * time.Millisecond)
		}

		logrus.Info("Products fetched and sent to Kafka")
		return c.JSON(http.StatusOK, map[string]string{"status": "Products fetched and sent to Kafka"})
	})

	// POST /favorites
	// Adds a product to a user's favorites list
	// Request body: {"user_id": uint, "product_id": uint}
	e.POST("/favorites", func(c echo.Context) error {
		// Parse and validate request
		var req struct {
			UserID    uint `json:"user_id" validate:"required"` // ID of the user adding favorite
			ProductID uint `json:"product_id" validate:"required"` // ID of product to favorite
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid favorites request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		// Validate required fields
		if err := validate.Struct(&req); err != nil {
			logrus.WithError(err).Error("Validation failed for favorites request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		// Add product to user's favorites
		if err := AddFavorite(db, req.UserID, req.ProductID); err != nil {
			logrus.WithError(err).Error("Failed to add favorite")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to add favorite"})
		}

		// Log successful addition
		logrus.WithFields(logrus.Fields{"user_id": req.UserID, "product_id": req.ProductID}).Info("Product added to favorites")
		return c.JSON(http.StatusOK, map[string]string{"status": "Product added to favorites"})
	})

	// DELETE /favorites
	// Removes a product from a user's favorites list
	// Request body: {"user_id": uint, "product_id": uint}
	e.DELETE("/favorites", func(c echo.Context) error {
		// Parse and validate request
		var req struct {
			UserID    uint `json:"user_id" validate:"required"` // ID of the user removing favorite
			ProductID uint `json:"product_id" validate:"required"` // ID of product to remove
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid favorites deletion request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		// Validate required fields
		if err := validate.Struct(&req); err != nil {
			logrus.WithError(err).Error("Validation failed for favorites deletion")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		// Remove product from user's favorites
		if err := RemoveFavorite(db, req.UserID, req.ProductID); err != nil {
			logrus.WithError(err).Error("Failed to remove favorite")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to remove favorite"})
		}

		// Log successful removal
		logrus.WithFields(logrus.Fields{"user_id": req.UserID, "product_id": req.ProductID}).Info("Product removed from favorites")
		return c.JSON(http.StatusOK, map[string]string{"status": "Product removed from favorites"})
	})

	// GET /favorites/:user_id
	// Retrieves all favorite products for a given user
	// URL parameters:
	//   - user_id: ID of the user whose favorites to retrieve
	e.GET("/favorites/:user_id", func(c echo.Context) error {
		// Parse and validate user ID from URL
		userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
		if err != nil {
			logrus.WithError(err).Error("Invalid user ID")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		}

		// Get user's favorite products
		favorites, err := GetUserFavorites(db, uint(userID))
		if err != nil {
			logrus.WithError(err).Error("Failed to get favorites")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get favorites"})
		}

		// Return list of favorites
		logrus.WithField("user_id", userID).Info("Fetched user favorites")
		return c.JSON(http.StatusOK, favorites)
	})

	// POST /users
	// Creates a new user account
	// Request body: {"email": string, "username": string, "password": string, "name": string}
	e.POST("/users", func(c echo.Context) error {
		// Parse and validate request
		var req struct {
			Email    string `json:"email" validate:"required,email"` // User's email (must be unique)
			Username string `json:"username" validate:"required"` // Username (must be unique)
			Password string `json:"password" validate:"required,min=6"` // Password (min 6 chars)
			Name     string `json:"name" validate:"required"` // User's full name
		}
		if err := c.Bind(&req); err != nil {
			logrus.WithError(err).Error("Invalid user creation request")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		// Validate required fields and formats
		if err := validate.Struct(&req); err != nil {
			logrus.WithError(err).Error("Validation failed for user creation")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		// Check for existing user with same email
		var count int64
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			logrus.WithField("email", req.Email).Error("User with this email already exists")
			return c.JSON(http.StatusConflict, map[string]string{"error": "User with this email already exists"})
		}

		// Check for existing user with same username
		db.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
		if count > 0 {
			logrus.WithField("username", req.Username).Error("Username is already taken")
			return c.JSON(http.StatusConflict, map[string]string{"error": "Username is already taken"})
		}

		// Create new user
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

		// Clear password before returning user data
		user.Password = ""
		logrus.WithField("user_id", user.ID).Info("User created successfully")
		return c.JSON(http.StatusCreated, user)
	})

	// GET /users/:id
	// Retrieves user details by ID
	// URL parameters:
	//   - id: ID of the user to retrieve
	e.GET("/users/:id", func(c echo.Context) error {
		// Parse and validate user ID from URL
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			logrus.WithError(err).Error("Invalid user ID")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		}

		// Get user from database
		var user models.User
		if err := db.First(&user, id).Error; err != nil {
			logrus.WithError(err).Error("User not found")
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}

		// Clear password before returning user data
		user.Password = ""
		logrus.WithField("user_id", id).Info("Fetched user details")
		return c.JSON(http.StatusOK, user)
	})
}
