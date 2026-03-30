package ratelimit

import (
    "context"
    "fmt"
    "net/http"
    "strconv"

    "secure-api-gateway/internal/auth"
    "secure-api-gateway/internal/config"
)

// Limiter is the common interface satisfied by RedisLimiter and MemoryLimiter.
type Limiter interface {
    Allow(ctx context.Context, clientID, route string, maxRequests, windowSeconds int) (bool, int, error)
    Ping(ctx context.Context) error
}

type Middleware struct {
    limiter Limiter
    routes  []config.Route
    auth    config.AuthConfig
}

func NewMiddleware(limiter Limiter, routes []config.Route, authCfg config.AuthConfig) *Middleware {
    return &Middleware{limiter: limiter, routes: routes, auth: authCfg}
}

func (m *Middleware) Limit(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := auth.GetClaims(r)
        clientID := r.RemoteAddr
        if claims != nil {
            clientID = claims.ClientID
        }
        maxRequests := 60
        windowSeconds := 60
        if r.URL.Path == m.auth.TokenEndpoint {
            maxRequests = m.auth.RateLimit.Requests
            windowSeconds = m.auth.RateLimit.WindowSeconds
        } else {
            for _, route := range m.routes {
                if r.URL.Path == route.Path {
                    maxRequests = route.RateLimit.Requests
                    windowSeconds = route.RateLimit.WindowSeconds
                    break
                }
            }
        }
        allowed, remaining, err := m.limiter.Allow(
            context.Background(),
            clientID,
            r.URL.Path,
            maxRequests,
            windowSeconds,
        )
        if err != nil {
            http.Error(w, `{"error":"rate limiter unavailable"}`, http.StatusServiceUnavailable)
            return
        }
        w.Header().Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
        w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
        w.Header().Set("X-RateLimit-Window", fmt.Sprintf("%ds", windowSeconds))
        if !allowed {
            w.Header().Set("Retry-After", strconv.Itoa(windowSeconds))
            http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
