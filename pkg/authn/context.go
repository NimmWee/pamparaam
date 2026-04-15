package authn

import "context"

type contextKey string

const identityKey contextKey = "identity"

// Identity carries the authenticated subject propagated across layers.
type Identity struct {
	UserID      string
	Email       string
	DisplayName string
	WorkspaceID string
	Roles       []string
}

// WithIdentity stores identity information in a request context.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

// IdentityFromContext safely extracts the current identity.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey).(Identity)
	return identity, ok
}
