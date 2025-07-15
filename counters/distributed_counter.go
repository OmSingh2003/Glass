package counters

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"glass/logging"
	"glass/state"
)

// CounterType represents different types of counters
type CounterType int

const (
	// SimpleCounter - basic increment/decrement counter
	SimpleCounter CounterType = iota
	// PartitionedCounter - counter split across multiple partitions for high throughput
	PartitionedCounter
	// CRDTCounter - Conflict-free Replicated Data Type counter
	CRDTCounter
	// RateLimitCounter - counter with automatic expiration for rate limiting
	RateLimitCounter
)

// CounterValue represents a counter value with metadata
type CounterValue struct {
	Value      int64     `json:"value"`
	NodeID     string    `json:"node_id"`
	Version    int64     `json:"version"`
	Timestamp  time.Time `json:"timestamp"`
	TTL        int64     `json:"ttl,omitempty"` // For rate limiting counters
}

// PartitionedCounterValue represents a partitioned counter
type PartitionedCounterValue struct {
	Partitions map[string]int64 `json:"partitions"`
	Total      int64            `json:"total"`
	NodeID     string           `json:"node_id"`
	Version    int64            `json:"version"`
	Timestamp  time.Time        `json:"timestamp"`
}

// CRDTCounterValue represents a CRDT counter with vector clocks
type CRDTCounterValue struct {
	NodeValues    map[string]int64 `json:"node_values"`
	VectorClock   map[string]int64 `json:"vector_clock"`
	Total         int64            `json:"total"`
	LastUpdated   time.Time        `json:"last_updated"`
}

// DistributedCounter manages different types of distributed counters
type DistributedCounter struct {
	stateManager *state.Manager
	nodeID       string
	logger       *logging.Logger
	mu           sync.RWMutex
	
	// Cache for frequently accessed counters
	cache map[string]*CounterValue
	cacheTTL time.Duration
}

// NewDistributedCounter creates a new distributed counter manager
func NewDistributedCounter(stateManager *state.Manager, nodeID string, logger *logging.Logger) *DistributedCounter {
	return &DistributedCounter{
		stateManager: stateManager,
		nodeID:       nodeID,
		logger:       logger,
		cache:        make(map[string]*CounterValue),
		cacheTTL:     5 * time.Second,
	}
}

// Increment increments a counter by the given delta
func (dc *DistributedCounter) Increment(ctx context.Context, counterName string, delta int64, counterType CounterType) (int64, error) {
	ctx = logging.WithFunctionName(ctx, "counter_increment")
	ctx = logging.WithStartTime(ctx)
	
	dc.logger.Info(ctx, "Incrementing counter", map[string]interface{}{
		"counter_name": counterName,
		"delta":        delta,
		"counter_type": counterType,
	})

	switch counterType {
	case SimpleCounter:
		return dc.incrementSimple(ctx, counterName, delta)
	case PartitionedCounter:
		return dc.incrementPartitioned(ctx, counterName, delta)
	case CRDTCounter:
		return dc.incrementCRDT(ctx, counterName, delta)
	case RateLimitCounter:
		return dc.incrementRateLimit(ctx, counterName, delta)
	default:
		return 0, fmt.Errorf("unsupported counter type: %v", counterType)
	}
}

// Get retrieves the current value of a counter
func (dc *DistributedCounter) Get(ctx context.Context, counterName string, counterType CounterType) (int64, error) {
	ctx = logging.WithFunctionName(ctx, "counter_get")
	ctx = logging.WithStartTime(ctx)
	
	switch counterType {
	case SimpleCounter:
		return dc.getSimple(ctx, counterName)
	case PartitionedCounter:
		return dc.getPartitioned(ctx, counterName)
	case CRDTCounter:
		return dc.getCRDT(ctx, counterName)
	case RateLimitCounter:
		return dc.getRateLimit(ctx, counterName)
	default:
		return 0, fmt.Errorf("unsupported counter type: %v", counterType)
	}
}

