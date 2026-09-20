package redis

import (
	"context"
	"encoding/json"
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

// SetJobMetadata keeps the small amount of HITL data needed by an external
// client to resume a job. It is deliberately separate from the status key so
// existing status readers keep their contract.
func SetJobMetadata(ctx context.Context, client *redis.Client, jobID string, metadata any) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode job metadata: %w", err)
	}
	if err := client.Set(ctx, "job:"+jobID+":metadata", payload, statusTTL).Err(); err != nil {
		return fmt.Errorf("set job metadata: %w", err)
	}
	return nil
}
