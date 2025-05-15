package kafka

import (
	"os"
	"strings"

	"github.com/IBM/sarama"

	"github.com/sirupsen/logrus"
)

// SetupConsumer initializes a Kafka consumer for a given topic.
func SetupConsumer(topic string, handler func([]byte)) {
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		brokers = []string{"localhost:9092"}
	}

	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating consumer")
	}

	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetNewest)
	if err != nil {
		logrus.WithError(err).Fatal("Error creating partition consumer")
	}

	logrus.WithField("topic", topic).Info("Started consuming from topic")
	go func() {
		for {
			select {
			case msg := <-partitionConsumer.Messages():
				logrus.WithField("topic", topic).Info("Received message")
				handler(msg.Value)
			case err := <-partitionConsumer.Errors():
				logrus.WithError(err).WithField("topic", topic).Error("Error consuming")
			}
		}
	}()
}