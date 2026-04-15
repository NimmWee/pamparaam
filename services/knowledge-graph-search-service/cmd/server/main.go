package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	searchapp "github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("knowledge-graph-search-service")
	var authClient authv1.AuthorizationServiceClient
	authAddr := getenvAny("127.0.0.1:9091", "AUTH_SERVICE_GRPC_ADDR")
	if authAddr != "" {
		conn, err := grpc.Dial(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to dial auth service", "error", err)
			os.Exit(1)
		}
		defer conn.Close()
		authClient = authv1.NewAuthorizationServiceClient(conn)
	}

	application, err := searchapp.NewApplication(searchapp.Config{
		AuthorizationClient: authClient,
		DatabaseURL:         os.Getenv("SEARCH_DATABASE_URL"),
		Logger:              logger,
	})
	if err != nil {
		logger.Error("failed to initialize search service", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	if natsURL := getenvAny("", "SEARCH_NATS_URL", "NATS_URL"); natsURL != "" && application.Consumer != nil {
		natsConn, err := messaging.Connect(ctx, messaging.NATSConfig{
			URL:  natsURL,
			Name: "knowledge-graph-search-service",
		})
		if err != nil {
			logger.Error("failed to connect to nats", "error", err)
			os.Exit(1)
		}
		if err := application.Consumer.Subscribe(ctx, natsConn); err != nil {
			logger.Error("failed to subscribe page event consumer", "error", err)
			os.Exit(1)
		}
	}

	if err := transport.RunHTTPServer(ctx, transport.HTTPServerConfig{
		ServiceName: "knowledge-graph-search-service",
		Port:        portFromEnv("SEARCH_SERVICE_PORT", 8084),
		Logger:      logger,
		Readyz:      application.Readyz,
		Routes: func(r chi.Router) {
			r.Mount("/", application.Handler)
		},
	}); err != nil {
		logger.Error("search service stopped", "error", err)
		os.Exit(1)
	}
}

func portFromEnv(key string, fallback int) int {
	return intFromEnvAny(fallback, key)
}

func intFromEnvAny(fallback int, keys ...string) int {
	for _, key := range keys {
		raw := os.Getenv(key)
		if raw == "" {
			continue
		}

		port, err := strconv.Atoi(raw)
		if err == nil {
			return port
		}
	}
	return fallback
}

func getenvAny(fallback string, keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return fallback
}

func getenv(key, fallback string) string {
	raw := os.Getenv(key)
	if raw != "" {
		return raw
	}
	return fallback
}