// incrementSimple handles simple counter increments
func (dc *DistributedCounter) incrementSimple(ctx context.Context, counterName string, delta int64) (int64, error) {
	key := fmt.Sprintf("counter:simple:%s", counterName)
	
	// Use Redis INCRBY for atomic increment
	newValue, err := dc.stateManager.Increment(ctx, key, delta)
	if err != nil {
		dc.logger.Error(ctx, "Failed to increment simple counter", map[string]interface{}{
			"counter_name": counterName,
			"error":        err.Error(),
		})
		return 0, err
	}
	
	// Update cache
	dc.updateCache(key, &CounterValue{
		Value:     int64(newValue),
		NodeID:    dc.nodeID,
		Version:   time.Now().UnixNano(),
		Timestamp: time.Now(),
	})
	
	return int64(newValue), nil
}

// incrementPartitioned handles partitioned counter increments
func (dc *DistributedCounter) incrementPartitioned(ctx context.Context, counterName string, delta int64) (int64, error) {
	// Use node ID as partition key for better distribution
	partitionKey := fmt.Sprintf("counter:partitioned:%s:partition:%s", counterName, dc.nodeID)
	totalKey := fmt.Sprintf("counter:partitioned:%s:total", counterName)
	
	// Increment partition counter
	_, err := dc.stateManager.Increment(ctx, partitionKey, delta)
	if err != nil {
		return 0, err
	}
	
	// Update total counter
	newTotal, err := dc.stateManager.Increment(ctx, totalKey, delta)
	if err != nil {
		return 0, err
	}
	
	dc.logger.Info(ctx, "Incremented partitioned counter", map[string]interface{}{
		"counter_name": counterName,
		"partition":    dc.nodeID,
		"delta":        delta,
		"new_total":    newTotal,
	})
	
	return int64(newTotal), nil
}

// incrementCRDT handles CRDT counter increments
func (dc *DistributedCounter) incrementCRDT(ctx context.Context, counterName string, delta int64) (int64, error) {
	key := fmt.Sprintf("counter:crdt:%s", counterName)
	
	// Get current CRDT value
	currentValue, err := dc.getCRDTValue(ctx, key)
	if err != nil {
		return 0, err
	}
	
	// Update this node's value
	if currentValue.NodeValues == nil {
		currentValue.NodeValues = make(map[string]int64)
	}
	if currentValue.VectorClock == nil {
		currentValue.VectorClock = make(map[string]int64)
	}
	
	currentValue.NodeValues[dc.nodeID] += delta
	currentValue.VectorClock[dc.nodeID] = time.Now().UnixNano()
	currentValue.LastUpdated = time.Now()
	
	// Calculate total
	currentValue.Total = 0
	for _, value := range currentValue.NodeValues {
		currentValue.Total += value
	}
	
	// Store updated value
	jsonData, err := json.Marshal(currentValue)
	if err != nil {
		return 0, err
	}
	
	err = dc.stateManager.Set(ctx, key, uint64(len(jsonData)))
	if err != nil {
		return 0, err
	}
	
	// Store the actual JSON data
	err = dc.stateManager.Set(ctx, key+":data", uint64(len(jsonData)))
	if err != nil {
		return 0, err
	}
	
	dc.logger.Info(ctx, "Updated CRDT counter", map[string]interface{}{
		"counter_name": counterName,
		"node_id":      dc.nodeID,
		"delta":        delta,
		"new_total":    currentValue.Total,
		"vector_clock": currentValue.VectorClock,
	})
	
	return currentValue.Total, nil
}

