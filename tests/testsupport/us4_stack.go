package testsupport

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authn"
	filev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/filev1"
	pagev1 "github.com/mtc/wiki-editor-backend/pkg/contracts/pagev1"
	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	gatewayapp "github.com/mtc/wiki-editor-backend/services/api-gateway/app"
	collabapp "github.com/mtc/wiki-editor-backend/services/collaboration-service/app"
	fileapp "github.com/mtc/wiki-editor-backend/services/file-service/app"
	searchapp "github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/app"
	pageapp "github.com/mtc/wiki-editor-backend/services/page-service/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type US4Stack struct {
	PageService   *pageapp.Application
	SearchService *searchapp.Application
	FileService   *fileapp.Application
	Gateway       *httptest.Server
	Auth          *AuthFixture
}

type TokenValidator struct {
	Tokens map[string]authn.Identity
}

func (v TokenValidator) ValidateAccessToken(_ context.Context, rawToken string) (authn.Identity, error) {
	if identity, ok := v.Tokens[rawToken]; ok {
		return identity, nil
	}
	return authn.Identity{UserID: UserIDOne, DisplayName: "Editor One", Roles: []string{"editor"}}, nil
}

func NewUS4Stack(t *testing.T, validator TokenValidator) *US4Stack {
	t.Helper()

	natsURL := NewNATSServer(t)
	authFixture := NewAuthFixture(t, AuthFixtureConfig{
		Memberships: []AuthMembership{
			{UserID: UserIDOne, WorkspaceID: WorkspaceID, Role: "editor"},
			{UserID: UserIDTwo, WorkspaceID: WorkspaceID, Role: "viewer"},
		},
	})

	fileService, err := fileapp.NewApplication(fileapp.Config{
		AuthorizationClient: authFixture.Client,
	})
	if err != nil {
		t.Fatalf("new file application: %v", err)
	}
	t.Cleanup(func() { _ = fileService.Close() })
	fileHTTP := httptest.NewServer(fileService.Handler)
	t.Cleanup(fileHTTP.Close)

	fileGRPCListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen file grpc: %v", err)
	}
	fileGRPCServer := grpc.NewServer()
	filev1.RegisterFileMetadataServiceServer(fileGRPCServer, fileService.GRPC)
	go func() { _ = fileGRPCServer.Serve(fileGRPCListener) }()
	t.Cleanup(func() {
		fileGRPCServer.GracefulStop()
		_ = fileGRPCListener.Close()
	})
	fileConn, err := grpc.Dial(fileGRPCListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial file grpc: %v", err)
	}
	t.Cleanup(func() { _ = fileConn.Close() })

	pageService, err := pageapp.NewApplication(pageapp.Config{
		AuthorizationClient: authFixture.Client,
		FileMetadataClient:  filev1.NewFileMetadataServiceClient(fileConn),
	})
	if err != nil {
		t.Fatalf("new page application: %v", err)
	}
	t.Cleanup(func() { _ = pageService.Close() })
	pageHTTP := httptest.NewServer(pageService.Handler)
	t.Cleanup(pageHTTP.Close)

	pageGRPCListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen page grpc: %v", err)
	}
	pageGRPCServer := grpc.NewServer()
	pagev1.RegisterPageRevisionServiceServer(pageGRPCServer, pageService.GRPC)
	go func() { _ = pageGRPCServer.Serve(pageGRPCListener) }()
	t.Cleanup(func() {
		pageGRPCServer.GracefulStop()
		_ = pageGRPCListener.Close()
	})
	pageConn, err := grpc.Dial(pageGRPCListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial page grpc: %v", err)
	}
	t.Cleanup(func() { _ = pageConn.Close() })
	publisherConn, err := messaging.Connect(context.Background(), messaging.NATSConfig{
		URL:  natsURL,
		Name: "us4-page-tests-publisher",
	})
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(publisherConn.Close)
	consumerConn, err := messaging.Connect(context.Background(), messaging.NATSConfig{
		URL:  natsURL,
		Name: "us4-page-tests-consumer",
	})
	if err != nil {
		t.Fatalf("connect nats consumer: %v", err)
	}
	t.Cleanup(consumerConn.Close)
	pageRelayCtx, pageRelayCancel := context.WithCancel(context.Background())
	t.Cleanup(pageRelayCancel)
	go func() {
		_ = (&messaging.OutboxRelay{
			Store:     pageService.Store,
			Publisher: messaging.NATSPublisher{Conn: publisherConn},
			PollEvery: 20 * time.Millisecond,
			BatchSize: 50,
		}).Run(pageRelayCtx)
	}()

	searchService, err := searchapp.NewApplication(searchapp.Config{
		AuthorizationClient: authFixture.Client,
	})
	if err != nil {
		t.Fatalf("new search application: %v", err)
	}
	t.Cleanup(func() { _ = searchService.Close() })
	searchHTTP := httptest.NewServer(searchService.Handler)
	t.Cleanup(searchHTTP.Close)
	searchConsumerCtx, searchConsumerCancel := context.WithCancel(context.Background())
	t.Cleanup(searchConsumerCancel)
	if err := searchService.Consumer.Subscribe(searchConsumerCtx, consumerConn); err != nil {
		t.Fatalf("subscribe search consumer: %v", err)
	}

	collabService, err := collabapp.NewApplication(collabapp.Config{
		PageServiceClient:   pagev1.NewPageRevisionServiceClient(pageConn),
		AuthorizationClient: authFixture.Client,
	})
	if err != nil {
		t.Fatalf("new collaboration application: %v", err)
	}
	t.Cleanup(func() { _ = collabService.Close() })
	collabHTTP := httptest.NewServer(collabService.Handler)
	t.Cleanup(collabHTTP.Close)

	if validator.Tokens == nil {
		validator.Tokens = map[string]authn.Identity{
			"editor-token":     {UserID: UserIDOne, Email: "editor@example.com", DisplayName: "Editor", Roles: []string{"editor"}},
			"viewer-token":     {UserID: UserIDTwo, Email: "viewer@example.com", DisplayName: "Viewer", Roles: []string{"viewer"}},
			"blocked-token":    {UserID: "44444444-4444-4444-4444-444444444444", Email: "blocked@example.com", DisplayName: "Blocked", Roles: []string{}},
			"page-grant-token": {UserID: "55555555-5555-5555-5555-555555555555", Email: "grant@example.com", DisplayName: "Granted", Roles: []string{}},
		}
	}

	gatewayHandler, err := gatewayapp.NewHTTPHandler(gatewayapp.Config{
		AuthServiceBaseURL:             authFixture.HTTPServer.URL,
		AuthServiceReadyURL:            authFixture.HTTPServer.URL,
		AuthServiceJWKSURL:             authFixture.HTTPServer.URL + "/.well-known/jwks.json",
		AuthorizationClient:            authFixture.Client,
		PageServiceBaseURL:             pageHTTP.URL,
		CollaborationServiceBaseURL:    collabHTTP.URL,
		KnowledgeGraphSearchServiceURL: searchHTTP.URL,
		MWSIntegrationServiceBaseURL:   searchHTTP.URL,
		FileServiceBaseURL:             fileHTTP.URL,
		OpenAPIPath:                    filepath.Join("..", "..", "specs", "001-wiki-editor-backend", "contracts", "public-api.openapi.yaml"),
		JWTIssuer:                      "wiki-auth",
		JWTAudience:                    "wiki-api",
		AccessTokenValidator:           validator,
	})
	if err != nil {
		t.Fatalf("new gateway handler: %v", err)
	}
	gateway := httptest.NewServer(gatewayHandler)
	t.Cleanup(gateway.Close)

	return &US4Stack{
		PageService:   pageService,
		SearchService: searchService,
		FileService:   fileService,
		Gateway:       gateway,
		Auth:          authFixture,
	}
}

func JSONRequest(t *testing.T, client *http.Client, method, url, token string, body any) *http.Response {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-Id", WorkspaceID)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}
