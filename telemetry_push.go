package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const telemetryRedisTimeout = 5 * time.Second

// pushToEndpoint sends the JSON payload to the configured Redis key.
// It tries both RPUSH (for list-based consumers) and PUBLISH (for channel-based consumers)
// to ensure the event reaches the pipeline regardless of its configuration.
func pushToEndpoint(endpoint string, key string, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), telemetryRedisTimeout)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:        endpoint,
		DialTimeout: telemetryRedisTimeout,
		ReadTimeout: telemetryRedisTimeout,
	})
	defer client.Close()

	// Use RPUSH for list-based pipelines (Logstash redis input with data_type=list)
	if err := client.RPush(ctx, key, string(payload)).Err(); err != nil {
		return fmt.Errorf("Redis RPUSH to %s/%s failed: %w", endpoint, key, err)
	}

	// Also PUBLISH for channel-based pipelines (Logstash redis input with data_type=channel)
	if err := client.Publish(ctx, key, string(payload)).Err(); err != nil {
		// Non-fatal: log but don't fail if PUBLISH errors (no subscribers is fine)
		log.Debugf("Telemetry: Redis PUBLISH to channel %s returned: %v", key, err)
	}

	return nil
}
