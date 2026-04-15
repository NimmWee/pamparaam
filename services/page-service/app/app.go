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
	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	grpcapi "github.com/mtc/wiki-editor-backend/services/page-service/api/grpc"
	"github.com/mtc/wiki-editor-backend/services/page-service/api/rest"
	fileadapter "github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/file"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/memory"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/mws"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/postgres"
	redisadapter "github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/redis"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
)

type Config struct {
	DatabaseURL          string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	AuthorizationClient  authv1.AuthorizationServiceClient
	MWSIntegrationClient mwsv1.MWSIntegrationServiceClient
	FileMetadataClient   filev1.FileMetadataServiceClient
	Logger               *slog.Logger
	Now                  func() time.Time
	GenerateID           func() string
}

type Application struct {
	Handler http.Handler
	GRPC    pagev1.PageRevisionServiceServer
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

	var store ports.Store
	var replayWindow ports.ReplayWindowStore
	closeFn := func() error { return nil }
	readyz := func(context.Context) error { return nil }

	if cfg.DatabaseURL != "" {
		pool, err := transport.OpenPostgres(context.Background(), transport.PostgresConfig{URL: cfg.DatabaseURL})
		if err != nil {
			return nil, err
		}
		pgStore := postgres.NewStore(pool)
		store = pgStore
		closeFn = func() error {
			pool.Close()
			return nil
		}
		readyz = pgStore.Ping
	} else {
		store = memory.NewStore()
	}
	if cfg.RedisAddr != "" {
		client, err := transport.OpenRedis(context.Background(), transport.RedisConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err != nil {
			return nil, err
		}
		baseClose := closeFn
		closeFn = func() error {
			_ = baseClose()
			return client.Close()
		}
		replayWindow = redisadapter.NewReplayWindowStore(client, 32)
	} else {
		replayWindow = memory.NewReplayWindowStore(32, cfg.Now)
	}

	var resolver ports.EmbedResolver
	if cfg.MWSIntegrationClient != nil {
		resolver = mws.NewClient(cfg.MWSIntegrationClient)
	}
	var files ports.FileMetadataResolver
	if cfg.FileMetadataClient != nil {
		files = fileadapter.NewClient(cfg.FileMetadataClient)
	}
	authorizer := usecase.NewPageActionAuthorizer(runtimeauthz.NewClient(cfg.AuthorizationClient))
	editorCapabilities := usecase.EditorRuntimeCapabilities{
		SupportsRealtimeCollaboration: true,
		SupportsSyncResumeReplay:      replayWindow != nil,
		SupportsFilesIntegration:      true,
		SupportsEmbedIntegration:      true,
	}

	handler := rest.NewHandler(
		usecase.NewCreatePage(store, resolver, files, authorizer, cfg.Now, cfg.GenerateID),
		usecase.NewGetPage(store, resolver, files, authorizer),
		usecase.NewArchivePage(store, authorizer, cfg.Now, cfg.GenerateID),
		usecase.NewAutosaveDraft(store, replayWindow, resolver, files, authorizer, cfg.Now, cfg.GenerateID),
		usecase.NewRecoverDraft(store, resolver, files, authorizer),
		usecase.NewPublishPage(store, replayWindow, authorizer, cfg.Now, cfg.GenerateID),
		usecase.NewListVersions(store, authorizer),
		usecase.NewRestoreRevision(store, replayWindow, resolver, files, authorizer, cfg.Now, cfg.GenerateID),
		usecase.NewGetEditorMetadata(authorizer, editorCapabilities),
		usecase.NewResumeEditorSync(store, replayWindow, resolver, files, authorizer),
	)

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(transport.RequestContextMiddleware)
	handler.RegisterRoutes(router)

	grpcServer := grpcapi.NewServer(
		usecase.NewGetRevisionHead(store, files, authorizer),
		usecase.NewCommitCollaborativeRevision(store, replayWindow, resolver, files, authorizer, cfg.Now, cfg.GenerateID),
	)

	return &Application{
		Handler: router,
		GRPC:    grpcServer,
		Store:   store,
		Readyz:  readyz,
		Close:   closeFn,
	}, nil
}
