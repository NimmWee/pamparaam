package http

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mtc/wiki-editor-backend/pkg/authn"
)

type JWTValidator struct {
	jwksURL  string
	issuer   string
	audience string
	cacheTTL time.Duration
	client   *http.Client

	mu         sync.RWMutex
	lastFetch  time.Time
	publicKeys map[string]*rsa.PublicKey
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KeyID    string `json:"kid"`
	Modulus  string `json:"n"`
	Exponent string `json:"e"`
}

func NewJWTValidator(jwksURL, issuer, audience string, cacheTTL time.Duration, client *http.Client) *JWTValidator {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}

	return &JWTValidator{
		jwksURL:    jwksURL,
		issuer:     issuer,
		audience:   audience,
		cacheTTL:   cacheTTL,
		client:     client,
		publicKeys: map[string]*rsa.PublicKey{},
	}
}

func (v *JWTValidator) ValidateAccessToken(ctx context.Context, rawToken string) (authn.Identity, error) {
	claims, err := v.parseToken(ctx, rawToken)
	if err != nil {
		return authn.Identity{}, err
	}

	return authn.Identity{
		UserID:      claimAsString(claims["sub"]),
		Email:       claimAsString(claims["email"]),
		DisplayName: claimAsString(claims["display_name"]),
	}, nil
}

func (v *JWTValidator) parseToken(ctx context.Context, rawToken string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		return v.lookupKey(ctx, token)
	}, jwt.WithAudience(v.audience), jwt.WithIssuer(v.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err == nil && token != nil && token.Valid {
		return validateAccessClaims(token.Claims)
	}

	if refreshErr := v.refreshKeys(ctx, true); refreshErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, refreshErr
	}

	token, err = jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		return v.lookupKey(ctx, token)
	}, jwt.WithAudience(v.audience), jwt.WithIssuer(v.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		return nil, err
	}
	return validateAccessClaims(token.Claims)
}

func (v *JWTValidator) lookupKey(ctx context.Context, token *jwt.Token) (any, error) {
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, fmt.Errorf("missing kid header")
	}

	if key := v.cachedKey(kid); key != nil {
		return key, nil
	}

	if err := v.refreshKeys(ctx, false); err != nil {
		return nil, err
	}

	if key := v.cachedKey(kid); key != nil {
		return key, nil
	}

	return nil, fmt.Errorf("signing key not found")
}

func (v *JWTValidator) cachedKey(kid string) *rsa.PublicKey {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if time.Since(v.lastFetch) > v.cacheTTL {
		return nil
	}

	return v.publicKeys[kid]
}

func (v *JWTValidator) refreshKeys(ctx context.Context, force bool) error {
	v.mu.RLock()
	if !force && time.Since(v.lastFetch) <= v.cacheTTL && len(v.publicKeys) > 0 {
		v.mu.RUnlock()
		return nil
	}
	v.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("jwks fetch failed with status %d", resp.StatusCode)
	}

	var document jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, key := range document.Keys {
		publicKey, err := decodePublicKey(key)
		if err != nil {
			return err
		}
		keys[key.KeyID] = publicKey
	}

	v.mu.Lock()
	v.publicKeys = keys
	v.lastFetch = time.Now()
	v.mu.Unlock()

	return nil
}

func decodePublicKey(key jwkKey) (*rsa.PublicKey, error) {
	modulusBytes, err := base64.RawURLEncoding.DecodeString(key.Modulus)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(key.Exponent)
	if err != nil {
		return nil, err
	}

	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("invalid exponent")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func validateAccessClaims(rawClaims any) (jwt.MapClaims, error) {
	claims, ok := rawClaims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claimAsString(claims["token_type"]) != "access" {
		return nil, fmt.Errorf("unexpected token type")
	}
	return claims, nil
}

func claimAsString(value any) string {
	if stringValue, ok := value.(string); ok {
		return stringValue
	}
	return ""
}
