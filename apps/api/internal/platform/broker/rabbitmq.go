package broker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQConfig struct {
	Host           string
	Port           int
	VHost          string
	Username       string
	Password       string
	Exchange       string
	RoutingKey     string
	TLS            bool
	PublishTimeout time.Duration
}

type RabbitMQPublisher struct {
	config     RabbitMQConfig
	connection *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	routingKey string
	timeout    time.Duration
	returns    <-chan amqp.Return
	mutex      sync.Mutex
}

func NewRabbitMQPublisher(config RabbitMQConfig) (*RabbitMQPublisher, error) {
	if config.Host == "" || config.Port < 1 || config.Port > 65535 || config.Username == "" || config.Password == "" {
		return nil, errors.New("RabbitMQ host, port, username, and password are required")
	}
	if !safeName(config.Exchange, 160) || !safeName(config.RoutingKey, 160) {
		return nil, errors.New("RabbitMQ exchange and routing key must be static safe names")
	}
	if config.PublishTimeout <= 0 || config.PublishTimeout > time.Minute {
		return nil, errors.New("RabbitMQ publish timeout must be between 1ns and 1m")
	}
	return &RabbitMQPublisher{config: config, exchange: config.Exchange, routingKey: config.RoutingKey, timeout: config.PublishTimeout}, nil
}

func (publisher *RabbitMQPublisher) connectLocked() error {
	scheme := "amqp"
	if publisher.config.TLS {
		scheme = "amqps"
	}
	endpoint := url.URL{Scheme: scheme, Host: fmt.Sprintf("%s:%d", publisher.config.Host, publisher.config.Port), Path: publisher.config.VHost}
	endpoint.User = url.UserPassword(publisher.config.Username, publisher.config.Password)
	dialConfig := amqp.Config{Heartbeat: 10 * time.Second, Locale: "en_US"}
	if publisher.config.TLS {
		dialConfig.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: publisher.config.Host}
	}
	connection, err := amqp.DialConfig(endpoint.String(), dialConfig)
	if err != nil {
		return errors.New("connect to RabbitMQ")
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return errors.New("open RabbitMQ channel")
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return errors.New("enable RabbitMQ publisher confirms")
	}
	if err := channel.ExchangeDeclare(publisher.exchange, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return errors.New("declare RabbitMQ exchange")
	}
	queue, err := channel.QueueDeclare("veltrix.domain-events.v1", true, false, false, false, amqp.Table{"x-queue-type": "quorum"})
	if err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return errors.New("declare RabbitMQ quorum queue")
	}
	if err := channel.QueueBind(queue.Name, publisher.routingKey, publisher.exchange, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return errors.New("bind RabbitMQ queue")
	}
	publisher.connection = connection
	publisher.channel = channel
	publisher.returns = channel.NotifyReturn(make(chan amqp.Return, 1))
	return nil
}

func (publisher *RabbitMQPublisher) disconnectLocked() {
	if publisher.channel != nil {
		_ = publisher.channel.Close()
	}
	if publisher.connection != nil {
		_ = publisher.connection.Close()
	}
	publisher.channel = nil
	publisher.connection = nil
	publisher.returns = nil
}

func (publisher *RabbitMQPublisher) Publish(ctx context.Context, envelope Envelope) error {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()
	if publisher.channel == nil || publisher.channel.IsClosed() {
		publisher.disconnectLocked()
		if err := publisher.connectLocked(); err != nil {
			return err
		}
	}
	select {
	case <-publisher.returns:
	default:
	}
	encoded, err := envelope.Marshal()
	if err != nil {
		return err
	}
	publishCtx, cancel := context.WithTimeout(ctx, publisher.timeout)
	defer cancel()
	confirmation, err := publisher.channel.PublishWithDeferredConfirmWithContext(publishCtx, publisher.exchange, publisher.routingKey, true, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		MessageId:    envelope.EventID,
		Type:         envelope.EventType,
		Timestamp:    time.Now().UTC(),
		Body:         encoded,
	})
	if err != nil {
		publisher.disconnectLocked()
		return errors.New("publish RabbitMQ message")
	}
	if confirmation == nil {
		return errors.New("RabbitMQ publish confirmation is unavailable")
	}
	acknowledged, waitErr := confirmation.WaitContext(publishCtx)
	if waitErr != nil || !acknowledged {
		publisher.disconnectLocked()
		return errors.New("RabbitMQ publish was not confirmed")
	}
	select {
	case returned := <-publisher.returns:
		if returned.MessageId == envelope.EventID {
			return errors.New("RabbitMQ message was returned as unroutable")
		}
	default:
	}
	return nil
}

func (publisher *RabbitMQPublisher) Close() error {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()
	publisher.disconnectLocked()
	return nil
}
