package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mtc/wiki-editor-backend/pkg/authn"
	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	pageapp "github.com/mtc/wiki-editor-backend/services/page-service/app"
	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func newGatewayBackedPageStack(t *testing.T) (*pageapp.Application, *httptest.Server) {
	return newGatewayBackedPageStackWithRole(t, "editor")
}

func newGatewayBackedPageStackWithRole(t *testing.T, role string) (*pageapp.Application, *httptest.Server) {
	t.Helper()

	authFixture := testsupport.NewAuthFixture(t, testsupport.AuthFixtureConfig{
		Memberships: []testsupport.AuthMembership{
			{UserID: userID, WorkspaceID: workspaceID, Role: role},
		},
	})

	pageService, err := pageapp.NewApplication(pageapp.Config{
		AuthorizationClient: authFixture.Client,
	})
	if err != nil {
		t.Fatalf("new page application: %v", err)
	}
	t.Cleanup(func() { _ = pageService.Close() })

	pageServer := httptest.NewServer(pageService.Handler)
	t.Cleanup(pageServer.Close)

	upstreamStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	}))
	t.Cleanup(upstreamStub.Close)

	gatewayHandler, err := gatewayapp.NewHTTPHandler(gatewayapp.Config{
		AuthServiceBaseURL:             authFixture.HTTPServer.URL,
		AuthServiceReadyURL:            authFixture.HTTPServer.URL,
		AuthServiceJWKSURL:             authFixture.HTTPServer.URL + "/.well-known/jwks.json",
		AuthorizationClient:            authFixture.Client,
		PageServiceBaseURL:             pageServer.URL,
		CollaborationServiceBaseURL:    upstreamStub.URL,
		KnowledgeGraphSearchServiceURL: upstreamStub.URL,
		MWSIntegrationServiceBaseURL:   upstreamStub.URL,
		FileServiceBaseURL:             upstreamStub.URL,
		OpenAPIPath:                    filepath.Join("..", "..", "specs", "001-wiki-editor-backend", "contracts", "public-api.openapi.yaml"),
		JWTIssuer:                      "wiki-auth",
		JWTAudience:                    "wiki-api",
		AccessTokenValidator:           staticValidator{},
	})
	if err != nil {
		t.Fatalf("new gateway handler: %v", err)
	}

	gatewayServer := httptest.NewServer(gatewayHandler)
	t.Cleanup(gatewayServer.Close)

	return pageService, gatewayServer
}

type staticValidator struct{}

func (staticValidator) ValidateAccessToken(_ context.Context, _ string) (authn.Identity, error) {
	return authn.Identity{
		UserID:      userID,
		Email:       "editor@example.com",
		DisplayName: "Editor",
		Roles:       []string{"editor"},
	}, nil
}

func performJSONRequestWithHeaders(t *testing.T, client *http.Client, method, url string, body any, headers map[string]string) *http.Response {
	t.Helper()

	var requestBody []byte
	var err error
	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-Id", workspaceID)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}
