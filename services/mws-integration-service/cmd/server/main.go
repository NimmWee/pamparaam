package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	mwsapp "github.com/mtc/wiki-editor-backend/services/mws-integration-service/app"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("mws-integration-service")
	resolverServer, err := mwsapp.NewResolverServer(mwsapp.Config{
		MWSBaseURL: getenvAny("http://localhost:8090", "MWS_BASE_URL", "MWS_MOCK_BASE_URL"),
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil {
		logger.Error("failed to initialize resolver server", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	mwsv1.RegisterMWSIntegrationServiceServer(grpcServer, resolverServer)

	grpcListener, err := net.Listen("tcp", ":"+strconv.Itoa(portFromEnv("MWS_INTEGRATION_SERVICE_GRPC_PORT", 9095)))
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
			ServiceName: "mws-integration-service",
			Port:        portFromEnv("MWS_INTEGRATION_SERVICE_PORT", 8085),
			Logger:      logger,
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
		logger.Error("mws integration service stopped", "error", err)
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
