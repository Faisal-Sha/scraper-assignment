package crawler

import (
	"scraper/internal/models"
	"time"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func AddFavorite(db *gorm.DB, userID, productID uint) error {
	favorite := models.UserFavorite{
		UserID:    userID,
		ProductID: productID,
		AddedAt:   time.Now(),
	}
	result := db.Create(&favorite)
	if result.Error != nil {
		logrus.WithError(result.Error).WithFields(logrus.Fields{
			"user_id":    userID,
			"product_id": productID,
		}).Error("Failed to add favorite")
	}
	return result.Error
}

func RemoveFavorite(db *gorm.DB, userID, productID uint) error {
	result := db.Where("user_id = ? AND product_id = ?", userID, productID).Delete(&models.UserFavorite{})
	if result.Error != nil {
		logrus.WithError(result.Error).WithFields(logrus.Fields{
			"user_id":    userID,
			"product_id": productID,
		}).Error("Failed to remove favorite")
	}
	return result.Error
}

func GetUserFavorites(db *gorm.DB, userID uint) ([]models.Product, error) {
	var favorites []models.UserFavorite
	result := db.Where("user_id = ?", userID).Find(&favorites)
	if result.Error != nil {
		logrus.WithError(result.Error).WithField("user_id", userID).Error("Failed to fetch favorites")
		return nil, result.Error
	}

	var productIDs []uint
	for _, fav := range favorites {
		productIDs = append(productIDs, fav.ProductID)
	}

	var products []models.Product
	result = db.Where("id IN ?", productIDs).Find(&products)
	if result.Error != nil {
		logrus.WithError(result.Error).WithField("user_id", userID).Error("Failed to fetch products")
	}
	return products, result.Error
}

func IsProductFavorited(db *gorm.DB, userID, productID uint) bool {
	var count int64
	db.Model(&models.UserFavorite{}).Where("user_id = ? AND product_id = ?", userID, productID).Count(&count)
	return count > 0
}