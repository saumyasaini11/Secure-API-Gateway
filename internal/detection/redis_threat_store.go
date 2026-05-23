package detection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	threatsListKey        = "analytics:threats"
	threatsTodayKeyFmt    = "analytics:threats:today:%s"
	threatsTypeKeyFmt     = "analytics:threats:type:%s"
	threatsEndpointKeyFmt = "analytics:threats:endpoint:%s"
	threatsListMaxLen     = 200
)

// RedisThreatStore persists and queries threat events using Redis.
// Key schema follows the existing analytics:* convention.
type RedisThreatStore struct {
	client *redis.Client
}

// NewRedisThreatStore creates a Redis-backed ThreatStore.
func NewRedisThreatStore(client *redis.Client) *RedisThreatStore {
	return &RedisThreatStore{client: client}
}

// LogThreat persists a ThreatLog entry and increments per-type and per-endpoint counters.
func (r *RedisThreatStore) LogThreat(ctx context.Context, entry ThreatLog) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal threat log: %w", err)
	}

	pipe := r.client.TxPipeline()

	// Main threat list — keeps the last 200 entries
	pipe.LPush(ctx, threatsListKey, data)
	pipe.LTrim(ctx, threatsListKey, 0, threatsListMaxLen-1)

	// Today's counter — keyed by UTC date, expires after 48h
	todayKey := fmt.Sprintf(threatsTodayKeyFmt, time.Now().UTC().Format("2006-01-02"))
	pipe.Incr(ctx, todayKey)
	pipe.Expire(ctx, todayKey, 48*time.Hour)

	// Per-attack-type counter (no expiry — all-time totals)
	pipe.Incr(ctx, fmt.Sprintf(threatsTypeKeyFmt, entry.AttackType))

	// Per-endpoint counter (no expiry — all-time totals)
	endpointKey := fmt.Sprintf(threatsEndpointKeyFmt, entry.Endpoint)
	pipe.Incr(ctx, endpointKey)

	_, err = pipe.Exec(ctx)
	return err
}

// GetThreats returns a paginated slice of threat logs and the total count.
func (r *RedisThreatStore) GetThreats(ctx context.Context, page, pageSize int) ([]ThreatLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	start := int64((page - 1) * pageSize)
	end := start + int64(pageSize) - 1

	total, err := r.client.LLen(ctx, threatsListKey).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count threats: %w", err)
	}

	raw, err := r.client.LRange(ctx, threatsListKey, start, end).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch threats: %w", err)
	}

	threats := make([]ThreatLog, 0, len(raw))
	for _, item := range raw {
		var t ThreatLog
		if json.Unmarshal([]byte(item), &t) == nil {
			threats = append(threats, t)
		}
	}
	return threats, total, nil
}

// GetThreatStats aggregates counters for the dashboard.
func (r *RedisThreatStore) GetThreatStats(ctx context.Context) (*ThreatStats, error) {
	stats := &ThreatStats{
		ByType:       make(map[string]int64),
		TopEndpoints: make(map[string]int64),
	}

	// Total blocked
	total, err := r.client.LLen(ctx, threatsListKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to count threats: %w", err)
	}
	stats.TotalBlocked = total

	// Today's count
	todayKey := fmt.Sprintf(threatsTodayKeyFmt, time.Now().UTC().Format("2006-01-02"))
	todayCount, _ := r.client.Get(ctx, todayKey).Int64()
	stats.TodayBlocked = todayCount

	// By attack type
	for _, t := range []string{"sqli", "xss", "oversized", "malformed"} {
		count, _ := r.client.Get(ctx, fmt.Sprintf(threatsTypeKeyFmt, t)).Int64()
		stats.ByType[t] = count
	}

	// Top endpoints (all-time)
	endpointKeys, _ := r.client.Keys(ctx, "analytics:threats:endpoint:*").Result()
	for _, key := range endpointKeys {
		endpoint := strings.TrimPrefix(key, "analytics:threats:endpoint:")
		count, _ := r.client.Get(ctx, key).Int64()
		stats.TopEndpoints[endpoint] = count
	}

	// Recent threats (last 10 for the live feed)
	raw, _ := r.client.LRange(ctx, threatsListKey, 0, 9).Result()
	for _, item := range raw {
		var t ThreatLog
		if json.Unmarshal([]byte(item), &t) == nil {
			stats.RecentThreats = append(stats.RecentThreats, t)
		}
	}

	return stats, nil
}
