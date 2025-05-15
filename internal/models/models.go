// Package models defines the data structures used throughout the scraper application
package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WebCategory represents a product category in the website's hierarchy
type WebCategory struct {
	Name  string `json:"name"`  // Display name of the category
	ID    int    `json:"id"`    // Unique identifier for the category
	Level int    `json:"level"` // Depth level in category hierarchy (0 for root)
}

// ProductPrice contains comprehensive pricing information for a product
type ProductPrice struct {
	SellingPrice        float64 `json:"sellingPrice"`        // Current selling price
	BuyingPrice         float64 `json:"buyingPrice"`         // Cost price for the merchant
	DiscountedPrice     float64 `json:"discountedPrice"`     // Price after applying discounts
	DiscountedPriceText string  `json:"discountedPriceText"` // Formatted discounted price
	OriginalPrice       float64 `json:"originalPrice"`       // Original price before discounts
	OriginalPriceText   string  `json:"originalPriceText"`   // Formatted original price
	SellingPriceText    string  `json:"sellingPriceText"`    // Formatted selling price
	BuyingPriceText     string  `json:"buyingPriceText"`     // Formatted buying price
	Currency            string  `json:"currency"`            // Currency code (e.g., TRY)
	DiscountPercentage  int     `json:"discountPercentage"`  // Discount percentage if applicable
	RRP                 float64 `json:"rrp"`                 // Recommended Retail Price
	ProfitMargin        float64 `json:"profitMargin"`        // Merchant's profit margin
}

// Item represents a detailed product listing from Trendyol's API
type Item struct {
	ID       int    `json:"id"`              // Unique product identifier
	Url      string `json:"productDetailUrl"` // Full URL to product page
	Name     string `json:"name"`            // Product name/title
	Code     string `json:"productCode"`      // SKU or product code

	// Category information including hierarchy
	Category struct {
		ID        int    `json:"id"`        // Category identifier
		Name      string `json:"name"`      // Category name
		Hierarchy string `json:"hierarchy"` // Full category path
	} `json:"category"`

	// Detailed seller/merchant information
	Seller struct {
		TaxOffice              string `json:"taxOffice"`              // Tax office location
		OfficialName           string `json:"officialName"`           // Legal business name
		RegisteredEmailAddress string `json:"registeredEmailAddress"` // Official contact email
		RegistrationNumber     string `json:"registrationNumber"`     // Business registration number
		TaxNumber              string `json:"taxNumber"`              // Tax identification number
		Address                string `json:"address"`                // Business address
		BusinessType           string `json:"businessType"`           // Type of business entity
		CodEligible            bool   `json:"codEligible"`            // Cash on delivery availability
	} `json:"sellerInfo"`

	// Complete category hierarchy from root to leaf
	WebCategoryTree []WebCategory `json:"webCategoryTree"`

	// Product rating and review statistics
	RatingScore struct {
		AverageRating float64 `json:"averageRating"` // Average user rating (0-5)
		CommentCount  int     `json:"commentCount"`  // Number of user reviews
		TotalCount    int     `json:"totalCount"`    // Total number of ratings
	} `json:"ratingScore"`

	Price    ProductPrice `json:"price"`    // Comprehensive pricing information
	Quantity int          `json:"quantity"` // Available stock quantity
}

// ProductItem represents a simplified product listing used in search results and listings
type ProductItem struct {
	ID          int    `json:"id"`    // Unique product identifier
	Name        string `json:"name"`  // Product name/title
	Brand       string `json:"brand"` // Brand name
	Image       string `json:"image"` // Primary product image URL

	// Basic rating information
	RatingScore struct {
		AverageRating float64 `json:"averageRating"` // Average user rating (0-5)
	} `json:"ratingScore"`

	// Basic price information
	Price struct {
		SellingPrice float64 `json:"sellingPrice"` // Current selling price
	} `json:"price"`
}

// Root represents the top-level response structure from Trendyol's API
type Root struct {
	Data struct {
		Contents []ProductItem `json:"contents"` // List of products in search/category results
	} `json:"data"`
}

