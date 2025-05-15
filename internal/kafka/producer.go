// Package kafka provides functionality for interacting with Apache Kafka message broker.
// It includes producers and consumers for handling product updates and notifications.
package kafka

import (
	"os"
	"strings"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

// SetupProducer initializes and configures a synchronous Kafka producer.
// It reads broker addresses from environment variables and sets up the producer
// with appropriate configuration for our use case.
//
// Environment Variables:
//   - KAFKA_BROKERS: Comma-separated list of Kafka broker addresses (default: localhost:9092)
//
// The producer is configured with:
//   - Synchronous operation (waits for acknowledgment)
//   - 5MB maximum message size (for large product batches)
//   - Automatic broker discovery
//
// Returns:
//   - sarama.SyncProducer: A configured Kafka producer
//   - Panics if producer creation fails
func SetupProducer() sarama.SyncProducer {
	// Get Kafka broker addresses from environment
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		// Default to localhost if no brokers specified
		brokers = []string{"localhost:9092"}
	}

	// Configure producer settings
	config := sarama.NewConfig()
	// Enable synchronous operation
	config.Producer.Return.Successes = true
	// Increase max message size to 5MB to handle large product batches
	config.Producer.MaxMessageBytes = 5 * 1024 * 1024

	// Create synchronous producer
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create Kafka producer")
	}

	logrus.WithField("brokers", brokers).Info("Kafka producer initialized")
	return producer
}