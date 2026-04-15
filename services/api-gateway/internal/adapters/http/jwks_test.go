package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidatorValidatesAccessToken(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("unexpected key generation error: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kid": "test-kid",
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
				},
			},
		})
	}))
	defer jwksServer.Close()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":          "user-1",
		"email":        "owner@example.com",
		"display_name": "Owner",
		"token_type":   "access",
		"iss":          "wiki-auth",
		"aud":          "wiki-api",
		"iat":          time.Now().Add(-time.Minute).Unix(),
		"exp":          time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "test-kid"

	rawToken, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}

	validator := NewJWTValidator(jwksServer.URL, "wiki-auth", "wiki-api", time.Minute, jwksServer.Client())
	identity, err := validator.ValidateAccessToken(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if identity.UserID != "user-1" || identity.Email != "owner@example.com" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}