// PriceStockLog tracks historical changes in product price and stock levels
type PriceStockLog struct {
	gorm.Model           // Includes ID, created_at, updated_at, deleted_at
	ProductID  uint      // Reference to the product
	OldPrice   string    // Previous price before change
	NewPrice   string    // New price after change
	OldStock   string    // Previous stock level
	NewStock   string    // New stock level
	ChangeTime time.Time // Exact time when change was detected
}

// User represents a registered user in the system
type User struct {
	gorm.Model           // Includes ID, created_at, updated_at, deleted_at
	Email       string    `gorm:"uniqueIndex;not null"` // Unique email address
	Username    string    // Display name
	Password    string    // Hashed password
	Name        string    // Full name
	IsActive    bool      `gorm:"default:true"` // Account status
	LastLoginAt time.Time // Most recent login timestamp
}

// Favorite represents a product favorited by a user (legacy model)
// Deprecated: Use UserFavorite instead
type Favorite struct {
	gorm.Model           // Includes ID, created_at, updated_at, deleted_at
	ProductID uint       // Reference to the favorited product
	UserID    uint       // Reference to the user
	AddedAt   time.Time  // When the product was favorited
}

// UserFavorite represents the many-to-many relationship between users and their favorite products
type UserFavorite struct {
	gorm.Model           // Includes ID, created_at, updated_at, deleted_at
	UserID    uint       `gorm:"index:idx_user_product,unique"` // Reference to the user
	ProductID uint       `gorm:"index:idx_user_product,unique"` // Reference to the product
	AddedAt   time.Time  // When the product was favorited
}

// Product represents a detailed product listing with various attributes
// Product represents a detailed product listing with various attributes stored in our database
type Product struct {
	gorm.Model           // Includes ID, created_at, updated_at, deleted_at
	ID                 uint           `gorm:"primaryKey"`       // Unique product identifier
	CategoryPath       string                                  // Full category hierarchy path
	Name               string                                  // Product name/title
	Images             datatypes.JSON `gorm:"type:jsonb"`     // Product images in different sizes
	Video              string                                  // Product video URL if available
	Seller             datatypes.JSON `gorm:"type:jsonb"`     // Seller/merchant information
	Brand              datatypes.JSON `gorm:"type:jsonb"`     // Brand details
	RatingScore        datatypes.JSON `gorm:"type:jsonb"`     // Product rating statistics
	FavoritesCount     string                                  // Number of users who favorited
	CommentsCount      string                                  // Number of user reviews
	AddToCartEvents    string                                  // Number of add to cart events
	Views              string                                  // Product page view count
	Orders             string                                  // Number of orders placed
	TopReviews         datatypes.JSON `gorm:"type:jsonb"`     // Most helpful user reviews
	SizeRecommendation string                                  // Size fit recommendations
	EstimatedDelivery  datatypes.JSON `gorm:"type:jsonb"`     // Delivery time estimates
	StockInfo          datatypes.JSON `gorm:"type:jsonb"`     // Current stock levels
	PriceInfo          datatypes.JSON `gorm:"type:jsonb"`     // Current pricing details
	SimilarProducts    datatypes.JSON `gorm:"type:jsonb"`     // Related product suggestions
	Attributes         datatypes.JSON `gorm:"type:jsonb"`     // Product specifications
	OtherSellers       datatypes.JSON `gorm:"type:jsonb"`     // Other merchants selling same product
	IsActive           bool           `gorm:"default:true"`   // Product availability status
	IsFavorite         bool           `gorm:"default:false"` // Whether product is favorited
	Price              float64        `gorm:"type:decimal(10,2)"` // Current price
}

