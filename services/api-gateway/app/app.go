package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	httpadapter "github.com/mtc/wiki-editor-backend/services/api-gateway/internal/adapters/http"
)

type Config = httpadapter.Config

func NewHTTPHandler(cfg Config) (http.Handler, error) {
	gateway, err := httpadapter.NewGateway(cfg)
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	router.Use(transport.RequestContextMiddleware)
	gateway.RegisterRoutes(router)
	return router, nil
}
