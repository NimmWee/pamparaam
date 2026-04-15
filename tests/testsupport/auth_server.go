package testsupport

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	authv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/authv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthMembership struct {
	UserID      string
	WorkspaceID string
	Role        string
}

type AuthPageGrant struct {
	UserID      string
	WorkspaceID string
	PageID      string
	Permission  string
}

type AuthFixtureConfig struct {
	Memberships []AuthMembership
	PageGrants  []AuthPageGrant
}

type AuthFixture struct {
	HTTPServer *httptest.Server
	Client     authv1.AuthorizationServiceClient
	server     *testAuthorizationServer
}

func NewAuthFixture(t *testing.T, cfg AuthFixtureConfig) *AuthFixture {
	t.Helper()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}))
	t.Cleanup(httpServer.Close)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen auth grpc: %v", err)
	}
	authServer := &testAuthorizationServer{
		memberships: cfg.Memberships,
		pageGrants:  cfg.PageGrants,
	}
	server := grpc.NewServer()
	authv1.RegisterAuthorizationServiceServer(server, authServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.GracefulStop()
		_ = listener.Close()
	})

	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial auth grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &AuthFixture{
		HTTPServer: httpServer,
		Client:     authv1.NewAuthorizationServiceClient(conn),
		server:     authServer,
	}
}

type testAuthorizationServer struct {
	mu          sync.RWMutex
	memberships []AuthMembership
	pageGrants  []AuthPageGrant
}

func (s *testAuthorizationServer) Authorize(_ context.Context, request *authv1.AuthorizeRequest) (*authv1.AuthorizeResponse, error) {
	result := s.evaluate(request.Identity.ActorUserID, request.Identity.WorkspaceID, request.PageID, authz.Action(request.Action))
	return &result, nil
}

func (s *testAuthorizationServer) BatchAuthorize(_ context.Context, request *authv1.BatchAuthorizeRequest) (*authv1.BatchAuthorizeResponse, error) {
	results := make([]authv1.AuthorizeResponse, 0, len(request.Checks))
	for _, check := range request.Checks {
		result := s.evaluate(check.Identity.ActorUserID, check.Identity.WorkspaceID, check.PageID, authz.Action(check.Action))
		results = append(results, result)
	}
	return &authv1.BatchAuthorizeResponse{Results: results}, nil
}

func (s *testAuthorizationServer) evaluate(userID, workspaceID, pageID string, action authz.Action) authv1.AuthorizeResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role := ""
	for _, membership := range s.memberships {
		if membership.UserID == userID && membership.WorkspaceID == workspaceID {
			role = membership.Role
			break
		}
	}

	permissions := make([]string, 0, 2)
	for _, grant := range s.pageGrants {
		if grant.UserID == userID && grant.WorkspaceID == workspaceID && grant.PageID == pageID {
			permissions = append(permissions, grant.Permission)
		}
	}

	allowed := workspaceAllows(role, action) || grantAllows(permissions, action)
	response := authv1.AuthorizeResponse{
		Allowed:                  allowed,
		EffectiveWorkspaceRole:   role,
		EffectivePagePermissions: permissions,
	}
	if !allowed {
		response.DenialReason = "permission_denied"
	}
	return response
}

func (f *AuthFixture) GrantPagePermission(grant AuthPageGrant) {
	f.server.mu.Lock()
	defer f.server.mu.Unlock()
	f.server.pageGrants = append(f.server.pageGrants, grant)
}

func workspaceAllows(role string, action authz.Action) bool {
	return authz.Allowed([]string{role}, action)
}

func grantAllows(permissions []string, action authz.Action) bool {
	has := func(expected string) bool {
		for _, permission := range permissions {
			if permission == expected {
				return true
			}
		}
		return false
	}

	switch action {
	case authz.ActionPageView:
		return has("view") || has("edit")
	case authz.ActionPageEdit, authz.ActionPageEmbedTable, authz.ActionFileUpload:
		return has("edit")
	default:
		return false
	}
}
