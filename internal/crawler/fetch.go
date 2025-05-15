package crawler

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"gorm.io/datatypes"

	"scraper/internal/models"

	"github.com/sirupsen/logrus"
)

// FetchProductDetails retrieves detailed product information from the Trendyol API.
func FetchProductDetails(productID int) map[string]interface{} {
	url := fmt.Sprintf("https://apigw.trendyol.com/discovery-sfint-product-service/api/product-detail/?contentId=%d&campaignId=null&storefrontId=36&culture=en-AE", productID)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error creating request")
		return nil
	}
	// Set headers (same as in original main.go)
	req.Header.Set("accept", "application/json")
	// ... (add all headers from original fetchProductDetails)
	resp, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error fetching product")
		return nil
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error reading response")
		return nil
	}
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(bodyText, &jsonResponse); err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error unmarshaling response")
		return nil
	}
	return jsonResponse
}

func readMockData() ([]models.Product, error) {
	data, err := ioutil.ReadFile("data.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read mock data: %v", err)
	}
	var trendyolResp []models.TrendyolResponse
	err = json.Unmarshal(data, &trendyolResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal mock data: %v", err)
	}
	return ConvertTrendyolToProduct(&trendyolResp), nil
}

// ConvertTrendyolToProduct converts Trendyol API response to Product model.
func ConvertTrendyolToProduct(item *[]models.TrendyolResponse) []models.Product {
	products := make([]models.Product, len(*item))

	for i, content := range *item {
		ratingJSON, _ := json.Marshal(map[string]interface{}{
			"averageRating": content.RatingScore.AverageRating,
			"commentCount":  content.RatingScore.CommentCount,
			"totalCount":    content.RatingScore.TotalCount,
		})
		comments := content.RatingScore.CommentCount
		stockJSON, _ := json.Marshal(map[string]interface{}{
			"stock":    content.WinnerVariant.Stock.Quantity,
			"disabled": content.WinnerVariant.Stock.Disabled,
		})
		priceJSON, _ := json.Marshal(map[string]interface{}{
			"price":         content.WinnerVariant.Price.DiscountedPrice,
			"originalPrice": content.WinnerVariant.Price.SellingPrice,
			"currency":      content.WinnerVariant.Price.Currency,
		})
		sellerJSON, _ := json.Marshal(map[string]interface{}{
			"address":                content.SellerInfo.Address,
			"businessType":           content.SellerInfo.BusinessType,
			"codEligible":            content.SellerInfo.CodEligible,
			"officialName":           content.SellerInfo.OfficialName,
			"registeredEmailAddress": content.SellerInfo.RegisteredEmailAddress,
			"registrationNumber":     content.SellerInfo.RegistrationNumber,
			"taxNumber":              content.SellerInfo.TaxNumber,
			"taxOffice":              content.SellerInfo.TaxOffice,
		})
		deliveryJSON, _ := json.Marshal(map[string]interface{}{
			"deliveryEndDate":   content.Delivery.DeliveryEndDate,
			"deliveryStartDate": content.Delivery.DeliveryStartDate,
		})
		otherSellersVariant := make(map[string]interface{})
		for _, attr := range content.OtherSellersVariants {
			otherSellersVariant["barcode"] = attr.Barcode
			otherSellersVariant["currency"] = attr.Currency
			otherSellersVariant["inStock"] = attr.InStock
			otherSellersVariant["itemNumber"] = attr.ItemNumber
			otherSellersVariant["price"] = attr.Price
			otherSellersVariant["value"] = attr.Value
		}
		otherSellersVariantsJSON, _ := json.Marshal(otherSellersVariant)
		brandJSON, _ := json.Marshal(content.Brand)
		attrMap := make(map[string]interface{})
		for _, attr := range content.Attributes {
			attrMap[attr.Key] = attr.Value
		}
		attributesJSON, _ := json.Marshal(attrMap)
		var images []string
		for _, img := range content.Images {
			images = append(images, img.MainImage)
		}
		imagesJSON, _ := json.Marshal(images)
		var orders, favorites, views, addToBasket string
		for _, count := range content.SocialProof {
			if count.Key == "orderCount" {
				orders = count.Value
			} else if count.Key == "favoriteCount" {
				favorites = count.Value
			} else if count.Key == "pageViewCount" {
				views = count.Value
			} else if count.Key == "basketCount" {
				addToBasket = count.Value
			}
		}
		products[i] = models.Product{
			ID:                uint(content.ID),
			Name:              content.Name,
			CategoryPath:      content.Category.Hierarchy,
			Brand:             datatypes.JSON(brandJSON),
			Seller:            datatypes.JSON(sellerJSON),
			RatingScore:       datatypes.JSON(ratingJSON),
			IsActive:          content.InStock,
			StockInfo:         datatypes.JSON(stockJSON),
			PriceInfo:         datatypes.JSON(priceJSON),
			Attributes:        datatypes.JSON(attributesJSON),
			Images:            datatypes.JSON(imagesJSON),
			Orders:            orders,
			FavoritesCount:    favorites,
			Views:             views,
			IsFavorite:        content.IsFavorited,
			CommentsCount:     strconv.Itoa(comments),
			AddToCartEvents:   addToBasket,
			EstimatedDelivery: datatypes.JSON(deliveryJSON),
			OtherSellers:      datatypes.JSON(otherSellersVariantsJSON),
		}
	}
	logrus.WithField("count", len(products)).Info("Converted Trendyol response to products")
	return products
}