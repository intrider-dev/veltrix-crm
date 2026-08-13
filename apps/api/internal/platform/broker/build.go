package broker

import (
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
)

func BuildPublishers(cfg config.Config) (map[string]Publisher, error) {
	publishers := make(map[string]Publisher, 2)
	closeAll := func() {
		for _, publisher := range publishers {
			_ = publisher.Close()
		}
	}
	if cfg.BrokerMode == "kafka" || cfg.BrokerMode == "combined" {
		publisher, err := NewKafkaPublisher(KafkaConfig{
			Brokers: cfg.KafkaBrokers, Topic: cfg.KafkaTopic, Username: cfg.KafkaUsername,
			Password: cfg.KafkaPassword, TLS: cfg.KafkaTLS, PublishTimeout: cfg.BrokerPublishTimeout,
			MaxBuffered: cfg.KafkaMaxBufferedRecords, BatchMaxBytes: cfg.KafkaBatchMaxBytes,
			AutoCreateTopic: cfg.KafkaAutoCreateTopics,
		})
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("configure Kafka publisher: %w", err)
		}
		publishers["kafka"] = publisher
	}
	if cfg.BrokerMode == "rabbitmq" || cfg.BrokerMode == "combined" {
		publisher, err := NewRabbitMQPublisher(RabbitMQConfig{
			Host: cfg.RabbitMQHost, Port: cfg.RabbitMQPort, VHost: cfg.RabbitMQVHost,
			Username: cfg.RabbitMQUsername, Password: cfg.RabbitMQPassword,
			Exchange: cfg.RabbitMQExchange, RoutingKey: cfg.RabbitMQRoutingKey,
			TLS: cfg.RabbitMQTLS, PublishTimeout: cfg.BrokerPublishTimeout,
		})
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("configure RabbitMQ publisher: %w", err)
		}
		publishers["rabbitmq"] = publisher
	}
	return publishers, nil
}

func ClosePublishers(publishers map[string]Publisher) error {
	var firstErr error
	for _, publisher := range publishers {
		if err := publisher.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