// incrementRateLimit handles rate limit counter increments with TTL
func (dc *DistributedCounter) incrementRateLimit(ctx context.Context, counterName string, delta int64) (int64, error) {
	// Include current time window in the key
	currentWindow := time.Now().Unix() / 60 // 1-minute windows
	key := fmt.Sprintf("counter:ratelimit:%s:window:%d", counterName, currentWindow)
	
	// Increment with TTL
	newValue, err := dc.stateManager.Increment(ctx, key, delta)
	if err != nil {
		return 0, err
	}
	
	// Set TTL for automatic cleanup (Redis doesn't support TTL with INCRBY directly)
	// We'll implement a background cleanup process
	
	dc.logger.Info(ctx, "Incremented rate limit counter", map[string]interface{}{
		"counter_name": counterName,
		"window":       currentWindow,
		"delta":        delta,
		"new_value":    newValue,
	})
	
	return int64(newValue), nil
}

// getSimple retrieves simple counter value
func (dc *DistributedCounter) getSimple(ctx context.Context, counterName string) (int64, error) {
	key := fmt.Sprintf("counter:simple:%s", counterName)
	
	// Check cache first
	if cached := dc.getFromCache(key); cached != nil {
		return cached.Value, nil
	}
	
	value, err := dc.stateManager.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	
	// Update cache
	dc.updateCache(key, &CounterValue{
		Value:     int64(value),
		NodeID:    dc.nodeID,
		Timestamp: time.Now(),
	})
	
	return int64(value), nil
}

// getPartitioned retrieves partitioned counter total
func (dc *DistributedCounter) getPartitioned(ctx context.Context, counterName string) (int64, error) {
	totalKey := fmt.Sprintf("counter:partitioned:%s:total", counterName)
	
	value, err := dc.stateManager.Get(ctx, totalKey)
	if err != nil {
		return 0, err
	}
	
	return int64(value), nil
}

// getCRDT retrieves CRDT counter value
func (dc *DistributedCounter) getCRDT(ctx context.Context, counterName string) (int64, error) {
	key := fmt.Sprintf("counter:crdt:%s", counterName)
	
	crdtValue, err := dc.getCRDTValue(ctx, key)
	if err != nil {
		return 0, err
	}
	
	return crdtValue.Total, nil
}

// getRateLimit retrieves rate limit counter value for current window
func (dc *DistributedCounter) getRateLimit(ctx context.Context, counterName string) (int64, error) {
	currentWindow := time.Now().Unix() / 60
	key := fmt.Sprintf("counter:ratelimit:%s:window:%d", counterName, currentWindow)
	
	value, err := dc.stateManager.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	
	return int64(value), nil
}

// getCRDTValue retrieves and parses CRDT counter value
func (dc *DistributedCounter) getCRDTValue(ctx context.Context, key string) (*CRDTCounterValue, error) {
	// This is a simplified implementation
	// In a real system, you'd store the JSON data separately
	return &CRDTCounterValue{
		NodeValues:  make(map[string]int64),
		VectorClock: make(map[string]int64),
		Total:       0,
		LastUpdated: time.Now(),
	}, nil
}

// updateCache updates the local cache
func (dc *DistributedCounter) updateCache(key string, value *CounterValue) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	dc.cache[key] = value
}

// getFromCache retrieves from local cache
func (dc *DistributedCounter) getFromCache(key string) *CounterValue {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	if value, exists := dc.cache[key]; exists {
		// Check if cache entry is still valid
		if time.Since(value.Timestamp) < dc.cacheTTL {
			return value
		}
		// Clean up expired entry
		delete(dc.cache, key)
	}
	
	return nil
}

// GetCounterStats returns statistics about counter usage
func (dc *DistributedCounter) GetCounterStats(ctx context.Context) map[string]interface{} {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	stats := map[string]interface{}{
		"cache_size":    len(dc.cache),
		"node_id":       dc.nodeID,
		"cache_ttl_ms":  dc.cacheTTL.Milliseconds(),
		"timestamp":     time.Now().Format(time.RFC3339),
	}
	
	// Add cache hit ratio and other metrics
	return stats
}

// CleanupExpiredCounters removes expired rate limit counters
func (dc *DistributedCounter) CleanupExpiredCounters(ctx context.Context) error {
	// This would implement cleanup logic for expired rate limit counters
	dc.logger.Info(ctx, "Cleaning up expired counters")
	return nil
}
