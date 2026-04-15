package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/adapters/security"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	databaseURL := os.Getenv("AUTH_DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "AUTH_DATABASE_URL is required")
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	hasher := security.NewPasswordHasher(12)
	passwordHash, err := hasher.Hash("demo-password")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	workspaceID := "11111111-1111-1111-1111-111111111111"
	pageID := "22222222-2222-2222-2222-222222222222"

	if _, err := pool.Exec(ctx, `
INSERT INTO workspaces (id, name, slug)
VALUES ($1, 'Demo Workspace', 'demo-workspace')
ON CONFLICT (id) DO NOTHING
`, workspaceID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	type demoUser struct {
		ID    string
		Email string
		Name  string
		Role  string
	}

	users := []demoUser{
		{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Email: "owner@example.com", Name: "Workspace Owner", Role: "owner"},
		{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Email: "editor@example.com", Name: "Page Editor", Role: "editor"},
		{ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", Email: "viewer@example.com", Name: "Page Viewer", Role: "viewer"},
	}

	for _, user := range users {
		if _, err := pool.Exec(ctx, `
INSERT INTO users (id, email, display_name, password_hash, status)
VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (email) DO UPDATE
SET display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    status = EXCLUDED.status,
    updated_at = NOW()
`, user.ID, user.Email, user.Name, passwordHash); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if _, err := pool.Exec(ctx, `
INSERT INTO workspace_memberships (workspace_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, user_id) DO UPDATE
SET role = EXCLUDED.role
`, workspaceID, user.ID, user.Role); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO page_grants (id, page_id, workspace_id, subject_user_id, permission)
VALUES ($1, $2, $3, $4, 'edit')
ON CONFLICT (page_id, subject_user_id, permission) DO NOTHING
`, uuid.NewString(), pageID, workspaceID, users[2].ID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("seeded auth demo data")
}
