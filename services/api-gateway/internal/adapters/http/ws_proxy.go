package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/mtc/wiki-editor-backend/pkg/transport"
)

func (g *Gateway) wsProxyTo(baseURL string) http.HandlerFunc {
	target, err := url.Parse(baseURL)
	if err != nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "invalid upstream configuration", http.StatusInternalServerError)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		req.URL.Path = joinPath(target.Path, req.URL.Path)
		req.Host = target.Host
		applyIdentityHeaders(req)
		req.Header.Set("X-Request-Id", transport.RequestIDFromContext(req.Context()))
		req.Header.Set("X-Correlation-Id", transport.CorrelationIDFromContext(req.Context()))
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		g.logger.Error("websocket proxy failed", "upstream", baseURL, "path", r.URL.Path, "error", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	return proxy.ServeHTTP
}
