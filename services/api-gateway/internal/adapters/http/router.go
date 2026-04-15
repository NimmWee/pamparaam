package http

import (
	"errors"
	"log/slog"
	nethttp "net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/authz"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
)

var ErrDependencyUnavailable = errors.New("gateway dependency unavailable")

type Config struct {
	AuthServiceBaseURL             string
	AuthServiceReadyURL            string
	AuthServiceJWKSURL             string
	PageServiceBaseURL             string
	CollaborationServiceBaseURL    string
	KnowledgeGraphSearchServiceURL string
	MWSIntegrationServiceBaseURL   string
	FileServiceBaseURL             string
	OpenAPIPath                    string
	JWTIssuer                      string
	JWTAudience                    string
	JWKSCacheTTL                   time.Duration
	AccessTokenValidator           AccessTokenValidator
	AuthorizationClient            authv1.AuthorizationServiceClient
	HTTPClient                     *nethttp.Client
	Logger                         *slog.Logger
}

type Gateway struct {
	cfg         Config
	logger      *slog.Logger
	httpClient  *nethttp.Client
	auth        *AuthMiddleware
	openAPISpec []byte
}

func NewGateway(cfg Config) (*Gateway, error) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &nethttp.Client{Timeout: 5 * time.Second}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.JWKSCacheTTL <= 0 {
		cfg.JWKSCacheTTL = 5 * time.Minute
	}

	spec, err := os.ReadFile(cfg.OpenAPIPath)
	if err != nil {
		return nil, err
	}

	validator := cfg.AccessTokenValidator
	if validator == nil {
		validator = NewJWTValidator(cfg.AuthServiceJWKSURL, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWKSCacheTTL, cfg.HTTPClient)
	}

	return &Gateway{
		cfg:         cfg,
		logger:      cfg.Logger,
		httpClient:  cfg.HTTPClient,
		auth:        NewAuthMiddleware(validator, runtimeauthz.NewClient(cfg.AuthorizationClient)),
		openAPISpec: spec,
	}, nil
}

func (g *Gateway) RegisterRoutes(router chi.Router) {
	router.Get("/openapi.yaml", g.handleOpenAPI)
	router.Get("/openapi/public-api.yaml", g.handleOpenAPI)
	router.Get("/.well-known/jwks.json", g.proxyTo(g.cfg.AuthServiceBaseURL, "", false))
	router.Group(func(ws chi.Router) {
		ws.Use(g.auth.RequireAuth)
		ws.Get("/ws/collab", g.wsProxyTo(g.cfg.CollaborationServiceBaseURL))
	})

	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/.well-known/jwks.json", g.proxyTo(g.cfg.AuthServiceBaseURL, "/api/v1", false))
		api.Post("/auth/login", g.proxyTo(g.cfg.AuthServiceBaseURL, "/api/v1", false))
		api.Post("/auth/refresh", g.proxyTo(g.cfg.AuthServiceBaseURL, "/api/v1", false))

		api.Group(func(protected chi.Router) {
			protected.Use(g.auth.RequireAuth)

			protected.Get("/auth/me", g.proxyTo(g.cfg.AuthServiceBaseURL, "/api/v1", true))

			protected.With(g.auth.RequireAction(authz.ActionPageEdit)).Post("/pages", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageView)).Get("/pages/{pageID}", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageArchive)).Post("/pages/{pageID}/archive", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageEdit)).Patch("/pages/{pageID}/draft", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageView)).Get("/pages/{pageID}/draft/recover", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPagePublish)).Post("/pages/{pageID}/publish", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageView)).Get("/pages/{pageID}/versions", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageRestore)).Post("/pages/{pageID}/versions/{revisionID}/restore", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageView)).Get("/pages/{pageID}/backlinks", g.proxyTo(g.cfg.KnowledgeGraphSearchServiceURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageView)).Get("/editor/metadata", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionPageView)).Post("/editor/sync", g.proxyTo(g.cfg.PageServiceBaseURL, "/api/v1", true))

			protected.With(g.auth.RequireAction(authz.ActionSearchQuery)).Handle("/search", g.proxyTo(g.cfg.KnowledgeGraphSearchServiceURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionSearchQuery)).Handle("/search/*", g.proxyTo(g.cfg.KnowledgeGraphSearchServiceURL, "/api/v1", true))

			protected.With(g.auth.RequireAction(authz.ActionFileUpload)).Post("/files/uploads", g.proxyTo(g.cfg.FileServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionFileUpload)).Post("/files/uploads/{uploadID}/complete", g.proxyTo(g.cfg.FileServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionFileRead)).Get("/files/{fileID}", g.proxyTo(g.cfg.FileServiceBaseURL, "/api/v1", true))
			protected.With(g.auth.RequireAction(authz.ActionFileUpload)).Delete("/files/{fileID}", g.proxyTo(g.cfg.FileServiceBaseURL, "/api/v1", true))

			protected.Handle("/mws", g.proxyTo(g.cfg.MWSIntegrationServiceBaseURL, "/api/v1", true))
			protected.Handle("/mws/*", g.proxyTo(g.cfg.MWSIntegrationServiceBaseURL, "/api/v1", true))
			protected.Handle("/collaboration", g.proxyTo(g.cfg.CollaborationServiceBaseURL, "/api/v1", true))
			protected.Handle("/collaboration/*", g.proxyTo(g.cfg.CollaborationServiceBaseURL, "/api/v1", true))
		})
	})
}

func (g *Gateway) AuthServiceReadyURL() string {
	return g.cfg.AuthServiceReadyURL
}

func (g *Gateway) HTTPClient() *nethttp.Client {
	return g.httpClient
}

func (g *Gateway) handleOpenAPI(w nethttp.ResponseWriter, _ *nethttp.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(nethttp.StatusOK)
	_, _ = w.Write(g.openAPISpec)
}

func (g *Gateway) proxyTo(baseURL string, stripPrefix string, enrichAuthHeaders bool) nethttp.HandlerFunc {
	target, err := url.Parse(baseURL)
	if err != nil {
		return func(w nethttp.ResponseWriter, _ *nethttp.Request) {
			nethttp.Error(w, "invalid upstream configuration", nethttp.StatusInternalServerError)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(req *nethttp.Request) {
		director(req)
		req.URL.Path = joinPath(target.Path, strings.TrimPrefix(req.URL.Path, stripPrefix))
		req.Host = target.Host
		if enrichAuthHeaders {
			applyIdentityHeaders(req)
		}
	}
	proxy.ErrorHandler = func(w nethttp.ResponseWriter, r *nethttp.Request, err error) {
		g.logger.Error("upstream proxy failed", "upstream", baseURL, "path", r.URL.Path, "error", err)
		nethttp.Error(w, "upstream unavailable", nethttp.StatusBadGateway)
	}

	return proxy.ServeHTTP
}

func joinPath(basePath string, requestPath string) string {
	cleanBase := strings.TrimSuffix(basePath, "/")
	cleanRequest := strings.TrimPrefix(requestPath, "/")
	if cleanBase == "" {
		return "/" + cleanRequest
	}
	if cleanRequest == "" {
		return cleanBase
	}
	return cleanBase + "/" + cleanRequest
}
