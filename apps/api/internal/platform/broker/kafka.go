package broker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

type KafkaConfig struct {
	Brokers         []string
	Topic           string
	Username        string
	Password        string
	TLS             bool
	PublishTimeout  time.Duration
	MaxBuffered     int
	BatchMaxBytes   int32
	AutoCreateTopic bool
}

type KafkaPublisher struct {
	client  *kgo.Client
	topic   string
	timeout time.Duration
}

func NewKafkaPublisher(config KafkaConfig) (*KafkaPublisher, error) {
	if len(config.Brokers) == 0 || !safeName(config.Topic, 249) {
		return nil, errors.New("Kafka brokers and a static topic are required")
	}
	if config.PublishTimeout <= 0 || config.PublishTimeout > time.Minute {
		return nil, errors.New("Kafka publish timeout must be between 1ns and 1m")
	}
	if config.MaxBuffered < 1 || config.MaxBuffered > 100000 {
		return nil, errors.New("Kafka buffered record limit must be between 1 and 100000")
	}
	if config.BatchMaxBytes < 1024 || config.BatchMaxBytes > MaxMessageSize*4 {
		return nil, errors.New("Kafka batch byte limit is outside the supported range")
	}
	options := []kgo.Opt{
		kgo.SeedBrokers(config.Brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.MaxBufferedRecords(config.MaxBuffered),
		kgo.ProducerBatchMaxBytes(config.BatchMaxBytes),
		kgo.ProducerLinger(5 * time.Millisecond),
		kgo.ProducerBatchCompression(kgo.ZstdCompression()),
		kgo.RecordRetries(8),
		kgo.RecordDeliveryTimeout(config.PublishTimeout),
	}
	if config.AutoCreateTopic {
		options = append(options, kgo.AllowAutoTopicCreation())
	}
	if config.TLS {
		options = append(options, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}
	if config.Username != "" || config.Password != "" {
		if config.Username == "" || config.Password == "" {
			return nil, errors.New("Kafka username and password must be configured together")
		}
		options = append(options, kgo.SASL(scram.Auth{User: config.Username, Pass: config.Password}.AsSha512Mechanism()))
	}
	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, errors.New("configure Kafka publisher")
	}
	return &KafkaPublisher{client: client, topic: config.Topic, timeout: config.PublishTimeout}, nil
}

func (publisher *KafkaPublisher) Publish(ctx context.Context, envelope Envelope) error {
	encoded, err := envelope.Marshal()
	if err != nil {
		return err
	}
	publishCtx, cancel := context.WithTimeout(ctx, publisher.timeout)
	defer cancel()
	result := publisher.client.ProduceSync(publishCtx, &kgo.Record{
		Topic: publisher.topic,
		Key:   []byte(envelope.PartitionKey()),
		Value: encoded,
		Headers: []kgo.RecordHeader{
			{Key: "message-id", Value: []byte(envelope.EventID)},
			{Key: "schema-version", Value: []byte("1")},
		},
	})
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("Kafka publish was not acknowledged: %w", err)
	}
	return nil
}

func (publisher *KafkaPublisher) Close() error {
	publisher.client.Close()
	return nil
}
