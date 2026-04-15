package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	httpadapter "github.com/mtc/wiki-editor-backend/services/api-gateway/internal/adapters/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("api-gateway")
	metrics := observability.NewMetrics("api-gateway", nil)
	var authClient authv1.AuthorizationServiceClient
	authAddr := getenv("AUTH_SERVICE_GRPC_ADDR", "127.0.0.1:9091")
	if authAddr != "" {
		conn, err := grpc.Dial(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to dial auth service", "error", err)
			os.Exit(1)
		}
		defer conn.Close()
		authClient = authv1.NewAuthorizationServiceClient(conn)
	}

	gateway, err := httpadapter.NewGateway(httpadapter.Config{
		AuthServiceBaseURL:             getenv("AUTH_SERVICE_BASE_URL", "http://localhost:8081"),
		AuthServiceReadyURL:            getenv("AUTH_SERVICE_READY_URL", "http://localhost:8081/health/ready"),
		AuthServiceJWKSURL:             getenv("AUTH_SERVICE_JWKS_URL", "http://localhost:8081/.well-known/jwks.json"),
		PageServiceBaseURL:             getenv("PAGE_SERVICE_BASE_URL", "http://localhost:8082"),
		CollaborationServiceBaseURL:    getenv("COLLABORATION_SERVICE_BASE_URL", "http://localhost:8083"),
		KnowledgeGraphSearchServiceURL: getenv("SEARCH_SERVICE_BASE_URL", "http://localhost:8084"),
		MWSIntegrationServiceBaseURL:   getenv("MWS_INTEGRATION_SERVICE_BASE_URL", "http://localhost:8085"),
		FileServiceBaseURL:             getenv("FILE_SERVICE_BASE_URL", "http://localhost:8086"),
		OpenAPIPath:                    getenv("GATEWAY_OPENAPI_PATH", "specs/001-wiki-editor-backend/contracts/public-api.openapi.yaml"),
		JWTIssuer:                      getenv("JWT_ISSUER", "wiki-auth"),
		JWTAudience:                    getenv("JWT_AUDIENCE", "wiki-api"),
		JWKSCacheTTL:                   durationFromEnv("AUTH_SERVICE_JWKS_CACHE_TTL", 5*time.Minute),
		AuthorizationClient:            authClient,
		HTTPClient:                     &http.Client{Timeout: 5 * time.Second},
		Logger:                         logger,
	})
	if err != nil {
		logger.Error("failed to initialize gateway", "error", err)
		os.Exit(1)
	}

	if err := transport.RunHTTPServer(ctx, transport.HTTPServerConfig{
		ServiceName: "api-gateway",
		Port:        portFromEnv("GATEWAY_PORT", 8080),
		Logger:      logger,
		Metrics:     metrics,
		Readyz: func(ctx context.Context) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, gateway.AuthServiceReadyURL(), nil)
			if err != nil {
				metrics.SetDependency("auth-service", false)
				return err
			}

			resp, err := gateway.HTTPClient().Do(req)
			if err != nil {
				metrics.SetDependency("auth-service", false)
				return err
			}
			defer resp.Body.Close()

			healthy := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
			metrics.SetDependency("auth-service", healthy)
			if !healthy {
				return httpadapter.ErrDependencyUnavailable
			}
			return nil
		},
		Routes: gateway.RegisterRoutes,
	}); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func portFromEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return port
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
