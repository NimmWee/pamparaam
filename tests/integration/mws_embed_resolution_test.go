package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	mwsv1 "github.com/mtc/wiki-editor-backend/pkg/contracts/mwsv1"
	mwsapp "github.com/mtc/wiki-editor-backend/services/mws-integration-service/app"
	pageapp "github.com/mtc/wiki-editor-backend/services/page-service/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	integrationWorkspaceID = "33333333-3333-3333-3333-333333333333"
	integrationPageTitle   = "Revenue Dashboard"
	integrationTableID     = "tbl_revenue"
)

func TestEmbedResolutionIntegration(t *testing.T) {
	t.Helper()

	mwsBackend := newFakeMWSBackend()
	upstreamServer := httptest.NewServer(mwsBackend)
	t.Cleanup(upstreamServer.Close)

	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc: %v", err)
	}
	t.Cleanup(func() { _ = grpcListener.Close() })

	resolverServer, err := mwsapp.NewResolverServer(mwsapp.Config{
		MWSBaseURL: upstreamServer.URL,
	})
	if err != nil {
		t.Fatalf("new resolver server: %v", err)
	}

	grpcServer := grpc.NewServer()
	mwsv1.RegisterMWSIntegrationServiceServer(grpcServer, resolverServer)
	go func() {
		_ = grpcServer.Serve(grpcListener)
	}()
	t.Cleanup(grpcServer.GracefulStop)

	conn, err := grpc.Dial(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	pageService, err := pageapp.NewApplication(pageapp.Config{
		MWSIntegrationClient: mwsv1.NewMWSIntegrationServiceClient(conn),
	})
	if err != nil {
		t.Fatalf("new page application: %v", err)
	}
	t.Cleanup(func() {
		_ = pageService.Close()
	})

	pageServer := httptest.NewServer(pageService.Handler)
	t.Cleanup(pageServer.Close)

	createPayload := map[string]any{
		"workspace_id": integrationWorkspaceID,
		"title":        integrationPageTitle,
		"slug":         "revenue-dashboard",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{
					"id":   "embed-1",
					"type": "table_embed",
					"embed": map[string]any{
						"mws_table_id": integrationTableID,
						"title":        "Revenue",
						"display_config": map[string]any{
							"view": "grid",
						},
					},
				},
			},
		},
	}

	createResponse := doJSON(t, pageServer.Client(), http.MethodPost, pageServer.URL+"/pages", createPayload)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create embed page status = %d, want %d", createResponse.StatusCode, http.StatusCreated)
	}

	var created map[string]any
	mustDecode(t, createResponse, &created)

	pageID, _ := created["page_id"].(string)
	if pageID == "" {
		t.Fatalf("page_id missing from create response: %#v", created)
	}

	embeddedTables, ok := created["embedded_tables"].([]any)
	if !ok || len(embeddedTables) != 1 {
		t.Fatalf("embedded_tables = %#v, want one entry", created["embedded_tables"])
	}

	descriptor := embeddedTables[0].(map[string]any)
	if got, _ := descriptor["preview_state"].(string); got != "ready" {
		t.Fatalf("preview_state = %q, want %q", got, "ready")
	}

	mwsBackend.Disable()

	getResponse := doJSON(t, pageServer.Client(), http.MethodGet, pageServer.URL+"/pages/"+pageID+"?view=draft", nil)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get degraded page status = %d, want %d", getResponse.StatusCode, http.StatusOK)
	}

	var page map[string]any
	mustDecode(t, getResponse, &page)

	embeddedTables, ok = page["embedded_tables"].([]any)
	if !ok || len(embeddedTables) != 1 {
		t.Fatalf("embedded_tables after degrade = %#v, want one entry", page["embedded_tables"])
	}
	descriptor = embeddedTables[0].(map[string]any)
	if got, _ := descriptor["preview_state"].(string); got != "degraded" {
		t.Fatalf("preview_state after MWS outage = %q, want %q", got, "degraded")
	}

	createWhileUnavailable := doJSON(t, pageServer.Client(), http.MethodPost, pageServer.URL+"/pages", map[string]any{
		"workspace_id": integrationWorkspaceID,
		"title":        "Blocked Create",
		"slug":         "blocked-create",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{
					"id":   "embed-2",
					"type": "table_embed",
					"embed": map[string]any{
						"mws_table_id": "tbl_blocked",
						"title":        "Blocked",
					},
				},
			},
		},
	})
	if createWhileUnavailable.StatusCode != http.StatusBadGateway {
		t.Fatalf("create with unavailable MWS status = %d, want %d", createWhileUnavailable.StatusCode, http.StatusBadGateway)
	}
}

type fakeMWSBackend struct {
	available bool
}

func newFakeMWSBackend() *fakeMWSBackend {
	return &fakeMWSBackend{available: true}
}

func (f *fakeMWSBackend) Disable() {
	f.available = false
}

func (f *fakeMWSBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !f.available {
		http.Error(w, "mws unavailable", http.StatusServiceUnavailable)
		return
	}

	switch r.URL.Path {
	case fmt.Sprintf("/tables/%s/access", integrationTableID), "/tables/tbl_blocked/access":
		writeJSON(w, http.StatusOK, map[string]any{"allowed": true})
	case fmt.Sprintf("/tables/%s/schema", integrationTableID), "/tables/tbl_blocked/schema":
		writeJSON(w, http.StatusOK, map[string]any{
			"columns": []map[string]any{
				{"name": "amount", "type": "number"},
			},
		})
	case fmt.Sprintf("/tables/%s/preview", integrationTableID), "/tables/tbl_blocked/preview":
		writeJSON(w, http.StatusOK, []map[string]any{
			{"amount": 42},
		})
	default:
		http.NotFound(w, r)
	}
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	req.Header.Set("X-Workspace-Id", integrationWorkspaceID)
	req.Header.Set("X-Auth-User-Id", "44444444-4444-4444-4444-444444444444")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func mustDecode(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
