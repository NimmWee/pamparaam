package domain

import "time"

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	TokenType        string
	ExpiresInSeconds int64
}

type AccessClaims struct {
	Subject     string
	Email       string
	DisplayName string
	ExpiresAt   time.Time
}

type RefreshClaims struct {
	Subject   string
	SessionID string
	ExpiresAt time.Time
}
