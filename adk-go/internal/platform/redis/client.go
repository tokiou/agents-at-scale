package redis

import (
	"context"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const statusTTL = 24 * time.Hour

func New(ctx context.Context, rawURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

func SetJobStatus(ctx context.Context, client *redis.Client, jobID, status string) error {
	if err := client.Set(ctx, "job:"+jobID+":status", status, statusTTL).Err(); err != nil {
		return fmt.Errorf("set job status: %w", err)
	}
	return nil
}
