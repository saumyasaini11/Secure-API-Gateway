package detection

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	// maxBodySize is the maximum body we will read and scan.
	// Bodies larger than this are rejected as oversized.
	maxBodySize = 1 * 1024 * 1024 // 1 MB
)

// AttackDetectionMiddleware returns a middleware that scans incoming requests
// for SQLi, XSS, oversized payloads, and malformed Content-Type headers.
//
// Placement: AFTER IPBlocklistMiddleware, BEFORE JWT validation.
// This means it runs on every request including /auth/token, which is
// intentional — we want to catch SQLi/XSS in login payloads too.
//
// Body-restore contract: the middleware reads r.Body fully (up to maxBodySize+1
// bytes), then replaces r.Body with io.NopCloser(bytes.NewReader(bodyBytes))
// so downstream handlers (analytics logger, reverse proxy) can still read it.
func AttackDetectionMiddleware(store ThreatStore, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractRealIP(r)

			// ── 1. Read body (must happen before any scanning) ──────────────
			var bodyBytes []byte
			if r.Body != nil && r.Body != http.NoBody {
				// LimitReader stops at maxBodySize+1 so we can detect oversized bodies
				limited := io.LimitReader(r.Body, int64(maxBodySize)+1)
				bodyBytes, _ = io.ReadAll(limited)
				r.Body.Close()

				if len(bodyBytes) > maxBodySize {
					recordThreat(r.Context(), store, logger, ThreatLog{
						Timestamp:      time.Now().UTC(),
						IP:             ip,
						ClientID:       "unauthenticated",
						AttackType:     "oversized",
						Endpoint:       r.URL.Path,
						Method:         r.Method,
						Severity:       "medium",
						PayloadSnippet: fmt.Sprintf("Body exceeds limit: %d bytes read", len(bodyBytes)),
					})
					http.Error(w, `{"error":"payload too large"}`, http.StatusRequestEntityTooLarge)
					return
				}

				// Restore body for all downstream handlers (proxy, analytics, etc.)
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			// ── 2. Malformed Content-Type check ─────────────────────────────
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				ct := r.Header.Get("Content-Type")
				if ct == "" && len(bodyBytes) > 0 {
					recordThreat(r.Context(), store, logger, ThreatLog{
						Timestamp:      time.Now().UTC(),
						IP:             ip,
						ClientID:       "unauthenticated",
						AttackType:     "malformed",
						Endpoint:       r.URL.Path,
						Method:         r.Method,
						Severity:       "low",
						PayloadSnippet: "missing Content-Type header on non-empty body",
					})
					// Log only — do not block; some legitimate clients omit Content-Type
				}
			}

			// ── 3. Build combined scan target ────────────────────────────────
			// Scan URL path, query string, body, and high-risk headers together.
			targets := []string{
				r.URL.Path,
				r.URL.RawQuery,
				string(bodyBytes),
				r.Header.Get("Referer"),
				r.Header.Get("User-Agent"),
			}
			combined := strings.Join(targets, " ")

			// ── 4. SQLi check ────────────────────────────────────────────────
			if attackType, severity, snippet := detectSQLi(combined); attackType != "" {
				recordThreat(r.Context(), store, logger, ThreatLog{
					Timestamp:      time.Now().UTC(),
					IP:             ip,
					ClientID:       "unauthenticated",
					AttackType:     attackType,
					Endpoint:       r.URL.Path,
					Method:         r.Method,
					Severity:       severity,
					PayloadSnippet: snippet,
				})
				http.Error(w, `{"error":"request blocked"}`, http.StatusBadRequest)
				return
			}

			// ── 5. XSS check ─────────────────────────────────────────────────
			if attackType, severity, snippet := detectXSS(combined); attackType != "" {
				recordThreat(r.Context(), store, logger, ThreatLog{
					Timestamp:      time.Now().UTC(),
					IP:             ip,
					ClientID:       "unauthenticated",
					AttackType:     attackType,
					Endpoint:       r.URL.Path,
					Method:         r.Method,
					Severity:       severity,
					PayloadSnippet: snippet,
				})
				http.Error(w, `{"error":"request blocked"}`, http.StatusBadRequest)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// recordThreat logs to both the structured zap logger and the ThreatStore.
func recordThreat(ctx context.Context, store ThreatStore, logger *zap.Logger, entry ThreatLog) {
	logger.Warn("Threat detected",
		zap.String("attack_type", entry.AttackType),
		zap.String("ip", entry.IP),
		zap.String("endpoint", entry.Endpoint),
		zap.String("method", entry.Method),
		zap.String("severity", entry.Severity),
	)
	if store != nil {
		if err := store.LogThreat(ctx, entry); err != nil {
			logger.Error("Failed to persist threat log", zap.Error(err))
		}
	}
}
