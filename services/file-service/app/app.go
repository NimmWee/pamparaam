package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	filev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/filev1"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	grpcapi "github.com/mtc/wiki-editor-backend/services/file-service/api/grpc"
	"github.com/mtc/wiki-editor-backend/services/file-service/api/rest"
	minioadapter "github.com/mtc/wiki-editor-backend/services/file-service/internal/adapters/minio"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/adapters/memory"
	postgresadapter "github.com/mtc/wiki-editor-backend/services/file-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/ports"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/usecase"
)

type Config struct {
	AuthorizationClient authv1.AuthorizationServiceClient
	DatabaseURL         string
	MinIOEndpoint       string
	MinIOAccessKeyID    string
	MinIOSecretKey      string
	MinIOBucket         string
	MinIOUseSSL         bool
	MinIOPublicBaseURL  string
	Logger              *slog.Logger
	Now                 func() time.Time
	GenerateID          func() string
}

type Application struct {
	Handler http.Handler
	GRPC    filev1.FileMetadataServiceServer
	Store   ports.Store
	Readyz  func(context.Context) error
	Close   func() error
}

func NewApplication(cfg Config) (*Application, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.GenerateID == nil {
		cfg.GenerateID = uuid.NewString
	}

	var (
		store       ports.Store
		objectStore ports.ObjectStore
		closeFn     = func() error { return nil }
		readyz      = func(context.Context) error { return nil }
	)
	if cfg.DatabaseURL != "" {
		pool, err := transport.OpenPostgres(context.Background(), transport.PostgresConfig{URL: cfg.DatabaseURL})
		if err != nil {
			return nil, err
		}
		pgStore := postgresadapter.NewStore(pool)
		store = pgStore
		readyz = pgStore.Ping
		closeFn = func() error {
			pool.Close()
			return nil
		}
	} else {
		store = memory.NewStore()
		readyz = store.Ping
	}
	if cfg.MinIOEndpoint != "" {
		client, err := transport.OpenMinIO(context.Background(), transport.MinIOConfig{
			Endpoint:        cfg.MinIOEndpoint,
			AccessKeyID:     cfg.MinIOAccessKeyID,
			SecretAccessKey: cfg.MinIOSecretKey,
			UseSSL:          cfg.MinIOUseSSL,
			DefaultBucket:   cfg.MinIOBucket,
			EnsureBucket:    true,
		})
		if err != nil {
			return nil, err
		}
		objectStore = minioadapter.NewObjectStore(client, cfg.MinIOBucket, cfg.MinIOPublicBaseURL)
	} else {
		objectStore = memory.NewObjectStore()
	}
	authorizer := usecase.NewFileActionAuthorizer(runtimeauthz.NewClient(cfg.AuthorizationClient))

	handler := rest.NewHandler(
		usecase.NewStartUpload(store, objectStore, authorizer, cfg.Now, cfg.GenerateID),
		usecase.NewCompleteUpload(store, objectStore, authorizer, cfg.Now),
		usecase.NewGetFile(store, objectStore, authorizer),
		usecase.NewDeleteFile(store, authorizer, cfg.Now),
	)
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(transport.RequestContextMiddleware)
	handler.RegisterRoutes(router)

	grpcServer := grpcapi.NewServer(usecase.NewGetFileMetadata(store, authorizer))
	return &Application{
		Handler: router,
		GRPC:    grpcServer,
		Store:   store,
		Readyz:  readyz,
		Close:   closeFn,
	}, nil
}
