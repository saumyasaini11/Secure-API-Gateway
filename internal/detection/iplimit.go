package detection

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const blockedIPsKey = "gateway:blocked_ips"

// IPBlocklistMiddleware returns a middleware that rejects requests from IPs
// listed in the gateway:blocked_ips Redis Set.
//
// It is designed to be the FIRST middleware in the chain (wired on the root
// router, before JWT validation and attack detection).
//
// Fail-open: if Redis is unreachable the request is allowed through and a
// warning is logged, so a Redis outage does not take down the gateway.
func IPBlocklistMiddleware(redisClient *redis.Client, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractRealIP(r)

			if redisClient != nil {
				blocked, err := redisClient.SIsMember(
					context.Background(), blockedIPsKey, ip,
				).Result()
				if err != nil {
					// Fail open — a Redis error must not block legitimate traffic
					logger.Warn("IP blocklist check failed (fail-open)",
						zap.String("ip", ip),
						zap.Error(err),
					)
				} else if blocked {
					logger.Info("Blocked IP rejected",
						zap.String("ip", ip),
						zap.String("path", r.URL.Path),
					)
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AddBlockedIP adds an IP address to the Redis blocked-IPs set.
func AddBlockedIP(ctx context.Context, redisClient *redis.Client, ip string) error {
	return redisClient.SAdd(ctx, blockedIPsKey, ip).Err()
}

// ListBlockedIPs returns all IPs currently in the Redis blocked-IPs set.
func ListBlockedIPs(ctx context.Context, redisClient *redis.Client) ([]string, error) {
	return redisClient.SMembers(ctx, blockedIPsKey).Result()
}

// ExtractRealIP determines the originating client IP by inspecting forwarded
// headers before falling back to the TCP remote address.
// Exported so that the detection middleware can reuse the same logic.
func ExtractRealIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be a comma-separated list; the leftmost is the originating client
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
