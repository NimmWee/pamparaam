package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	collabapp "github.com/mtc/wiki-editor-backend/services/collaboration-service/app"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("collaboration-service")
	conn, err := grpc.Dial(getenvAny("127.0.0.1:9092", "PAGE_SERVICE_GRPC_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to dial page service", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	var authClient authv1.AuthorizationServiceClient
	authAddr := getenvAny("127.0.0.1:9091", "AUTH_SERVICE_GRPC_ADDR")
	if authAddr != "" {
		authConn, err := grpc.Dial(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to dial auth service", "error", err)
			os.Exit(1)
		}
		defer authConn.Close()
		authClient = authv1.NewAuthorizationServiceClient(authConn)
	}

	application, err := collabapp.NewApplication(collabapp.Config{
		PageServiceClient:   pagev1.NewPageRevisionServiceClient(conn),
		AuthorizationClient: authClient,
		RedisAddr:           getenvAny("", "COLLABORATION_REDIS_ADDR", "REDIS_ADDR"),
		RedisPassword:       getenvAny("", "COLLABORATION_REDIS_PASSWORD", "REDIS_PASSWORD"),
		RedisDB:             intFromEnvAny(0, "COLLABORATION_REDIS_DB", "REDIS_DB"),
		Logger:              logger,
		Now:                 time.Now,
	})
	if err != nil {
		logger.Error("failed to initialize collaboration service", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	if natsURL := getenvAny("", "COLLABORATION_NATS_URL", "NATS_URL"); natsURL != "" && application.RefreshConsumer != nil {
		natsConn, err := nats.Connect(natsURL)
		if err != nil {
			logger.Error("failed to connect to nats", "error", err)
			os.Exit(1)
		}
		defer natsConn.Close()
		if err := application.RefreshConsumer.Subscribe(ctx, natsConn); err != nil {
			logger.Error("failed to subscribe revision refresh consumer", "error", err)
			os.Exit(1)
		}
	}

	if err := transport.RunHTTPServer(ctx, transport.HTTPServerConfig{
		ServiceName: "collaboration-service",
		Port:        portFromEnv("COLLABORATION_SERVICE_PORT", 8083),
		Logger:      logger,
		Readyz:      application.Readyz,
		Routes: func(r chi.Router) {
			r.Mount("/", application.Handler)
		},
	}); err != nil {
		logger.Error("collaboration service stopped", "error", err)
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
