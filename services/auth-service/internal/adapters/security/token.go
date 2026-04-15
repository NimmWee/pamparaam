package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

type TokenServiceConfig struct {
	Issuer        string
	Audience      string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	KeyID         string
	PrivateKeyPEM string
}

type TokenService struct {
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
	keyID      string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewTokenService(cfg TokenServiceConfig) (*TokenService, error) {
	privateKey, err := loadPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}

	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 15 * time.Minute
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 30 * 24 * time.Hour
	}
	if cfg.KeyID == "" {
		cfg.KeyID = "local-demo-key"
	}

	return &TokenService{
		issuer:     cfg.Issuer,
		audience:   cfg.Audience,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		keyID:      cfg.KeyID,
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
	}, nil
}

func (t *TokenService) IssueTokens(_ context.Context, user domain.User) (domain.TokenPair, string, time.Time, error) {
	now := time.Now().UTC()
	accessExpiry := now.Add(t.accessTTL)
	refreshExpiry := now.Add(t.refreshTTL)
	refreshID := uuid.NewString()

	accessToken, err := t.sign(jwt.MapClaims{
		"sub":          user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"token_type":   "access",
		"iss":          t.issuer,
		"aud":          t.audience,
		"iat":          now.Unix(),
		"exp":          accessExpiry.Unix(),
		"jti":          uuid.NewString(),
	})
	if err != nil {
		return domain.TokenPair{}, "", time.Time{}, err
	}

	refreshToken, err := t.sign(jwt.MapClaims{
		"sub":        user.ID,
		"token_type": "refresh",
		"iss":        t.issuer,
		"aud":        t.audience,
		"iat":        now.Unix(),
		"exp":        refreshExpiry.Unix(),
		"jti":        refreshID,
	})
	if err != nil {
		return domain.TokenPair{}, "", time.Time{}, err
	}

	return domain.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        "Bearer",
		ExpiresInSeconds: int64(t.accessTTL.Seconds()),
	}, refreshID, refreshExpiry, nil
}

func (t *TokenService) ParseAccessToken(token string) (domain.AccessClaims, error) {
	claims, err := t.parse(token, "access")
	if err != nil {
		return domain.AccessClaims{}, err
	}

	return domain.AccessClaims{
		Subject:     asString(claims["sub"]),
		Email:       asString(claims["email"]),
		DisplayName: asString(claims["display_name"]),
		ExpiresAt:   unixTime(claims["exp"]),
	}, nil
}

func (t *TokenService) ParseRefreshToken(token string) (domain.RefreshClaims, error) {
	claims, err := t.parse(token, "refresh")
	if err != nil {
		return domain.RefreshClaims{}, err
	}

	return domain.RefreshClaims{
		Subject:   asString(claims["sub"]),
		SessionID: asString(claims["jti"]),
		ExpiresAt: unixTime(claims["exp"]),
	}, nil
}

func (t *TokenService) JWKS() map[string]any {
	return map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": t.keyID,
				"n":   base64.RawURLEncoding.EncodeToString(t.publicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(t.publicKey.E)).Bytes()),
			},
		},
	}
}

func (t *TokenService) sign(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = t.keyID
	return token.SignedString(t.privateKey)
}

func (t *TokenService) parse(rawToken string, expectedType string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected signing algorithm: %s", token.Method.Alg())
		}
		return t.publicKey, nil
	}, jwt.WithAudience(t.audience), jwt.WithIssuer(t.issuer))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if asString(claims["token_type"]) != expectedType {
		return nil, fmt.Errorf("unexpected token type")
	}

	return claims, nil
}

func loadPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	if privateKeyPEM == "" {
		return rsa.GenerateKey(rand.Reader, 2048)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid private key PEM")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key must be RSA")
		}
		return rsaKey, nil
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func asString(value any) string {
	if stringValue, ok := value.(string); ok {
		return stringValue
	}
	return ""
}

func unixTime(value any) time.Time {
	switch typed := value.(type) {
	case float64:
		return time.Unix(int64(typed), 0).UTC()
	case int64:
		return time.Unix(typed, 0).UTC()
	default:
		return time.Time{}
	}
}
