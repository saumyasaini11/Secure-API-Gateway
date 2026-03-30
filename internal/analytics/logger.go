package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RequestLog struct {
	RequestID  string    `json:"request_id"`
	ClientID   string    `json:"client_id"`
	Route      string    `json:"route"`
	Method     string    `json:"method"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	Timestamp  time.Time `json:"timestamp"`
	IP         string    `json:"ip"`
}

type Logger struct {
	client *redis.Client
}

func NewLogger(client *redis.Client) *Logger {
	return &Logger{client: client}
}

// Log stores a request log entry in Redis
func (l *Logger) Log(ctx context.Context, entry RequestLog) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	key := "analytics:requests"
	pipe := l.client.TxPipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 999)

	// Per-client request count
	clientKey := fmt.Sprintf("analytics:client:%s:count", entry.ClientID)
	pipe.Incr(ctx, clientKey)
	pipe.Expire(ctx, clientKey, 24*time.Hour)

	// Per-route request count
	routeKey := fmt.Sprintf("analytics:route:%s:count", entry.Route)
	pipe.Incr(ctx, routeKey)
	pipe.Expire(ctx, routeKey, 24*time.Hour)

	// Error tracking
	if entry.StatusCode >= 400 {
		errorKey := fmt.Sprintf("analytics:client:%s:errors", entry.ClientID)
		pipe.Incr(ctx, errorKey)
		pipe.Expire(ctx, errorKey, 24*time.Hour)

		// Brute force detection — track 401s in short window
		if entry.StatusCode == 401 {
			bruteKey := fmt.Sprintf("analytics:bruteforce:%s", entry.ClientID)
			pipe.Incr(ctx, bruteKey)
			pipe.Expire(ctx, bruteKey, 10*time.Minute)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// WrappedResponseWriter captures the status code written by handlers
type WrappedResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func NewWrappedResponseWriter(w http.ResponseWriter) *WrappedResponseWriter {
	return &WrappedResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
}

func (w *WrappedResponseWriter) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}