// TrendyolResponse represents the raw API response from Trendyol's product detail endpoint
type TrendyolResponse struct {
	ID          int    `json:"id"`          // Unique product identifier
	Name        string `json:"name"`        // Product name/title
	ProductCode string `json:"productCode"` // SKU or product code
	InStock     bool   `json:"inStock"`     // Overall stock status

	// All available product variants (sizes, colors, etc.)
	AllVariants []struct {
		Barcode    string  `json:"barcode"`    // Variant barcode
		Currency   string  `json:"currency"`   // Price currency
		InStock    bool    `json:"inStock"`    // Variant availability
		ItemNumber int     `json:"itemNumber"` // Variant identifier
		Price      float64 `json:"price"`      // Variant price
		Value      string  `json:"value"`      // Variant description
	} `json:"allVariants"`

	IsFavorited bool `json:"isPeopleLikeThisProduct"` // Product popularity indicator

	// Brand information
	Brand struct {
		ID   int    `json:"id"`   // Brand identifier
		Name string `json:"name"` // Brand name
	} `json:"brand"`

	// Category information
	Category struct {
		Hierarchy string `json:"hierarchy"` // Full category path
		ID        int    `json:"id"`        // Category identifier
		Name      string `json:"name"`      // Category name
	} `json:"category"`

	// Product rating statistics
	RatingScore struct {
		AverageRating float32 `json:"averageRating"` // Average user rating (0-5)
		TotalCount    int     `json:"totalCount"`    // Total number of ratings
		CommentCount  int     `json:"commentCount"`  // Number of reviews
	} `json:"ratingScore"`

	// Best available variant (best price/availability)
	WinnerVariant struct {
		Price struct {
			Currency        string  `json:"currency"`        // Price currency
			DiscountedPrice float64 `json:"discountedPrice"` // Price after discount
			SellingPrice    float64 `json:"sellingPrice"`    // Original price
		} `json:"price"`
		Stock struct {
			Quantity int  `json:"quantity"` // Available quantity
			Disabled bool `json:"disabled"` // Stock status
		} `json:"stock"`
	} `json:"winnerVariant"`

	// Best merchant information
	WinnerMerchantListing struct {
		Merchant struct {
			ID   int    `json:"id"`   // Merchant identifier
			Name string `json:"name"` // Merchant name
		} `json:"merchant"`
	} `json:"winnerMerchantListing"`

	// Product images in different sizes
	Images []struct {
		Org       string `json:"org"`       // Original size image
		Preview   string `json:"preview"`   // Thumbnail image
		MainImage string `json:"mainImage"` // Standard size image
		Zoom      string `json:"zoom"`      // High resolution image
	} `json:"images"`

	// Detailed seller information
	SellerInfo struct {
		Address                string `json:"address"`                // Business address
		BusinessType           string `json:"businessType"`           // Type of business
		CodEligible            bool   `json:"codEligible"`            // Cash on delivery
		OfficialName           string `json:"officialName"`           // Legal name
		RegisteredEmailAddress string `json:"registeredEmailAddress"` // Contact email
		RegistrationNumber     string `json:"registrationNumber"`     // Business ID
		TaxNumber              string `json:"taxNumber"`              // Tax ID
		TaxOffice              string `json:"taxOffice"`              // Tax office
	} `json:"sellerInfo"`

	// Delivery information
	Delivery struct {
		DeliveryEndDate   string `json:"deliveryEndDate"`   // Latest delivery date
		DeliveryStartDate string `json:"deliveryStartDate"` // Earliest delivery date
	} `json:"winnerMerchantListing"` // Note: This tag appears to be incorrect, should be "delivery"

	// Product specifications
	Attributes []struct {
		Key   string `json:"key"`   // Attribute name
		Value string `json:"value"` // Attribute value
		Type  string `json:"type"`  // Attribute type
	} `json:"attributes"`

	// Product description sections
	Description struct {
		ContentDescriptions []struct {
			Description string `json:"description"` // Content text
			Type        string `json:"type"`        // Content type
		} `json:"contentDescriptions"`
	} `json:"description"`

	// Social proof metrics
	SocialProof []struct {
		Key   string `json:"key"`   // Metric name
		Value string `json:"value"` // Metric value
	} `json:"socialProof"`

	OrderCount int `json:"orderCount"` // Total number of orders

	// Other merchants' variants
	OtherSellersVariants []struct {
		Barcode    string  `json:"barcode"`    // Variant barcode
		Currency   string  `json:"currency"`   // Price currency
		InStock    bool    `json:"inStock"`    // Availability
		ItemNumber int     `json:"itemNumber"` // Variant ID
		Price      float64 `json:"price"`      // Variant price
		Value      string  `json:"value"`      // Variant description
	} `json:"otherMerchantVariants"`
}