package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"secure-api-gateway/internal/auth"
	"secure-api-gateway/internal/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Gateway struct {
	routes []config.Route
	logger *zap.Logger
}

func NewGateway(routes []config.Route, logger *zap.Logger) *Gateway {
	return &Gateway{routes: routes, logger: logger}
}

func (g *Gateway) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var matchedRoute *config.Route
		for i, route := range g.routes {
			if r.URL.Path == route.Path {
				matchedRoute = &g.routes[i]
				break
			}
		}

		if matchedRoute == nil {
			http.Error(w, `{"error":"route not found"}`, http.StatusNotFound)
			return
		}

		backendURL, err := url.Parse(matchedRoute.Backend)
		if err != nil {
			http.Error(w, `{"error":"invalid backend configuration"}`, http.StatusInternalServerError)
			return
		}

		requestID := uuid.NewString()

		clientID := "anonymous"
		if claims := auth.GetClaims(r); claims != nil {
			clientID = claims.ClientID
		}

		r.Header.Del("X-Gateway-Client-ID")
		r.Header.Del("X-Request-ID")
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Real-IP")

		r.Header.Set("X-Gateway-Client-ID", clientID)
		r.Header.Set("X-Request-ID", requestID)
		r.Header.Set("X-Forwarded-By", "secure-api-gateway")

		proxy := httputil.NewSingleHostReverseProxy(backendURL)

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			g.logger.Error("Backend unreachable",
				zap.String("backend", matchedRoute.Backend),
				zap.String("request_id", requestID),
				zap.Error(err),
			)
			http.Error(w, `{"error":"backend service unavailable"}`, http.StatusBadGateway)
		}

		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("X-Powered-By")
			resp.Header.Del("Server")
			resp.Header.Del("X-AspNet-Version")
			return nil
		}

		g.logger.Info("Proxying request",
			zap.String("client_id", clientID),
			zap.String("path", r.URL.Path),
			zap.String("backend", matchedRoute.Backend),
			zap.String("request_id", requestID),
		)

		proxy.ServeHTTP(w, r)
	}
}