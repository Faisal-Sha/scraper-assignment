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

type NotificationServer struct {
	proto.UnimplementedNotificationServiceServer
	emailService *EmailService
	db *gorm.DB
}

func Start() {
	dbConn := db.Setup()
	emailService := NewEmailService(dbConn)

	// Start HTTP server
	e := echo.New()
	port := findAvailablePort(8082, "Notification HTTP")
	go func() {
		logrus.WithField("port", port).Info("Starting Notification HTTP server")
		if err := e.Start(fmt.Sprintf(":%d", port)); err != nil {
			logrus.WithError(err).Fatal("Notification HTTP server failed")
		}
	}()

	// Start gRPC server
	s, lis := startGRPCServer(emailService, dbConn)
	go func() {
		logrus.WithField("port", port+1).Info("Starting Notification gRPC server")
		log.Fatal(s.Serve(lis))
	}()
}

func startGRPCServer(emailService *EmailService, db *gorm.DB) (*grpc.Server, net.Listener) {
	port := findAvailablePort(8083, "Notification gRPC")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen")
	}

	s := grpc.NewServer()
	proto.RegisterNotificationServiceServer(s, &NotificationServer{emailService: emailService, db: db})
	return s, lis
}

func findAvailablePort(basePort int, serviceName string) int {
	port := basePort
	maxAttempts := 10

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
	logrus.WithFields(logrus.Fields{
		"service": serviceName,
		"port":    port,
	}).Warn("Failed to find available port, using default")
	return port
}