package state

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Manager handles the connection to the state store (Redis).
type Manager struct {
	client *redis.Client
}

// NewManager creates and returns a new state manager.
func NewManager() (*Manager, error) {
	return NewManagerWithOptions(&redis.Options{
		Addr: "localhost:6379", // Default Redis address
		DB:   0,                // Default database
	})
}

// NewManagerWithOptions creates a new state manager with custom Redis options.
func NewManagerWithOptions(opts *redis.Options) (*Manager, error) {
	rdb := redis.NewClient(opts)

	// Ping the Redis server to check the connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", opts.Addr, err)
	}

	return &Manager{client: rdb}, nil
}

// Set stores a key-value pair.
func (m *Manager) Set(ctx context.Context, key string, value uint64) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	return m.client.Set(ctx, key, value, 0).Err()
}

// Get retrieves a value by key.
func (m *Manager) Get(ctx context.Context, key string) (uint64, error) {
	if key == "" {
		return 0, fmt.Errorf("key cannot be empty")
	}

	val, err := m.client.Get(ctx, key).Uint64()
	if err == redis.Nil {
		return 0, nil // Key does not exist, return 0
	}
	return val, err
}

// Exists checks if a key exists in the state store.
func (m *Manager) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("key cannot be empty")
	}

	count, err := m.client.Exists(ctx, key).Result()
	return count > 0, err
}

// Delete removes a key from the state store.
func (m *Manager) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	return m.client.Del(ctx, key).Err()
}

// Increment atomically increments a key's value by delta and returns the new value.
func (m *Manager) Increment(ctx context.Context, key string, delta int64) (uint64, error) {
	if key == "" {
		return 0, fmt.Errorf("key cannot be empty")
	}

	result, err := m.client.IncrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, err
	}
	return uint64(result), nil
}

// Close closes the Redis connection.
func (m *Manager) Close() error {
	return m.client.Close()
}
