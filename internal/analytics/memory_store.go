package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MemoryStore is a thread-safe in-memory analytics store.
// It replaces Redis-based Logger and Reporter when Redis is unavailable.
type MemoryStore struct {
	mu       sync.RWMutex
	requests []RequestLog // ring buffer, max 1000

	clientCount map[string]int64
	routeCount  map[string]int64
	errorCount  map[string]int64
	bruteForce  map[string]int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		clientCount: make(map[string]int64),
		routeCount:  make(map[string]int64),
		errorCount:  make(map[string]int64),
		bruteForce:  make(map[string]int64),
	}
}

// Log implements the same signature as Logger.Log so it can be used interchangeably.
func (m *MemoryStore) Log(_ context.Context, entry RequestLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prepend, keep latest 1000
	m.requests = append([]RequestLog{entry}, m.requests...)
	if len(m.requests) > 1000 {
		m.requests = m.requests[:1000]
	}

	m.clientCount[entry.ClientID]++
	m.routeCount[entry.Route]++
	if entry.StatusCode >= 400 {
		m.errorCount[entry.ClientID]++
		if entry.StatusCode == 401 {
			m.bruteForce[entry.ClientID]++
		}
	}
	return nil
}

// GetStats implements the same signature as Reporter.GetStats.
func (m *MemoryStore) GetStats(_ context.Context) (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recent := m.requests
	if len(recent) > 50 {
		recent = recent[:50]
	}

	// deep-copy maps
	cc := make(map[string]int64, len(m.clientCount))
	for k, v := range m.clientCount {
		cc[k] = v
	}
	rc := make(map[string]int64, len(m.routeCount))
	for k, v := range m.routeCount {
		rc[k] = v
	}
	ec := make(map[string]int64, len(m.errorCount))
	for k, v := range m.errorCount {
		ec[k] = v
	}
	bf := make(map[string]int64)
	for k, v := range m.bruteForce {
		if v >= 5 {
			bf[k] = v
		}
	}

	return &Stats{
		TotalRequests:   int64(len(m.requests)),
		ClientCounts:    cc,
		RouteCounts:     rc,
		ErrorCounts:     ec,
		BruteForceFlags: bf,
		RecentRequests:  recent,
	}, nil
}

// ── Compile-time helpers so main.go can use a single interface ──────────────

// RequestLogger is satisfied by both *Logger and *MemoryStore.
type RequestLogger interface {
	Log(ctx context.Context, entry RequestLog) error
}

// StatsReporter is satisfied by both *Reporter and *MemoryStore.
type StatsReporter interface {
	GetStats(ctx context.Context) (*Stats, error)
}

// ── JSON round-trip helpers (used by dashboard SSE) ─────────────────────────

func MarshalStats(s *Stats) ([]byte, error) {
	return json.Marshal(s)
}

// ── Unused import guard ──────────────────────────────────────────────────────
var _ = fmt.Sprintf
var _ = time.Second
