package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey string

const (
	requestIDKey     contextKey = "request_id"
	correlationIDKey contextKey = "correlation_id"
)

// RequestContextMiddleware ensures every request carries request and correlation identifiers.
func RequestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = newOpaqueID()
		}

		correlationID := r.Header.Get("X-Correlation-Id")
		if correlationID == "" {
			correlationID = requestID
		}

		w.Header().Set("X-Request-Id", requestID)
		w.Header().Set("X-Correlation-Id", correlationID)

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, correlationIDKey, correlationID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDKey).(string); ok {
		return value
	}
	return ""
}

func CorrelationIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(correlationIDKey).(string); ok {
		return value
	}
	return ""
}

func newOpaqueID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(buffer)
}
