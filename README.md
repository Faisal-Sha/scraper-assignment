# Scraper Microservices

## Testing Price Drop Notifications

To test the price drop notification feature, follow these steps:

1. **Prerequisites**:
   - Ensure all services are running (`docker compose up`)
   - Configure email settings in `.env` file
   - Have a user account with favorite products

2. **Add a Product to Favorites**:
   ```sql
   -- Run this in the PostgreSQL container
   docker exec -it scraper-postgres-1 psql -U postgres -d scraper
   -- Replace user_id and product_id with actual values
   INSERT INTO user_favorites (user_id, product_id) VALUES (your_user_id, product_id);
   ```

3. **Simulate Price Drop**:
   ```bash
   # Replace product_id and new_price with your values
   curl -X POST http://localhost:8080/simulate-price-drop \
     -H 'Content-Type: application/json' \
     -d '{
       "product_id": your_product_id,
       "new_price": new_price_value
     }'
   ```

4. **Verify Notification**:
   - Check your email inbox (and spam folder) for the price drop notification
   - The email subject will be: "Price Drop Alert! [Product Name] is now cheaper"
   - The email will contain the old and new prices

5. **Monitor Logs**:
   ```bash
   # View application logs
   docker compose logs -f
   ```

Example Flow:
```bash
# 1. Add product to user's favorites
docker exec -it scraper-postgres-1 psql -U postgres -d scraper -c \
  "INSERT INTO user_favorites (user_id, product_id) VALUES (3, 169048650);"

# 2. Simulate a price drop from 50.00 to 45.00
curl -X POST http://localhost:8080/simulate-price-drop \
  -H 'Content-Type: application/json' \
  -d '{"product_id": 169048650, "new_price": 45.00}'

# 3. Check logs for notification status
docker compose logs -f | grep "notification"
```

## Overview

A scalable microservices-based backend for fetching, analyzing, and notifying users about product updates from Trendyol. The system uses a microservices architecture with Kafka for message queuing and PostgreSQL for data storage.

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

## Prerequisites

- Go 1.19 or later
- Docker and Docker Compose
- PostgreSQL 13 or later
- Apache Kafka

## Configuration

1. Environment Variables (.env):
```bash
# Email Configuration
EMAIL_APP_PASSWORD=your_app_password
EMAIL_SENDER=your_email@example.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587

# Database Configuration
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=scraper
DB_PORT=5432

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_PRODUCTS_TOPIC=PRODUCTS
KAFKA_FAVORITES_TOPIC=FAVORITE_PRODUCTS

# Server Configuration
CRAWLER_PORT=8080
NOTIFICATION_PORT=8081
CRAWLER_GRPC_PORT=8082
NOTIFICATION_GRPC_PORT=8083
```

2. Kafka Topics:
   - PRODUCTS: Main topic for product updates
   - FAVORITE_PRODUCTS: Topic for favorite product updates

## Setup

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd scraper
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Set up environment variables:
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. Start required services:
   ```bash
   docker compose up -d
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
## Running the Application

1. Start all services:
   ```bash
   go run cmd/scraper/main.go
   ```

2. The following services will start:
   - Crawler Service (HTTP: 8080, gRPC: 8081)
   - Product Analysis Service (HTTP: 8082)
   - Favorites Service (HTTP: 8083)
   - Notification Service (HTTP: 8084, gRPC: 8085)

## Testing Guide

### 1. Unit Tests
```bash
# Run all unit tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 2. Component Tests
```bash
# Test Kafka Producer
go test ./internal/kafka/producer_test.go -v

# Test Database Operations
go test ./internal/db/db_test.go -v

# Test Email Notifications
go test ./internal/notification/email_test.go -v
```

2. Integration Testing:
   ```bash
   # Run integration tests
   cd tests && go test -v
   ```

### 3. Manual Testing Flow

#### a. Environment Setup
```bash
# Start infrastructure services
docker compose up -d

# Verify services are running
docker compose ps

# Check Kafka topics
docker compose exec kafka kafka-topics --list --bootstrap-server localhost:9092
```

