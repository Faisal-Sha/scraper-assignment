// Package kafka provides functionality for interacting with Apache Kafka message broker.
// It includes producers and consumers for handling product updates and notifications.
package kafka

import (
	"os"
	"strings"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
)

// SetupConsumer initializes and configures a Kafka consumer for a given topic.
// It sets up a consumer group that processes messages from the specified topic
// using the provided handler function. The consumer runs in a separate goroutine
// and continuously processes messages until the application terminates.
//
// Parameters:
//   - topic: The Kafka topic to consume messages from
//   - handler: A function that processes each message received from the topic
//              The function takes a byte slice containing the message value
//
// Environment Variables:
//   - KAFKA_BROKERS: Comma-separated list of Kafka broker addresses (default: localhost:9092)
//
// The consumer is configured with:
//   - Error reporting enabled
//   - Latest offset strategy (only processes new messages)
//   - Single partition consumption (partition 0)
//   - Automatic broker discovery
//
// The function runs indefinitely in a goroutine and will:
//   1. Connect to the Kafka brokers
//   2. Create a partition consumer
//   3. Process messages as they arrive using the handler function
//   4. Log any errors that occur during consumption
//
// Note: This is a simplified consumer implementation. In production, you might want to:
//   - Implement proper shutdown handling
//   - Use consumer groups for scalability
//   - Add retry logic for failed messages
//   - Implement offset management
func SetupConsumer(topic string, handler func([]byte)) {
	// Get Kafka broker addresses from environment
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		// Default to localhost if no brokers specified
		brokers = []string{"localhost:9092"}
	}

	// Configure consumer settings
	config := sarama.NewConfig()
	// Enable error reporting
	config.Consumer.Return.Errors = true

	// Create the consumer
	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating consumer")
	}

	// Create a partition consumer
	// Note: In production, you'd typically use a consumer group instead
	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetNewest)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating partition consumer")
	}

	logrus.WithField("topic", topic).Info("Started consuming from topic")

	// Start consuming messages in a separate goroutine
	go func() {
		for {
			select {
			// Handle incoming messages
			case msg := <-partitionConsumer.Messages():
				logrus.WithField("topic", topic).Info("Received message")
				handler(msg.Value)

			// Handle errors
			case err := <-partitionConsumer.Errors():
				logrus.WithError(err).WithField("topic", topic).Error("Error consuming")
			}
		}
	}()
}