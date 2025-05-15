# Scraper Microservices

A scalable microservices-based backend for fetching, analyzing, and notifying users about product updates from Trendyol.

## Architecture

The system consists of four microservices:
- **Crawler Service**: Fetches product data from Trendyol and sends it to Kafka.
- **Product Analysis Service**: Analyzes product data, updates the database, and forwards favorited products.
- **Favorites Service**: Prioritizes updates for favorited products and logs price/stock changes.
- **Notification Service**: Sends email notifications for price drops.

## Directory Structure
scraper/
├── cmd/
│   └── scraper/
│       └── main.go               # Application entry point
├── internal/
│   ├── crawler/                 # Crawler service logic
│   │   ├── server.go            # gRPC and HTTP server setup
│   │   ├── handlers.go          # HTTP handlers for fetching and user management
│   │   ├── fetch.go             # Product fetching and conversion logic
│   │   ├── favorites.go         # Favorite-related database operations
│   │   ├── fetch_test.go        # Unit tests for fetch.go
│   │   └── favorites_test.go    # Unit tests for favorites.go
│   ├── analysis/                # Product analysis service logic
│   │   ├── server.go            # HTTP server and health check
│   │   ├── consumer.go          # Kafka consumer for product analysis
│   │   └── consumer_test.go     # Unit tests for consumer.go
│   ├── favorites/               # Favorite product service logic
│   │   ├── server.go            # HTTP server and health check
│   │   ├── scheduler.go         # Cron scheduler for periodic updates
│   │   ├── consumer.go          # Kafka consumer for favorite products
│   │   └── scheduler_test.go    # Unit tests for scheduler.go
│   ├── notification/            # Notification service logic
│   │   ├── server.go            # gRPC server for notifications
│   │   ├── email.go             # Email sending logic
│   │   └── email_test.go        # Unit tests for email.go
│   ├── db/                      # Database setup and utilities
│   │   └── db.go                # Database connection and migrations
│   ├── kafka/                   # Kafka producer/consumer setup
│   │   ├── producer.go          # Kafka producer logic
│   │   ├── consumer.go          # Kafka consumer logic
│   │   └── producer_test.go     # Unit tests for producer.go
│   ├── models/                  # Database models
│   │   └── models.go            # Struct definitions (Product, User, etc.)
│   └── proto/                   # gRPC proto files
│       ├── crawler.proto        # Crawler service proto definition
│       ├── crawler.pb.go        # Generated gRPC code for crawler
│       ├── notification.proto    # Notification service proto definition
│       └── notification.pb.go    # Generated gRPC code for notification
├── pkg/
│   ├── config/                  # Configuration loading
│   │   └── config.go            # Environment variable loading
│   └── logger/                  # Centralized logging
│       └── logger.go            # Structured logging setup
├── go.mod                       # Go module file
├── go.sum                       # Go module checksums
├── .env                         # Environment variables
├── Dockerfile                   # Docker configuration
├── docker-compose.yml           # Docker Compose for services
├── README.md                    # Project documentation
└── tests/                       # Integration tests
    └── integration_test.go      # Integration tests for microservices


## API Endpoints
GET /fetch: Fetches product data and sends to Kafka.
POST /favorites: Adds a product to a user's favorites.
DELETE /favorites: Removes a product from a user's favorites.
GET /favorites/:user_id: Lists a user's favorite products.
POST /users: Creates a new user.
GET /users/:id: Retrieves user details.
GET /health: Health check for analysis and favorites services.

## Setup

1. Install dependencies:
   ```bash
   go mod tidy
   ```

2. Set up environment variables:
   ```bash
   cp .env.example .env
   ```

3. Run the crawler service:
   ```bash
   go run cmd/crawler/main.go
   ```

4. Run the product analysis service:
   ```bash
   go run cmd/product-analysis/main.go
   ```

5. Run the favorites service:
   ```bash
   go run cmd/favorites/main.go
   ```

6. Run the notification service:
   ```bash
   go run cmd/notification/main.go
   ```
7. Run the tests:
   ```bash
   go test ./...