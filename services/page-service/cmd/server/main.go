package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	filev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/filev1"
	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	pageapp "github.com/mtc/wiki-editor-backend/services/page-service/app"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := observability.NewLogger("page-service")
	var mwsClient mwsv1.MWSIntegrationServiceClient
	var authClient authv1.AuthorizationServiceClient
	var fileClient filev1.FileMetadataServiceClient
	mwsAddr := getenvAny("127.0.0.1:9095", "MWS_INTEGRATION_SERVICE_GRPC_ADDR")
	if mwsAddr != "" {
		conn, err := grpc.Dial(mwsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to dial mws integration service", "error", err)
			os.Exit(1)
		}
		defer conn.Close()
		mwsClient = mwsv1.NewMWSIntegrationServiceClient(conn)
	}
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
	fileAddr := getenvAny("", "FILE_SERVICE_GRPC_ADDR")
	if fileAddr != "" {
		conn, err := grpc.Dial(fileAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to dial file service", "error", err)
			os.Exit(1)
		}
		defer conn.Close()
		fileClient = filev1.NewFileMetadataServiceClient(conn)
	}

	application, err := pageapp.NewApplication(pageapp.Config{
		DatabaseURL:          os.Getenv("PAGE_DATABASE_URL"),
		RedisAddr:            getenvAny("", "PAGE_REDIS_ADDR", "REDIS_ADDR"),
		RedisPassword:        getenvAny("", "PAGE_REDIS_PASSWORD", "REDIS_PASSWORD"),
		RedisDB:              intFromEnvAny(0, "PAGE_REDIS_DB", "REDIS_DB"),
		AuthorizationClient:  authClient,
		MWSIntegrationClient: mwsClient,
		FileMetadataClient:   fileClient,
		Logger:               logger,
		Now:                  time.Now,
	})
	if err != nil {
		logger.Error("failed to initialize page service", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	grpcServer := grpc.NewServer()
	pagev1.RegisterPageRevisionServiceServer(grpcServer, application.GRPC)
	grpcListener, err := net.Listen("tcp", ":"+strconv.Itoa(intFromEnvAny(9092, "PAGE_SERVICE_GRPC_PORT")))
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
	if natsURL := getenvAny("", "PAGE_NATS_URL", "NATS_URL"); natsURL != "" {
		natsConn, err := messaging.Connect(ctx, messaging.NATSConfig{
			URL:  natsURL,
			Name: "page-service",
		})
		if err != nil {
			logger.Error("failed to connect to nats", "error", err)
			os.Exit(1)
		}
		group.Go(func() error {
			relay := &messaging.OutboxRelay{
				Store:     application.Store,
				Publisher: messaging.NATSPublisher{Conn: natsConn},
				PollEvery: time.Second,
				BatchSize: 50,
			}
			if err := relay.Run(groupCtx); err != nil && err != context.Canceled {
				return err
			}
			return nil
		})
	}
	group.Go(func() error {
		return transport.RunHTTPServer(groupCtx, transport.HTTPServerConfig{
			ServiceName: "page-service",
			Port:        portFromEnv("PAGE_SERVICE_PORT", 8082),
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
		logger.Error("page service stopped", "error", err)
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
