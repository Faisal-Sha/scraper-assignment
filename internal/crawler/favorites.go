// Package crawler implements product data fetching and favorite product management
package crawler

import (
	"scraper/internal/models"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AddFavorite creates a new favorite relationship between a user and a product.
// It records the time when the product was favorited.
//
// Parameters:
//   - db: Database connection
//   - userID: ID of the user adding the favorite
//   - productID: ID of the product being favorited
//
// Returns:
//   - error: Any database error that occurred, nil if successful
func AddFavorite(db *gorm.DB, userID, productID uint) error {
	// Create new favorite record
	favorite := models.UserFavorite{
		UserID:    userID,
		ProductID: productID,
		AddedAt:   time.Now(),
	}

	// Attempt to save to database
	result := db.Create(&favorite)
	if result.Error != nil {
		logrus.WithError(result.Error).WithFields(logrus.Fields{
			"user_id":    userID,
			"product_id": productID,
		}).Error("Failed to add favorite")
	}
	return result.Error
}

// RemoveFavorite deletes a favorite relationship between a user and a product.
//
// Parameters:
//   - db: Database connection
//   - userID: ID of the user removing the favorite
//   - productID: ID of the product to unfavorite
//
// Returns:
//   - error: Any database error that occurred, nil if successful
func RemoveFavorite(db *gorm.DB, userID, productID uint) error {
	// Delete the favorite record
	result := db.Where("user_id = ? AND product_id = ?", userID, productID).Delete(&models.UserFavorite{})
	if result.Error != nil {
		logrus.WithError(result.Error).WithFields(logrus.Fields{
			"user_id":    userID,
			"product_id": productID,
		}).Error("Failed to remove favorite")
	}
	return result.Error
}

// GetUserFavorites retrieves all favorited products for a given user.
// This is a two-step process:
// 1. Get all favorite relationships for the user
// 2. Fetch the actual product details for those relationships
//
// Parameters:
//   - db: Database connection
//   - userID: ID of the user whose favorites to fetch
//
// Returns:
//   - []models.Product: Slice of favorited products
//   - error: Any database error that occurred
func GetUserFavorites(db *gorm.DB, userID uint) ([]models.Product, error) {
	// First, get all favorite relationships
	var favorites []models.UserFavorite
	result := db.Where("user_id = ?", userID).Find(&favorites)
	if result.Error != nil {
		logrus.WithError(result.Error).WithField("user_id", userID).Error("Failed to fetch favorites")
		return nil, result.Error
	}

	// Extract product IDs from favorites
	var productIDs []uint
	for _, fav := range favorites {
		productIDs = append(productIDs, fav.ProductID)
	}

	// Fetch actual product details
	var products []models.Product
	result = db.Where("id IN ?", productIDs).Find(&products)
	if result.Error != nil {
		logrus.WithError(result.Error).WithField("user_id", userID).Error("Failed to fetch products")
	}
	return products, result.Error
}

// IsProductFavorited checks if a specific product is favorited by a user.
//
// Parameters:
//   - db: Database connection
//   - userID: ID of the user to check
//   - productID: ID of the product to check
//
// Returns:
//   - bool: true if the product is favorited by the user, false otherwise
func IsProductFavorited(db *gorm.DB, userID, productID uint) bool {
	// Count matching favorite relationships
	var count int64
	db.Model(&models.UserFavorite{}).Where("user_id = ? AND product_id = ?", userID, productID).Count(&count)
	return count > 0
}