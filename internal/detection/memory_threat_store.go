package detection

import (
	"context"
	"sync"
	"time"
)

// MemoryThreatStore is a thread-safe in-memory ThreatStore used when Redis
// is unavailable. It mirrors the analytics.MemoryStore pattern exactly.
type MemoryThreatStore struct {
	mu         sync.RWMutex
	threats    []ThreatLog      // ring buffer, max 200 entries
	byType     map[string]int64 // attack type → count
	byEndpoint map[string]int64 // endpoint path → count
	todayCount int64
	todayDate  string // "2006-01-02" in UTC
}

// NewMemoryThreatStore creates a ready-to-use in-memory ThreatStore.
func NewMemoryThreatStore() *MemoryThreatStore {
	return &MemoryThreatStore{
		byType:     make(map[string]int64),
		byEndpoint: make(map[string]int64),
		todayDate:  time.Now().UTC().Format("2006-01-02"),
	}
}

// LogThreat prepends an entry to the ring buffer and updates counters.
func (m *MemoryThreatStore) LogThreat(_ context.Context, entry ThreatLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prepend — newest first (same as Redis LPush)
	m.threats = append([]ThreatLog{entry}, m.threats...)
	if len(m.threats) > 200 {
		m.threats = m.threats[:200]
	}

	m.byType[entry.AttackType]++
	m.byEndpoint[entry.Endpoint]++

	// Reset today's counter at UTC midnight
	today := time.Now().UTC().Format("2006-01-02")
	if today != m.todayDate {
		m.todayCount = 0
		m.todayDate = today
	}
	m.todayCount++

	return nil
}

// GetThreats returns a paginated slice from the in-memory ring buffer.
func (m *MemoryThreatStore) GetThreats(_ context.Context, page, pageSize int) ([]ThreatLog, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	total := int64(len(m.threats))
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= len(m.threats) {
		return []ThreatLog{}, total, nil
	}
	if end > len(m.threats) {
		end = len(m.threats)
	}

	result := make([]ThreatLog, end-start)
	copy(result, m.threats[start:end])
	return result, total, nil
}

// GetThreatStats returns aggregated stats from the in-memory store.
func (m *MemoryThreatStore) GetThreatStats(_ context.Context) (*ThreatStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep-copy maps to avoid data races on the returned value
	byType := make(map[string]int64, len(m.byType))
	for k, v := range m.byType {
		byType[k] = v
	}
	byEndpoint := make(map[string]int64, len(m.byEndpoint))
	for k, v := range m.byEndpoint {
		byEndpoint[k] = v
	}

	recent := m.threats
	if len(recent) > 10 {
		recent = recent[:10]
	}
	recentCopy := make([]ThreatLog, len(recent))
	copy(recentCopy, recent)

	return &ThreatStats{
		TotalBlocked:  int64(len(m.threats)),
		TodayBlocked:  m.todayCount,
		ByType:        byType,
		TopEndpoints:  byEndpoint,
		RecentThreats: recentCopy,
	}, nil
}
