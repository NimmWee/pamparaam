package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mtc/wiki-editor-backend/pkg/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPServerConfig defines the baseline runtime shared by service entrypoints.
type HTTPServerConfig struct {
	ServiceName string
	Port        int
	Logger      *slog.Logger
	Metrics     *observability.Metrics
	Readyz      func(context.Context) error
	Routes      func(r chi.Router)
}

// RunHTTPServer starts a minimal chi-based server with standard platform endpoints.
func RunHTTPServer(ctx context.Context, cfg HTTPServerConfig) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(RequestContextMiddleware)
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": cfg.ServiceName,
			"status":  "ok",
		})
	})
	router.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Readyz != nil {
			if err := cfg.Readyz(r.Context()); err != nil {
				cfg.Logger.Error("readiness check failed", "error", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "not_ready",
					"error":  err.Error(),
				})
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	router.Handle("/metrics", promhttp.Handler())

	if cfg.Routes != nil {
		cfg.Routes(router)
	}

	handler := http.Handler(router)
	if cfg.Metrics != nil {
		handler = cfg.Metrics.InstrumentHTTP("all", handler)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		cfg.Logger.Info("starting http server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg.Logger.Info("shutting down http server")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return nil
	case err := <-errCh:
		return err
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
