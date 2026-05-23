package detection

import (
	"context"
	"time"
)

// ThreatLog represents one blocked or flagged request event.
type ThreatLog struct {
	Timestamp      time.Time `json:"timestamp"`
	IP             string    `json:"ip"`
	ClientID       string    `json:"client_id"`      // always "unauthenticated" for pre-JWT requests
	AttackType     string    `json:"attack_type"`    // "sqli" | "xss" | "oversized" | "malformed"
	Endpoint       string    `json:"endpoint"`
	Method         string    `json:"method"`
	Severity       string    `json:"severity"`       // "high" | "medium" | "low"
	PayloadSnippet string    `json:"payload_snippet"`
}

// ThreatStats holds aggregated threat data for dashboard consumption.
type ThreatStats struct {
	TotalBlocked  int64            `json:"total_blocked"`
	TodayBlocked  int64            `json:"today_blocked"`
	ByType        map[string]int64 `json:"by_type"`
	TopEndpoints  map[string]int64 `json:"top_endpoints"`
	RecentThreats []ThreatLog      `json:"recent_threats"`
}

// ThreatStore is the interface satisfied by both RedisThreatStore and MemoryThreatStore.
// It mirrors the analytics.RequestLogger / StatsReporter dual-implementation pattern.
type ThreatStore interface {
	LogThreat(ctx context.Context, entry ThreatLog) error
	GetThreats(ctx context.Context, page, pageSize int) ([]ThreatLog, int64, error)
	GetThreatStats(ctx context.Context) (*ThreatStats, error)
}
