package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryLimiter is a thread-safe in-memory sliding-window rate limiter.
// It satisfies the same Allow/Ping contract as RedisLimiter so it can be
// used as a drop-in fallback when Redis is unavailable.
type MemoryLimiter struct {
	mu      sync.Mutex
	windows map[string][]int64 // key → sorted slice of unix-ms timestamps
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		windows: make(map[string][]int64),
	}
}

func (m *MemoryLimiter) Allow(_ context.Context, clientID, route string, maxRequests, windowSeconds int) (bool, int, error) {
	key := fmt.Sprintf("ratelimit:%s:%s", clientID, route)
	now := time.Now().UnixMilli()
	cutoff := now - int64(windowSeconds)*1000

	m.mu.Lock()
	defer m.mu.Unlock()

	// Evict old timestamps
	ts := m.windows[key]
	start := 0
	for start < len(ts) && ts[start] < cutoff {
		start++
	}
	ts = ts[start:]

	count := len(ts)
	remaining := maxRequests - count - 1

	if count >= maxRequests {
		m.windows[key] = ts
		return false, 0, nil
	}

	m.windows[key] = append(ts, now)
	return true, remaining, nil
}

func (m *MemoryLimiter) Ping(_ context.Context) error {
	return nil // always healthy
}
