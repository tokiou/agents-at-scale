package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const statusTTL = 24 * time.Hour

// New connects with at most poolSize connections, the same cap the Python
// runtime uses, so neither runtime opens one connection per request.
func New(ctx context.Context, rawURL string, poolSize int) (*redis.Client, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	options.PoolSize = poolSize

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

// GetJobStatus returns the stored status or "" when the job is unknown.
func GetJobStatus(ctx context.Context, client *redis.Client, jobID string) (string, error) {
	status, err := client.Get(ctx, "job:"+jobID+":status").Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get job status: %w", err)
	}
	return status, nil
}

// MarkJobTiming records a lifecycle timestamp (epoch milliseconds) in the
// job:<id>:timing hash. Only the first write of a field is kept, so retries
// do not move published_ms or started_ms.
func MarkJobTiming(ctx context.Context, client *redis.Client, jobID, field string, at time.Time) error {
	key := "job:" + jobID + ":timing"
	pipe := client.TxPipeline()
	pipe.HSetNX(ctx, key, field, at.UnixMilli())
	pipe.Expire(ctx, key, statusTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set job timing: %w", err)
	}
	return nil
}

// SetJobTiming overwrites a timing field; used for values such as attempts
// and finished_ms that reflect the latest attempt.
func SetJobTiming(ctx context.Context, client *redis.Client, jobID, field string, value int64) error {
	key := "job:" + jobID + ":timing"
	pipe := client.TxPipeline()
	pipe.HSet(ctx, key, field, value)
	pipe.Expire(ctx, key, statusTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set job timing: %w", err)
	}
	return nil
}

// ClaimIdempotencyKey stores jobID under the key unless it already exists.
// It returns the job ID that owns the key and whether this call claimed it.
func ClaimIdempotencyKey(ctx context.Context, client *redis.Client, key, jobID string) (string, bool, error) {
	redisKey := "airline:idempotency:" + key
	claimed, err := client.SetNX(ctx, redisKey, jobID, statusTTL).Result()
	if err != nil {
		return "", false, fmt.Errorf("claim idempotency key: %w", err)
	}
	if claimed {
		return jobID, true, nil
	}
	owner, err := client.Get(ctx, redisKey).Result()
	if err != nil {
		return "", false, fmt.Errorf("get idempotency key: %w", err)
	}
	return owner, false, nil
}

func ReleaseIdempotencyKey(ctx context.Context, client *redis.Client, key string) error {
	if err := client.Del(ctx, "airline:idempotency:"+key).Err(); err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}

// releaseLock deletes the lock only when it still holds our token, so an
// expired lock that another worker re-acquired is left alone.
var releaseLock = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)

// AcquireConversationLock takes the per-conversation lock for ttl.
func AcquireConversationLock(ctx context.Context, client *redis.Client, conversation, token string, ttl time.Duration) (bool, error) {
	acquired, err := client.SetNX(ctx, "airline:lock:"+conversation, token, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire conversation lock: %w", err)
	}
	return acquired, nil
}

func ReleaseConversationLock(ctx context.Context, client *redis.Client, conversation, token string) error {
	if err := releaseLock.Run(ctx, client, []string{"airline:lock:" + conversation}, token).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("release conversation lock: %w", err)
	}
	return nil
}
