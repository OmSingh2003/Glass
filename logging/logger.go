package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LogLevel represents different log levels
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp     string            `json:"timestamp"`
	Level         string            `json:"level"`
	Message       string            `json:"message"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	NodeID        string            `json:"node_id,omitempty"`
	FunctionName  string            `json:"function_name,omitempty"`
	Duration      int64             `json:"duration_ms,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
	File          string            `json:"file,omitempty"`
	Line          int               `json:"line,omitempty"`
}

// Logger provides structured logging with correlation tracking
type Logger struct {
	level   LogLevel
	nodeID  string
	metrics *MetricsCollector
	mu      sync.RWMutex
}

// MetricsCollector collects performance metrics
type MetricsCollector struct {
	mu                sync.RWMutex
	functionCalls     map[string]int64
	functionDurations map[string][]time.Duration
	errorCounts       map[string]int64
	totalRequests     int64
	startTime         time.Time
}

// NewLogger creates a new structured logger
func NewLogger(nodeID string, level LogLevel) *Logger {
	return &Logger{
		level:   level,
		nodeID:  nodeID,
		metrics: NewMetricsCollector(),
	}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		functionCalls:     make(map[string]int64),
		functionDurations: make(map[string][]time.Duration),
		errorCounts:       make(map[string]int64),
		startTime:         time.Now(),
	}
}

// contextKey type for context values
type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	functionNameKey  contextKey = "function_name"
	startTimeKey     contextKey = "start_time"
)

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.New().String()
	}
	return context.WithValue(ctx, correlationIDKey, id)
}

// WithFunctionName adds a function name to the context
func WithFunctionName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, functionNameKey, name)
}

// WithStartTime adds a start time to the context
func WithStartTime(ctx context.Context) context.Context {
	return context.WithValue(ctx, startTimeKey, time.Now())
}

// getCorrelationID extracts correlation ID from context
func getCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// getFunctionName extracts function name from context
func getFunctionName(ctx context.Context) string {
	if name, ok := ctx.Value(functionNameKey).(string); ok {
		return name
	}
	return ""
}

// getStartTime extracts start time from context
func getStartTime(ctx context.Context) time.Time {
	if startTime, ok := ctx.Value(startTimeKey).(time.Time); ok {
		return startTime
	}
	return time.Now()
}

// log writes a log entry with the given level and message
func (l *Logger) log(ctx context.Context, level LogLevel, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		file = "unknown"
		line = 0
	}

	// Calculate duration if start time is available
	var duration int64
	if startTime := getStartTime(ctx); !startTime.IsZero() {
		duration = time.Since(startTime).Milliseconds()
	}

	entry := LogEntry{
		Timestamp:     time.Now().Format(time.RFC3339),
		Level:         levelNames[level],
		Message:       message,
		CorrelationID: getCorrelationID(ctx),
		NodeID:        l.nodeID,
		FunctionName:  getFunctionName(ctx),
		Duration:      duration,
		Fields:        fields,
		File:          file,
		Line:          line,
	}

	// JSON encode and output
	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	fmt.Println(string(jsonData))

	// Update metrics
	if functionName := getFunctionName(ctx); functionName != "" {
		l.metrics.RecordFunctionCall(functionName, time.Duration(duration)*time.Millisecond)
		if level == ERROR {
			l.metrics.RecordError(functionName)
		}
	}
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, DEBUG, message, f)
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, INFO, message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, WARN, message, f)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, ERROR, message, f)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, FATAL, message, f)
	os.Exit(1)
}

// RecordFunctionCall records a function call metric
func (mc *MetricsCollector) RecordFunctionCall(functionName string, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.functionCalls[functionName]++
	mc.functionDurations[functionName] = append(mc.functionDurations[functionName], duration)
	mc.totalRequests++
}

// RecordError records an error metric
func (mc *MetricsCollector) RecordError(functionName string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.errorCounts[functionName]++
}

// GetMetrics returns current metrics
func (l *Logger) GetMetrics() map[string]interface{} {
	l.metrics.mu.RLock()
	defer l.metrics.mu.RUnlock()

	metrics := map[string]interface{}{
		"uptime_seconds":  time.Since(l.metrics.startTime).Seconds(),
		"total_requests":  l.metrics.totalRequests,
		"function_calls":  l.metrics.functionCalls,
		"error_counts":    l.metrics.errorCounts,
		"node_id":         l.nodeID,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	// Calculate average durations
	avgDurations := make(map[string]float64)
	for funcName, durations := range l.metrics.functionDurations {
		if len(durations) > 0 {
			var total time.Duration
			for _, d := range durations {
				total += d
			}
			avgDurations[funcName] = float64(total.Milliseconds()) / float64(len(durations))
		}
	}
	metrics["avg_durations_ms"] = avgDurations

	return metrics
}

// Global logger instance
var globalLogger *Logger

// InitGlobalLogger initializes the global logger
func InitGlobalLogger(nodeID string, level LogLevel) {
	globalLogger = NewLogger(nodeID, level)
}

// GetGlobalLogger returns the global logger
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		globalLogger = NewLogger("unknown", INFO)
	}
	return globalLogger
}
