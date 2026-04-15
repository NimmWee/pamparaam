package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	const query = `
SELECT id, email, display_name, password_hash, status
FROM users
WHERE email = $1
`
	return scanUser(r.pool.QueryRow(ctx, query, email))
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	const query = `
SELECT id, email, display_name, password_hash, status
FROM users
WHERE id = $1
`
	return scanUser(r.pool.QueryRow(ctx, query, userID))
}

func (r *Repository) ListMemberships(ctx context.Context, userID string) ([]domain.Membership, error) {
	const query = `
SELECT workspace_id, user_id, role
FROM workspace_memberships
WHERE user_id = $1
ORDER BY created_at ASC
`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memberships []domain.Membership
	for rows.Next() {
		var membership domain.Membership
		if err := rows.Scan(&membership.WorkspaceID, &membership.UserID, &membership.Role); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}

	return memberships, rows.Err()
}

func (r *Repository) ListPageGrants(ctx context.Context, userID, workspaceID, pageID string) ([]domain.PageGrant, error) {
	const query = `
SELECT page_id, workspace_id, subject_user_id, permission
FROM page_grants
WHERE subject_user_id = $1 AND workspace_id = $2 AND page_id = $3
ORDER BY created_at ASC
`
	rows, err := r.pool.Query(ctx, query, userID, workspaceID, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []domain.PageGrant
	for rows.Next() {
		var grant domain.PageGrant
		if err := rows.Scan(&grant.PageID, &grant.WorkspaceID, &grant.SubjectUserID, &grant.Permission); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}

	return grants, rows.Err()
}

func (r *Repository) CreateRefreshSession(ctx context.Context, session domain.RefreshSession) error {
	const query = `
INSERT INTO refresh_sessions (id, user_id, expires_at, user_agent, client_ip)
VALUES ($1, $2, $3, $4, $5)
`
	_, err := r.pool.Exec(ctx, query, session.ID, session.UserID, session.ExpiresAt, session.UserAgent, session.ClientIP)
	return err
}

func (r *Repository) GetRefreshSession(ctx context.Context, sessionID string) (domain.RefreshSession, error) {
	const query = `
SELECT id, user_id, expires_at, COALESCE(user_agent, ''), COALESCE(client_ip, '')
FROM refresh_sessions
WHERE id = $1 AND revoked_at IS NULL
`
	var session domain.RefreshSession
	err := r.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.ExpiresAt,
		&session.UserAgent,
		&session.ClientIP,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshSession{}, ErrNotFound
	}
	return session, err
}

func (r *Repository) RevokeRefreshSession(ctx context.Context, sessionID, replacedBy string, revokedAt time.Time) error {
	const query = `
UPDATE refresh_sessions
SET revoked_at = $2, replaced_by = NULLIF($3, '')
WHERE id = $1
`
	_, err := r.pool.Exec(ctx, query, sessionID, revokedAt, replacedBy)
	return err
}

func scanUser(row pgx.Row) (domain.User, error) {
	var user domain.User
	err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	return user, err
}
