package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	grpcapi "github.com/mtc/wiki-editor-backend/services/auth-service/api/grpc"
	restapi "github.com/mtc/wiki-editor-backend/services/auth-service/api/rest"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/adapters/security"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/usecase"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("auth-service")
	metrics := observability.NewMetrics("auth-service", nil)

	pool, err := transport.OpenPostgres(ctx, transport.PostgresConfig{
		URL: os.Getenv("AUTH_DATABASE_URL"),
	})
	if err != nil {
		logger.Error("failed to open postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repository := postgres.NewRepository(pool)
	passwordHasher := security.NewPasswordHasher(12)
	tokenService, err := security.NewTokenService(security.TokenServiceConfig{
		Issuer:        getenv("JWT_ISSUER", "wiki-auth"),
		Audience:      getenv("JWT_AUDIENCE", "wiki-api"),
		AccessTTL:     durationFromEnv("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:    durationFromEnv("JWT_REFRESH_TTL", 30*24*time.Hour),
		KeyID:         getenv("JWT_KEY_ID", "wiki-local-rsa"),
		PrivateKeyPEM: os.Getenv("JWT_PRIVATE_KEY_PEM"),
	})
	if err != nil {
		logger.Error("failed to initialize token service", "error", err)
		os.Exit(1)
	}

	authenticator := usecase.NewAuthenticator(repository, passwordHasher, tokenService)
	currentUser := usecase.NewCurrentUserReader(repository)
	authorizer := usecase.NewAuthorizer(repository)

	httpHandler := restapi.NewHandler(authenticator, currentUser, tokenService)
	grpcHandler := grpcapi.NewServer(authorizer)

	grpcServer := grpc.NewServer()
	authv1.RegisterAuthorizationServiceServer(grpcServer, grpcHandler)

	grpcListener, err := net.Listen("tcp", ":"+strconv.Itoa(intFromEnv("AUTH_SERVICE_GRPC_PORT", 9091)))
	if err != nil {
		logger.Error("failed to listen for grpc", "error", err)
		os.Exit(1)
	}

	grpcErrCh := make(chan error, 1)
	go func() {
		logger.Info("starting grpc server", "addr", grpcListener.Addr().String())
		grpcErrCh <- grpcServer.Serve(grpcListener)
	}()

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return transport.RunHTTPServer(groupCtx, transport.HTTPServerConfig{
			ServiceName: "auth-service",
			Port:        intFromEnv("AUTH_SERVICE_PORT", 8081),
			Logger:      logger,
			Metrics:     metrics,
			Readyz: func(ctx context.Context) error {
				err := repository.Ping(ctx)
				metrics.SetDependency("postgres", err == nil)
				return err
			},
			Routes: httpHandler.RegisterRoutes,
		})
	})

	group.Go(func() error {
		select {
		case err := <-grpcErrCh:
			if err == grpc.ErrServerStopped {
				return nil
			}
			return err
		case <-groupCtx.Done():
			return nil
		}
	})

	if err := group.Wait(); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func intFromEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
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
