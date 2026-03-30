package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Reporter struct {
	client *redis.Client
}

func NewReporter(client *redis.Client) *Reporter {
	return &Reporter{client: client}
}

type Stats struct {
	TotalRequests   int64            `json:"total_requests"`
	ClientCounts    map[string]int64 `json:"client_counts"`
	RouteCounts     map[string]int64 `json:"route_counts"`
	ErrorCounts     map[string]int64 `json:"error_counts"`
	BruteForceFlags map[string]int64 `json:"brute_force_flags"`
	RecentRequests  []RequestLog     `json:"recent_requests"`
}

// GetStats aggregates analytics data from Redis
func (r *Reporter) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		ClientCounts:    make(map[string]int64),
		RouteCounts:     make(map[string]int64),
		ErrorCounts:     make(map[string]int64),
		BruteForceFlags: make(map[string]int64),
	}

	// Get recent requests
	raw, err := r.client.LRange(ctx, "analytics:requests", 0, 49).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent requests: %w", err)
	}

	for _, item := range raw {
		var entry RequestLog
		if err := json.Unmarshal([]byte(item), &entry); err == nil {
			stats.RecentRequests = append(stats.RecentRequests, entry)
			stats.TotalRequests++
		}
	}

	// Get client counts
	clientKeys, _ := r.client.Keys(ctx, "analytics:client:*:count").Result()
	for _, key := range clientKeys {
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			clientID := parts[2]
			count, _ := r.client.Get(ctx, key).Result()
			val, _ := strconv.ParseInt(count, 10, 64)
			stats.ClientCounts[clientID] = val
		}
	}

	// Get route counts
	routeKeys, _ := r.client.Keys(ctx, "analytics:route:*:count").Result()
	for _, key := range routeKeys {
		route := strings.TrimSuffix(strings.TrimPrefix(key, "analytics:route:"), ":count")
		count, _ := r.client.Get(ctx, key).Result()
		val, _ := strconv.ParseInt(count, 10, 64)
		stats.RouteCounts[route] = val
	}

	// Get error counts
	errorKeys, _ := r.client.Keys(ctx, "analytics:client:*:errors").Result()
	for _, key := range errorKeys {
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			clientID := parts[2]
			count, _ := r.client.Get(ctx, key).Result()
			val, _ := strconv.ParseInt(count, 10, 64)
			stats.ErrorCounts[clientID] = val
		}
	}

	// Get brute force flags — clients with 5+ 401s in 10 min
	bruteKeys, _ := r.client.Keys(ctx, "analytics:bruteforce:*").Result()
	for _, key := range bruteKeys {
		clientID := strings.TrimPrefix(key, "analytics:bruteforce:")
		count, _ := r.client.Get(ctx, key).Result()
		val, _ := strconv.ParseInt(count, 10, 64)
		if val >= 5 {
			stats.BruteForceFlags[clientID] = val
		}
	}

	return stats, nil
}
