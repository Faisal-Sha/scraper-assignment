package crawler

import (
	"fmt"
	"log"
	"net"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"

	"scraper/internal/db"
	"scraper/internal/kafka"
	"scraper/internal/proto"

	"github.com/sirupsen/logrus"
)

type CrawlerServer struct {
	proto.UnimplementedCrawlerServiceServer
}

func Start() {
	// Initialize dependencies
	dbConn := db.Setup()
	producer := kafka.SetupProducer()

	// Start HTTP server
	e := echo.New()
	registerHandlers(e, dbConn, producer)

	port := findAvailablePort(8080, "Crawler HTTP")
	go func() {
		logrus.WithField("port", port).Info("Starting Crawler HTTP server")
		if err := e.Start(fmt.Sprintf(":%d", port)); err != nil {
			logrus.Fatalf("Crawler HTTP server failed: %v", err)
		}
	}()

	// Start gRPC server
	s, lis := startGRPCServer()
	go func() {
		logrus.WithField("port", port+1).Info("Starting Crawler gRPC server")
		log.Fatal(s.Serve(lis))
	}()
}

func startGRPCServer() (*grpc.Server, net.Listener) {
	port := findAvailablePort(8081, "Crawler gRPC")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logrus.Fatalf("Failed to listen on port %d: %v", port, err)
	}

	s := grpc.NewServer()
	proto.RegisterCrawlerServiceServer(s, &CrawlerServer{})
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