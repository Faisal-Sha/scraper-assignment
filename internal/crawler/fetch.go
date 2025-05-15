// Package crawler implements the product crawler service which fetches product data
// from Trendyol's API and publishes it to Kafka for further processing
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

// FetchProductDetails retrieves detailed product information from Trendyol's API.
// It makes an HTTP GET request to Trendyol's product detail endpoint and returns
// the raw JSON response as a map.
//
// Parameters:
//   - productID: The unique identifier of the product to fetch
//
// Returns:
//   - map[string]interface{}: The raw JSON response from Trendyol's API
//   - nil if any error occurs during the request
//
// The function handles various error cases:
//   - Request creation errors
//   - Network/HTTP errors
//   - Response reading errors
//   - JSON parsing errors
func FetchProductDetails(productID int) map[string]interface{} {
	// Construct the API URL with the product ID
	url := fmt.Sprintf("https://apigw.trendyol.com/discovery-sfint-product-service/api/product-detail/?contentId=%d&campaignId=null&storefrontId=36&culture=en-AE", productID)

	// Create HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error creating request")
		return nil
	}

	// Set required headers for the API request
	req.Header.Set("accept", "application/json")
	// TODO: Add all required headers from original fetchProductDetails
	// req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	// req.Header.Set("Referer", "https://www.trendyol.com/")
	// req.Header.Set("Origin", "https://www.trendyol.com")
	// req.Header.Set("Connection", "keep-alive")
	// req.Header.Set("Cache-Control", "max-age=0")
	// req.Header.Set("Upgrade-Insecure-Requests", "1")
	// req.Header.Set("DNT", "1")
	// req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error fetching product")
		return nil
	}
	defer resp.Body.Close()

	// Read and parse the response
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error reading response")
		return nil
	}

	// Parse JSON response
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(bodyText, &jsonResponse); err != nil {
		logrus.WithError(err).WithField("product_id", productID).Error("Error unmarshaling response")
		return nil
	}

	return jsonResponse
}

// readMockData reads and parses mock product data from a JSON file.
// This is used for testing and development purposes.
//
// Returns:
//   - []models.Product: Slice of parsed product models
//   - error: Any error that occurred during file reading or parsing
func readMockData() ([]models.Product, error) {
	// Read mock data file
	data, err := ioutil.ReadFile("data.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read mock data: %v", err)
	}

	// Parse JSON into TrendyolResponse structs
	var trendyolResp []models.TrendyolResponse
	err = json.Unmarshal(data, &trendyolResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal mock data: %v", err)
	}

	// Convert to Product models
	return ConvertTrendyolToProduct(&trendyolResp), nil
}

// ConvertTrendyolToProduct converts Trendyol API responses to our internal Product models.
// It handles the mapping of all fields and nested structures, converting them to the
// appropriate format for our database schema.
//
// Parameters:
//   - item: Pointer to slice of TrendyolResponse objects from the API
//
// Returns:
//   - []models.Product: Slice of converted Product models ready for database insertion
func ConvertTrendyolToProduct(item *[]models.TrendyolResponse) []models.Product {
	// Initialize slice with exact capacity needed
	products := make([]models.Product, len(*item))

		// Process each product from the API response
	for i, content := range *item {
		// Convert rating information to JSON
		ratingJSON, _ := json.Marshal(map[string]interface{}{
			"averageRating": content.RatingScore.AverageRating,
			"commentCount":  content.RatingScore.CommentCount,
			"totalCount":    content.RatingScore.TotalCount,
		})
		comments := content.RatingScore.CommentCount

		// Convert stock information to JSON
		stockJSON, _ := json.Marshal(map[string]interface{}{
			"stock":    content.WinnerVariant.Stock.Quantity,
			"disabled": content.WinnerVariant.Stock.Disabled,
		})

		// Convert pricing information to JSON
		priceJSON, _ := json.Marshal(map[string]interface{}{
			"price":         content.WinnerVariant.Price.DiscountedPrice,
			"originalPrice": content.WinnerVariant.Price.SellingPrice,
			"currency":      content.WinnerVariant.Price.Currency,
		})

		// Convert seller information to JSON
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

		// Convert delivery information to JSON
		deliveryJSON, _ := json.Marshal(map[string]interface{}{
			"deliveryEndDate":   content.Delivery.DeliveryEndDate,
			"deliveryStartDate": content.Delivery.DeliveryStartDate,
		})

		// Process other sellers' variants
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

		// Convert brand information to JSON
		brandJSON, _ := json.Marshal(content.Brand)

		// Process product attributes
		attrMap := make(map[string]interface{})
		for _, attr := range content.Attributes {
			attrMap[attr.Key] = attr.Value
		}
		attributesJSON, _ := json.Marshal(attrMap)

		// Extract main product images
		var images []string
		for _, img := range content.Images {
			images = append(images, img.MainImage)
		}
		imagesJSON, _ := json.Marshal(images)

		// Process social proof metrics
		var orders, favorites, views, addToBasket string
		for _, count := range content.SocialProof {
			switch count.Key {
			case "orderCount":
				orders = count.Value
			case "favoriteCount":
				favorites = count.Value
			case "pageViewCount":
				views = count.Value
			case "basketCount":
				addToBasket = count.Value
			}
		}

		// Create the product model with all converted data
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

	// Log success and return converted products
	logrus.WithField("count", len(products)).Info("Converted Trendyol response to products")
	return products
}