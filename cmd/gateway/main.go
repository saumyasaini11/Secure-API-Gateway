package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"secure-api-gateway/api"
	"secure-api-gateway/internal/analytics"
	"secure-api-gateway/internal/auth"
	"secure-api-gateway/internal/config"
	"secure-api-gateway/internal/detection"
	"secure-api-gateway/internal/proxy"
	"secure-api-gateway/internal/ratelimit"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	startTime := time.Now()

	// ── Logger ────────────────────────────────────────────────
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	logger.Info("Starting Secure API Gateway")

	// ── Config ────────────────────────────────────────────────
	cfg := config.Load("config.yaml")
	logger.Info("Config loaded",
		zap.Int("routes", len(cfg.Routes)),
		zap.String("env", cfg.Server.Env),
	)

	// ── Redis (optional) ──────────────────────────────────────
	var limiter ratelimit.Limiter
	var analyticsLogger analytics.RequestLogger
	var reporter analytics.StatsReporter
	var threatStore detection.ThreatStore

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis unavailable – using in-memory fallbacks (rate limit + analytics)",
			zap.String("addr", cfg.Redis.Addr),
			zap.Error(err),
		)
		memStore := analytics.NewMemoryStore()
		analyticsLogger = memStore
		reporter = memStore
		limiter = ratelimit.NewMemoryLimiter()
		threatStore = detection.NewMemoryThreatStore()
	} else {
		logger.Info("Redis connected", zap.String("addr", cfg.Redis.Addr))
		analyticsLogger = analytics.NewLogger(redisClient)
		reporter = analytics.NewReporter(redisClient)
		limiter = ratelimit.NewRedisLimiter(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		threatStore = detection.NewRedisThreatStore(redisClient)
	}

	// ── JWT Manager ───────────────────────────────────────────
	jwtMgr, err := auth.NewJWTManager(
		cfg.JWT.PrivateKeyPath,
		cfg.JWT.PublicKeyPath,
		cfg.JWT.Issuer,
		cfg.JWT.TokenTTLMin,
	)
	if err != nil {
		logger.Fatal("Failed to initialize JWT manager", zap.Error(err))
	}
	logger.Info("JWT manager ready", zap.String("issuer", cfg.JWT.Issuer))

	// ── Rate Limiter ──────────────────────────────────────────
	rlMiddleware := ratelimit.NewMiddleware(limiter, cfg.Routes, cfg.Auth)

	// ── Gateway Proxy ─────────────────────────────────────────
	gateway := proxy.NewGateway(cfg.Routes, logger)

	// ── Router ────────────────────────────────────────────────
	router := mux.NewRouter()
	// Security middleware — runs on ALL routes before JWT validation
	router.Use(detection.IPBlocklistMiddleware(redisClient, logger))
	router.Use(detection.AttackDetectionMiddleware(threatStore, logger))

	// Public auth endpoint (rate limited)
	router.Handle(cfg.Auth.TokenEndpoint, rlMiddleware.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientID string `json:"client_id"`
			Secret   string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientID == "" || req.Secret == "" {
			http.Error(w, `{"error":"client_id and secret required"}`, http.StatusBadRequest)
			return
		}
		token, err := jwtMgr.IssueToken(req.ClientID, "client")
		if err != nil {
			http.Error(w, `{"error":"token issuance failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"token":"%s","expires_in":%d}`, token, cfg.JWT.TokenTTLMin*60)
	}))).Methods("POST")

	// Protected API routes
	protected := router.PathPrefix("/api").Subrouter()
	protected.Use(jwtMgr.Middleware)
	protected.Use(rlMiddleware.Limit)
	protected.Use(analyticsMiddleware(analyticsLogger))
	protected.PathPrefix("/").HandlerFunc(gateway.Handler())

	// ── Admin Server ──────────────────────────────────────────
	adminServer := api.NewAdminServer(reporter, threatStore, redisClient, jwtMgr, cfg, logger, startTime)

	// ── HTTP Servers ──────────────────────────────────────────
	mainSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	adminSrv := &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", cfg.Server.AdminPort),
		Handler:      adminServer.Router(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Gateway listening", zap.Int("port", cfg.Server.Port))
		if err := mainSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Gateway error", zap.Error(err))
		}
	}()

	go func() {
		logger.Info("Admin server listening", zap.Int("port", cfg.Server.AdminPort))
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Admin error", zap.Error(err))
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mainSrv.Shutdown(shutCtx)
	adminSrv.Shutdown(shutCtx)
	logger.Info("Gateway stopped cleanly")
}

// analyticsMiddleware logs every request passing through the gateway
func analyticsMiddleware(al analytics.RequestLogger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := analytics.NewWrappedResponseWriter(w)
			next.ServeHTTP(wrapped, r)

			clientID := "anonymous"
			if claims := auth.GetClaims(r); claims != nil {
				clientID = claims.ClientID
			}

			al.Log(context.Background(), analytics.RequestLog{
				ClientID:   clientID,
				Route:      r.URL.Path,
				Method:     r.Method,
				StatusCode: wrapped.StatusCode,
				LatencyMs:  time.Since(start).Milliseconds(),
				Timestamp:  time.Now().UTC(),
				IP:         r.RemoteAddr,
			})
		})
	}
}
