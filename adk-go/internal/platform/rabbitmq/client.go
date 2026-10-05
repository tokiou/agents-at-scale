package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Job is the queued envelope. Attempt counts failed processing attempts and
// Error keeps the last failure reason when the job is dead-lettered.
type Job struct {
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
	Attempt int             `json:"attempt"`
	Error   string          `json:"error,omitempty"`
}

// Client publishes and consumes jobs. Besides the main queue it declares one
// delay queue per retry level ("<queue>.retry.<n>") whose messages expire back
// into the main queue, and a "<queue>.dead" queue for jobs that will not be
// retried.
type Client struct {
	connection  *amqp091.Connection
	channel     *amqp091.Channel
	queue       string
	retryQueues []string
}

func New(rawURL, queue string, retryDelays []time.Duration) (*Client, error) {
	connection, err := amqp091.Dial(rawURL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	client := &Client{connection: connection, channel: channel, queue: queue}
	if err := client.declare(retryDelays); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) declare(retryDelays []time.Duration) error {
	if _, err := c.channel.QueueDeclare(c.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq queue: %w", err)
	}
	if _, err := c.channel.QueueDeclare(DeadQueue(c.queue), true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare rabbitmq dead queue: %w", err)
	}
	for level, delay := range retryDelays {
		name := RetryQueue(c.queue, level+1)
		if _, err := c.channel.QueueDeclare(name, true, false, false, false, amqp091.Table{
			"x-message-ttl":             delay.Milliseconds(),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": c.queue,
		}); err != nil {
			return fmt.Errorf("declare rabbitmq retry queue %s: %w", name, err)
		}
		c.retryQueues = append(c.retryQueues, name)
	}
	return nil
}

// RetryQueue names the delay queue for a retry level (1-based).
func RetryQueue(queue string, level int) string {
	return fmt.Sprintf("%s.retry.%d", queue, level)
}

// DeadQueue names the queue that keeps jobs that will not be retried.
func DeadQueue(queue string) string {
	return queue + ".dead"
}

func (c *Client) Publish(ctx context.Context, job Job) error {
	return c.publish(ctx, c.queue, job)
}

// Retry publishes the job to the delay queue of the given level (1-based);
// levels beyond the declared ones use the longest delay.
func (c *Client) Retry(ctx context.Context, job Job, level int) error {
	if len(c.retryQueues) == 0 {
		return fmt.Errorf("no retry queues declared")
	}
	level = min(max(level, 1), len(c.retryQueues))
	return c.publish(ctx, c.retryQueues[level-1], job)
}

func (c *Client) DeadLetter(ctx context.Context, job Job) error {
	return c.publish(ctx, DeadQueue(c.queue), job)
}

func (c *Client) publish(ctx context.Context, queue string, job Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode job: %w", err)
	}
	if err := c.channel.PublishWithContext(ctx, "", queue, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         body,
	}); err != nil {
		return fmt.Errorf("publish job: %w", err)
	}
	return nil
}

// Consume starts delivering jobs with at most prefetch unacknowledged
// messages in flight for this consumer.
func (c *Client) Consume(prefetch int) (<-chan amqp091.Delivery, error) {
	if err := c.channel.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("set rabbitmq prefetch: %w", err)
	}
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
