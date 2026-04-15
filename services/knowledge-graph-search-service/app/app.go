package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/api/rest"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/adapters"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/adapters/memory"
	postgresadapter "github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/ports"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/usecase"
)

type Config struct {
	AuthorizationClient authv1.AuthorizationServiceClient
	DatabaseURL         string
	Logger              *slog.Logger
}

type Application struct {
	Handler   http.Handler
	Store     ports.Store
	Projector *usecase.PageEventProjector
	Consumer  *adapters.PageEventConsumer
	Readyz    func(context.Context) error
	Close     func() error
}

func NewApplication(cfg Config) (*Application, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	var (
		store   ports.Store
		closeFn = func() error { return nil }
		readyz  = func(context.Context) error { return nil }
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
	filter := usecase.NewResultFilter(runtimeauthz.NewClient(cfg.AuthorizationClient))
	projector := usecase.NewPageEventProjector(store)
	handler := rest.NewHandler(
		usecase.NewSearchPages(store, filter),
		usecase.NewGetBacklinks(store, filter),
	)
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(transport.RequestContextMiddleware)
	handler.RegisterRoutes(router)

	return &Application{
		Handler:   router,
		Store:     store,
		Projector: projector,
		Consumer:  adapters.NewPageEventConsumer(projector),
		Readyz:    readyz,
		Close:     closeFn,
	}, nil
}
