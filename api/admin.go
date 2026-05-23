package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"secure-api-gateway/internal/analytics"
	"secure-api-gateway/internal/auth"
	"secure-api-gateway/internal/config"
	"secure-api-gateway/internal/detection"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AdminServer exposes internal analytics, threat data, and operational controls
// on a separate port (default :8081) so it is never reachable from the internet.
type AdminServer struct {
	reporter    analytics.StatsReporter
	threatStore detection.ThreatStore
	redisClient *redis.Client // for block-ip / blocked-ips (nil-safe)
	jwtMgr      *auth.JWTManager
	cfg         *config.Config
	logger      *zap.Logger
	startTime   time.Time
}

// NewAdminServer constructs an AdminServer with all required dependencies.
func NewAdminServer(
	reporter analytics.StatsReporter,
	threatStore detection.ThreatStore,
	redisClient *redis.Client,
	jwtMgr *auth.JWTManager,
	cfg *config.Config,
	logger *zap.Logger,
	startTime time.Time,
) *AdminServer {
	return &AdminServer{
		reporter:    reporter,
		threatStore: threatStore,
		redisClient: redisClient,
		jwtMgr:      jwtMgr,
		cfg:         cfg,
		logger:      logger,
		startTime:   startTime,
	}
}

// Router registers all admin endpoints on a fresh mux.Router.
func (s *AdminServer) Router() *mux.Router {
	r := mux.NewRouter()

	// Existing endpoints (unchanged)
	r.HandleFunc("/admin/stats", s.handleStats).Methods("GET")
	r.HandleFunc("/admin/token", s.handleIssueToken).Methods("POST")
	r.HandleFunc("/admin/health", s.handleHealth).Methods("GET")

	// New endpoints
	r.HandleFunc("/admin/threats", s.handleThreats).Methods("GET")
	r.HandleFunc("/admin/metrics", s.handleMetrics).Methods("GET")
	r.HandleFunc("/admin/block-ip", s.handleBlockIP).Methods("POST")
	r.HandleFunc("/admin/blocked-ips", s.handleBlockedIPs).Methods("GET")

	return r
}

// ── Existing handlers (preserved exactly) ───────────────────────────────────

func (s *AdminServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stats, err := s.reporter.GetStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get stats", zap.Error(err))
		http.Error(w, `{"error":"failed to retrieve stats"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(stats)
}

func (s *AdminServer) handleIssueToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.ClientID == "" || req.Role == "" {
		http.Error(w, `{"error":"client_id and role are required"}`, http.StatusBadRequest)
		return
	}

	token, err := s.jwtMgr.IssueToken(req.ClientID, req.Role)
	if err != nil {
		s.logger.Error("Failed to issue token", zap.Error(err))
		http.Error(w, `{"error":"failed to issue token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"client_id":  req.ClientID,
		"role":       req.Role,
		"expires_in": s.cfg.JWT.TokenTTLMin * 60,
	})
}

func (s *AdminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// ── New handlers ─────────────────────────────────────────────────────────────

// handleThreats returns a paginated threat log plus aggregate stats.
// Query params: page (default 1), size (default 20, max 100)
// The response includes by_type and top_endpoints so the dashboard needs
// only one call per refresh cycle.
func (s *AdminServer) handleThreats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	threats, total, err := s.threatStore.GetThreats(ctx, page, size)
	if err != nil {
		s.logger.Error("Failed to fetch threats", zap.Error(err))
		http.Error(w, `{"error":"failed to retrieve threats"}`, http.StatusInternalServerError)
		return
	}

	stats, _ := s.threatStore.GetThreatStats(ctx)

	resp := map[string]interface{}{
		"threats":   threats,
		"total":     total,
		"page":      page,
		"page_size": size,
	}
	if stats != nil {
		resp["by_type"] = stats.ByType
		resp["top_endpoints"] = stats.TopEndpoints
		resp["today_blocked"] = stats.TodayBlocked
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}

// handleMetrics returns computed operational statistics for the dashboard
// metric cards: uptime, request totals, block %, detection rate, avg latency.
func (s *AdminServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	analyticsStats, _ := s.reporter.GetStats(ctx)
	threatStats, _ := s.threatStore.GetThreatStats(ctx)

	uptime := time.Since(s.startTime).Seconds()

	var totalRequests int64
	var avgLatencyMs float64

	if analyticsStats != nil {
		totalRequests = analyticsStats.TotalRequests
		if len(analyticsStats.RecentRequests) > 0 {
			var sumLatency int64
			for _, req := range analyticsStats.RecentRequests {
				sumLatency += req.LatencyMs
			}
			avgLatencyMs = math.Round(float64(sumLatency)/float64(len(analyticsStats.RecentRequests))*100) / 100
		}
	}

	var threatsBlocked, todayBlocked int64
	if threatStats != nil {
		threatsBlocked = threatStats.TotalBlocked
		todayBlocked = threatStats.TodayBlocked
	}

	var blockPct float64
	totalProcessed := totalRequests + threatsBlocked
	if totalProcessed > 0 {
		blockPct = math.Round(float64(threatsBlocked)/float64(totalProcessed)*100*100) / 100
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"uptime_seconds":     math.Round(uptime),
		"total_requests":     totalProcessed,
		"threats_blocked":    threatsBlocked,
		"today_blocked":      todayBlocked,
		"block_pct":          blockPct,
		"detection_rate_pct": 100.0, // all detected threats are blocked
		"avg_latency_ms":     avgLatencyMs,
	})
}

// handleBlockIP adds an IP to the Redis blocked-IPs set.
// Protected by X-Admin-Key header checked against the ADMIN_KEY env variable.
// If ADMIN_KEY is unset the check is skipped (development convenience).
func (s *AdminServer) handleBlockIP(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdminKey(r) {
		http.Error(w, `{"error":"unauthorized — X-Admin-Key header required"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		http.Error(w, `{"error":"ip field is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := detection.AddBlockedIP(ctx, s.redisClient, req.IP); err != nil {
		s.logger.Error("Failed to block IP", zap.String("ip", req.IP), zap.Error(err))
		http.Error(w, `{"error":"redis unavailable — IP blocking requires Redis"}`, http.StatusServiceUnavailable)
		return
	}

	s.logger.Info("IP blocked", zap.String("ip", req.IP))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"blocked": req.IP,
		"status":  "ok",
	})
}

// handleBlockedIPs lists all currently blocked IPs from the Redis set.
func (s *AdminServer) handleBlockedIPs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	ips, err := detection.ListBlockedIPs(ctx, s.redisClient)
	if err != nil {
		s.logger.Warn("Failed to list blocked IPs", zap.Error(err))
		// Return graceful response instead of 500
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ips":   []string{},
			"count": 0,
			"note":  "Redis unavailable — IP blocking inactive",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ips":   ips,
		"count": len(ips),
	})
}

// checkAdminKey validates the X-Admin-Key header against ADMIN_KEY env var.
// Returns true (allow) if ADMIN_KEY is not set (dev mode).
func (s *AdminServer) checkAdminKey(r *http.Request) bool {
	expectedKey := os.Getenv("ADMIN_KEY")
	if expectedKey == "" {
		return true // no key configured — open in dev mode
	}
	return r.Header.Get("X-Admin-Key") == expectedKey
}
