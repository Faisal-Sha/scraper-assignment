package kafka

import (
	"os"
	"strings"

	"github.com/IBM/sarama"

	"github.com/sirupsen/logrus"
)

// SetupProducer initializes a Kafka producer.
func SetupProducer() sarama.SyncProducer {
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		brokers = []string{"localhost:9092"}
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create Kafka producer")
	}

	logrus.WithField("brokers", brokers).Info("Kafka producer initialized")
	return producer
}