#### b. Database Verification
```bash
# Connect to PostgreSQL
docker compose exec postgres psql -U postgres -d scraper

# Check tables
\dt

# Check product count
SELECT COUNT(*) FROM products;
```

#### c. Test Product Flow

1. Fetch Products:
```bash
# Fetch new products
curl -X GET http://localhost:8080/fetch

# Check Kafka messages
docker compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic PRODUCTS --from-beginning
```

2. User Operations:
```bash
# Create a new user
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","username":"testuser","password":"password123","name":"Test User"}'

# Add product to favorites
curl -X POST http://localhost:8080/favorites \
  -H 'Content-Type: application/json' \
  -d '{"user_id":1,"product_id":123}'

# Get user's favorites
curl -X GET http://localhost:8080/favorites/1
```

   c. Test Product Analysis Service:
   ```bash
   # Start product analysis service
   go run cmd/product-analysis/main.go
   
   # Check Kafka topic for product updates
   kafka-console-consumer --bootstrap-server localhost:9092 --topic product-updates
   ```

   d. Test Favorites Service:
   ```bash
   # Start favorites service
   go run cmd/favorites/main.go
   
   # Add a product to favorites
   curl -X POST http://localhost:8082/favorites -d '{"user_id": 1, "product_id": "123"}'
   ```

   e. Test Notification Service:
   ```bash
   # Start notification service
   go run cmd/notification/main.go
   
   # Monitor notification logs
   tail -f logs/notification.log
   ```

4. End-to-End Test Flow:

   a. Create a new user
   ```bash
   curl -X POST http://localhost:8080/users -d '{"email": "test@example.com"}'
   ```

   b. Add products to favorites
   ```bash
   curl -X POST http://localhost:8080/favorites -d '{"user_id": 1, "product_id": "123"}'
   ```

   c. Verify product updates
   ```bash
   # Check favorite products
   curl http://localhost:8080/favorites/1
   
   # Monitor price updates
   kafka-console-consumer --bootstrap-server localhost:9092 --topic price-updates
   ```

### 4. System Verification

#### a. Database Verification
```bash
# Check products table
docker compose exec postgres psql -U postgres -d scraper -c "SELECT COUNT(*) FROM products;"

# Check users table
docker compose exec postgres psql -U postgres -d scraper -c "SELECT * FROM users;"

# Check favorites table
docker compose exec postgres psql -U postgres -d scraper -c "SELECT * FROM favorites;"
```

#### b. Kafka Topic Verification
```bash
# Check PRODUCTS topic
docker compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic PRODUCTS --from-beginning

# Check FAVORITE_PRODUCTS topic
docker compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic FAVORITE_PRODUCTS --from-beginning
```

#### c. Log Monitoring
```bash
# View service logs
docker compose logs -f

# View specific service logs
docker compose logs -f crawler
```

### 5. Troubleshooting

1. Kafka Issues:
   ```bash
   # Reset Kafka
   docker compose restart kafka
   
   # Clear Kafka data
   docker compose down
   rm -rf /tmp/kafka-logs/
   docker compose up -d
   ```

2. Database Issues:
   ```bash
   # Reset PostgreSQL
   docker compose restart postgres
   
   # Check PostgreSQL logs
   docker compose logs postgres
   ```

3. Port Conflicts:
   ```bash
   # Kill processes using ports
   sudo kill -9 $(sudo lsof -t -i:8080-8089)
   ```

### 6. Performance Considerations

1. Kafka Configuration:
   - Message size limit: 5MB
   - Batch size: 50 messages
   - Batch interval: 500ms

2. Database Optimization:
   - Indexes on frequently queried fields
   - Regular VACUUM operations
   - Connection pooling

3. Rate Limiting:
   - 4-second delay between product fetches
   - Batch processing for large datasets
   - Scheduled updates for favorite products

6. Performance Testing:
   ```bash
   # Run benchmarks
   go test -bench=. ./...
   
   # Test crawler with multiple concurrent requests
   ab -n 1000 -c 10 http://localhost:8080/fetch
   ```