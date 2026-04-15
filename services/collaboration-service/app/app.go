package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/api/websocket"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/adapters"
	memoryadapters "github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/adapters/memory"
	redisadapters "github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/adapters/redis"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/ports"
	"github.com/mtc/wiki-editor-backend/services/collaboration-service/internal/usecase"
)

type Config struct {
	PageServiceClient   pagev1.PageRevisionServiceClient
	AuthorizationClient authv1.AuthorizationServiceClient
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
	Logger              *slog.Logger
	Now                 func() time.Time
	GenerateID          func() string
	HeartbeatInterval   time.Duration
	PresenceTTL         time.Duration
}

type Application struct {
	Handler         http.Handler
	Readyz          func(context.Context) error
	Close           func() error
	RefreshConsumer *adapters.RevisionRefreshConsumer
}

var ErrMissingPageClient = errors.New("page service client is required")

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
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 15 * time.Second
	}
	if cfg.PresenceTTL <= 0 {
		cfg.PresenceTTL = 45 * time.Second
	}
	if cfg.PageServiceClient == nil {
		return nil, ErrMissingPageClient
	}

	var (
		rooms    ports.RoomStateStore
		presence ports.PresenceStore
		closeFn  = func() error { return nil }
		readyz   = func(context.Context) error { return nil }
	)

	if cfg.RedisAddr != "" {
		client, err := transport.OpenRedis(context.Background(), transport.RedisConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err != nil {
			return nil, err
		}
		rooms = redisadapters.NewRoomStateStore(client)
		presence = redisadapters.NewPresenceStore(client)
		closeFn = client.Close
		readyz = func(ctx context.Context) error {
			return client.Ping(ctx).Err()
		}
	} else {
		rooms = memoryadapters.NewRoomStateStore()
		presence = memoryadapters.NewPresenceStore()
	}

	pageClient := adapters.NewPageClient(cfg.PageServiceClient)
	lifecycle := usecase.NewSessionLifecycle(rooms, presence, pageClient, cfg.Now, cfg.GenerateID)
	submitPatch := usecase.NewSubmitPatch(rooms, presence, pageClient)
	authorizer := usecase.NewSessionAuthorizer(runtimeauthz.NewClient(cfg.AuthorizationClient))
	refreshConsumer := adapters.NewRevisionRefreshConsumer(lifecycle)

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(transport.RequestContextMiddleware)
	websocket.NewHandler(lifecycle, submitPatch, authorizer, cfg.HeartbeatInterval, cfg.PresenceTTL).RegisterRoutes(router)

	return &Application{
		Handler:         router,
		Readyz:          readyz,
		Close:           closeFn,
		RefreshConsumer: refreshConsumer,
	}, nil
}
