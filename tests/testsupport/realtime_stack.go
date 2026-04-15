package testsupport

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mtc/wiki-editor-backend/pkg/authn"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	collabapp "github.com/mtc/wiki-editor-backend/services/collaboration-service/app"
	pageapp "github.com/mtc/wiki-editor-backend/services/page-service/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	WorkspaceID = "11111111-1111-1111-1111-111111111111"
	UserIDOne   = "22222222-2222-2222-2222-222222222222"
	UserIDTwo   = "33333333-3333-3333-3333-333333333333"
)

type RealtimeStack struct {
	PageServer   *httptest.Server
	CollabServer *httptest.Server
	Gateway      *httptest.Server
}

type Envelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type StaticValidator struct {
	UserID      string
	DisplayName string
}

func (v StaticValidator) ValidateAccessToken(_ context.Context, _ string) (authn.Identity, error) {
	return authn.Identity{
		UserID:      v.UserID,
		Email:       v.UserID + "@example.com",
		DisplayName: v.DisplayName,
		Roles:       []string{"editor"},
	}, nil
}

func NewRealtimeStack(t *testing.T, validator gatewayapp.Config, heartbeatInterval, presenceTTL time.Duration) *RealtimeStack {
	t.Helper()

	authFixture := NewAuthFixture(t, AuthFixtureConfig{
		Memberships: []AuthMembership{
			{UserID: UserIDOne, WorkspaceID: WorkspaceID, Role: "editor"},
			{UserID: UserIDTwo, WorkspaceID: WorkspaceID, Role: "editor"},
		},
	})

	pageService, err := pageapp.NewApplication(pageapp.Config{
		AuthorizationClient: authFixture.Client,
	})
	if err != nil {
		t.Fatalf("new page application: %v", err)
	}
	t.Cleanup(func() { _ = pageService.Close() })

	pageHTTP := httptest.NewServer(pageService.Handler)
	t.Cleanup(pageHTTP.Close)

	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen page grpc: %v", err)
	}
	grpcServer := grpc.NewServer()
	pagev1.RegisterPageRevisionServiceServer(grpcServer, pageService.GRPC)
	go func() { _ = grpcServer.Serve(grpcListener) }()
	t.Cleanup(func() {
		grpcServer.GracefulStop()
		_ = grpcListener.Close()
	})

	pageConn, err := grpc.Dial(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial page grpc: %v", err)
	}
	t.Cleanup(func() { _ = pageConn.Close() })

	collabService, err := collabapp.NewApplication(collabapp.Config{
		PageServiceClient:   pagev1.NewPageRevisionServiceClient(pageConn),
		AuthorizationClient: authFixture.Client,
		HeartbeatInterval:   heartbeatInterval,
		PresenceTTL:         presenceTTL,
	})
	if err != nil {
		t.Fatalf("new collaboration application: %v", err)
	}
	t.Cleanup(func() { _ = collabService.Close() })

	collabHTTP := httptest.NewServer(collabService.Handler)
	t.Cleanup(collabHTTP.Close)

	upstreamStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	}))
	t.Cleanup(upstreamStub.Close)

	if validator.AccessTokenValidator == nil {
		validator.AccessTokenValidator = StaticValidator{UserID: UserIDOne, DisplayName: "Editor One"}
	}
	validator.AuthServiceBaseURL = authFixture.HTTPServer.URL
	validator.AuthServiceReadyURL = authFixture.HTTPServer.URL
	validator.AuthServiceJWKSURL = authFixture.HTTPServer.URL + "/.well-known/jwks.json"
	validator.AuthorizationClient = authFixture.Client
	validator.PageServiceBaseURL = pageHTTP.URL
	validator.CollaborationServiceBaseURL = collabHTTP.URL
	validator.KnowledgeGraphSearchServiceURL = upstreamStub.URL
	validator.MWSIntegrationServiceBaseURL = upstreamStub.URL
	validator.FileServiceBaseURL = upstreamStub.URL
	validator.OpenAPIPath = filepath.Join("..", "..", "specs", "001-wiki-editor-backend", "contracts", "public-api.openapi.yaml")
	validator.JWTIssuer = "wiki-auth"
	validator.JWTAudience = "wiki-api"

	gatewayHandler, err := gatewayapp.NewHTTPHandler(validator)
	if err != nil {
		t.Fatalf("new gateway handler: %v", err)
	}
	gateway := httptest.NewServer(gatewayHandler)
	t.Cleanup(gateway.Close)

	return &RealtimeStack{
		PageServer:   pageHTTP,
		CollabServer: collabHTTP,
		Gateway:      gateway,
	}
}

func DialCollabSocket(t *testing.T, gatewayURL, pageID, workspaceID, token string, headers http.Header) *websocket.Conn {
	t.Helper()

	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("X-Workspace-Id", workspaceID)

	dialer := websocket.Dialer{}
	url := "ws" + gatewayURL[len("http"):] + "/ws/collab?page_id=" + pageID + "&workspace_id=" + workspaceID
	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func ReadEnvelope(t *testing.T, conn *websocket.Conn) Envelope {
	t.Helper()
	var envelope Envelope
	if err := conn.ReadJSON(&envelope); err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	return envelope
}
