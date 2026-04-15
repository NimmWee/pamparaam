package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	filev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/filev1"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	fileapp "github.com/mtc/wiki-editor-backend/services/file-service/app"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("file-service")
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
	application, err := fileapp.NewApplication(fileapp.Config{
		AuthorizationClient: authClient,
		DatabaseURL:         os.Getenv("FILE_DATABASE_URL"),
		MinIOEndpoint:       getenvAny("", "FILE_MINIO_ENDPOINT", "MINIO_ENDPOINT"),
		MinIOPublicBaseURL:  getenvAny("", "FILE_MINIO_PUBLIC_BASE_URL"),
		MinIOAccessKeyID:    getenvAny("", "FILE_MINIO_ACCESS_KEY_ID", "MINIO_ROOT_USER"),
		MinIOSecretKey:      getenvAny("", "FILE_MINIO_SECRET_ACCESS_KEY", "MINIO_ROOT_PASSWORD"),
		MinIOBucket:         getenvAny("wiki-files", "FILE_MINIO_BUCKET", "MINIO_BUCKET_ATTACHMENTS"),
		MinIOUseSSL:         strings.EqualFold(getenvAny("", "FILE_MINIO_USE_SSL", "MINIO_USE_SSL"), "true"),
		Logger:              logger,
		Now:                 time.Now,
	})
	if err != nil {
		logger.Error("failed to initialize file service", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	grpcServer := grpc.NewServer()
	filev1.RegisterFileMetadataServiceServer(grpcServer, application.GRPC)
	grpcListener, err := net.Listen("tcp", ":"+strconv.Itoa(intFromEnvAny(9096, "FILE_SERVICE_GRPC_PORT")))
	if err != nil {
		logger.Error("failed to listen for grpc", "error", err)
		os.Exit(1)
	}
	grpcErrCh := make(chan error, 1)
	go func() { grpcErrCh <- grpcServer.Serve(grpcListener) }()
	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return transport.RunHTTPServer(groupCtx, transport.HTTPServerConfig{
			ServiceName: "file-service",
			Port:        portFromEnv("FILE_SERVICE_PORT", 8086),
			Logger:      logger,
			Readyz:      application.Readyz,
			Routes: func(r chi.Router) {
				r.Mount("/", application.Handler)
			},
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
		logger.Error("file service stopped", "error", err)
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
