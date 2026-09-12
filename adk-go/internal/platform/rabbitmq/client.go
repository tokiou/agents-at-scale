package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type Job struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

type Client struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
	queue      string
}

func New(rawURL, queue string) (*Client, error) {
	connection, err := amqp091.Dial(rawURL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	if _, err = channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		channel.Close()
		connection.Close()
		return nil, fmt.Errorf("declare rabbitmq queue: %w", err)
	}
	return &Client{connection: connection, channel: channel, queue: queue}, nil
}

func (c *Client) Publish(ctx context.Context, job Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode job: %w", err)
	}
	if err := c.channel.PublishWithContext(ctx, "", c.queue, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         body,
	}); err != nil {
		return fmt.Errorf("publish job: %w", err)
	}
	return nil
}

func (c *Client) Consume() (<-chan amqp091.Delivery, error) {
	deliveries, err := c.channel.Consume(c.queue, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("consume jobs: %w", err)
	}
	return deliveries, nil
}

func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.channel != nil {
		c.channel.Close()
	}
	if c.connection != nil {
		c.connection.Close()
	}
}
