// Package notification implements the notification service for sending price drop alerts to users
package notification

import (
	"fmt"
	"log"
	"net"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"scraper/internal/db"
	"scraper/internal/proto"

	"github.com/sirupsen/logrus"
)

// NotificationServer implements the gRPC notification service.
// It handles sending notifications to users about price changes in their
// favorited products.
type NotificationServer struct {
	proto.UnimplementedNotificationServiceServer
	emailService *EmailService // Service for sending email notifications
	db          *gorm.DB      // Database connection
}

// Start initializes and runs the notification service.
// This service provides both HTTP and gRPC endpoints:
// - HTTP server: For health checks and future REST endpoints
// - gRPC server: For receiving notification requests from other services
//
// The service performs the following setup:
// 1. Initializes database connection
// 2. Creates email notification service
// 3. Starts HTTP server on first available port starting from 8082
// 4. Starts gRPC server on first available port starting from 8083
//
// Both servers are started in separate goroutines to run concurrently.
func Start() {
	// Initialize dependencies
	dbConn := db.Setup()
	emailService := NewEmailService(dbConn)

	// Start HTTP server for health checks
	e := echo.New()
	port := findAvailablePort(8082, "Notification HTTP")
	go func() {
		logrus.WithField("port", port).Info("Starting Notification HTTP server")
		if err := e.Start(fmt.Sprintf(":%d", port)); err != nil {
			logrus.WithError(err).Fatal("Notification HTTP server failed")
		}
	}()

	// Start gRPC server for notification requests
	s, lis := startGRPCServer(emailService, dbConn)
	go func() {
		logrus.WithField("port", port+1).Info("Starting Notification gRPC server")
		log.Fatal(s.Serve(lis))
	}()
}

// startGRPCServer initializes and configures the gRPC server.
// It performs the following steps:
// 1. Finds an available port starting from 8083
// 2. Creates a TCP listener on the port
// 3. Creates a new gRPC server
// 4. Registers the notification service
//
// Parameters:
//   - emailService: Service for sending email notifications
//   - db: Database connection for user lookups
//
// Returns:
//   - *grpc.Server: Configured gRPC server
//   - net.Listener: TCP listener for the server
func startGRPCServer(emailService *EmailService, db *gorm.DB) (*grpc.Server, net.Listener) {
	// Find available port
	port := findAvailablePort(8083, "Notification gRPC")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen")
	}

	// Create and configure gRPC server
	s := grpc.NewServer()
	proto.RegisterNotificationServiceServer(s, &NotificationServer{emailService: emailService, db: db})
	return s, lis
}

// findAvailablePort attempts to find an available TCP port starting from basePort.
// It will try up to maxAttempts times before falling back to the last attempted port.
//
// The function works by:
// 1. Attempting to create a TCP listener on the current port
// 2. If successful, closing the listener and returning that port
// 3. If unsuccessful, incrementing the port number and trying again
//
// Parameters:
//   - basePort: Starting port number to try
//   - serviceName: Name of the service for logging purposes
//
// Returns:
//   - int: Available port number, or last attempted port if none found
func findAvailablePort(basePort int, serviceName string) int {
	port := basePort
	maxAttempts := 10

	// Try ports until we find an available one
	for attempt := 0; attempt < maxAttempts; attempt++ {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			logrus.WithFields(logrus.Fields{
				"service": serviceName,
				"port":    port,
			}).Info("Found available port")
			return port
		}
		logrus.WithFields(logrus.Fields{
			"service": serviceName,
			"port":    port,
		}).Warn("Port in use, trying next port")
		port++
	}

	// Fall back to last attempted port
	logrus.WithFields(logrus.Fields{
		"service": serviceName,
		"port":    port,
	}).Warn("Failed to find available port, using default")
	return port
}