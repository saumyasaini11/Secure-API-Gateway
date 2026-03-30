package api

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "secure-api-gateway/internal/analytics"
    "secure-api-gateway/internal/auth"
    "secure-api-gateway/internal/config"

    "github.com/gorilla/mux"
    "go.uber.org/zap"
)

type AdminServer struct {
    reporter analytics.StatsReporter
    jwtMgr   *auth.JWTManager
    cfg      *config.Config
    logger   *zap.Logger
}

func NewAdminServer(
    reporter analytics.StatsReporter,
    jwtMgr *auth.JWTManager,
    cfg *config.Config,
    logger *zap.Logger,
) *AdminServer {
    return &AdminServer{
        reporter: reporter,
        jwtMgr:   jwtMgr,
        cfg:      cfg,
        logger:   logger,
    }
}

func (s *AdminServer) Router() *mux.Router {
    r := mux.NewRouter()
    r.HandleFunc("/admin/stats", s.handleStats).Methods("GET")
    r.HandleFunc("/admin/token", s.handleIssueToken).Methods("POST")
    r.HandleFunc("/admin/health", s.handleHealth).Methods("GET")
    return r
}

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
