package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestBacklinkSearchAndRBACIntegration(t *testing.T) {
	stack := testsupport.NewUS4Stack(t, testsupport.TokenValidator{})

	createResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", "editor-token", map[string]any{
		"workspace_id": testsupport.WorkspaceID,
		"title":        "Secure Page",
		"slug":         "secure-page",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "Shared text"},
			},
		},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("editor create page status = %d", createResp.StatusCode)
	}
	var created map[string]any
	mustDecode(t, createResp, &created)
	pageID := created["page_id"].(string)

	testsupport.Eventually(t, 3*time.Second, func() bool {
		results, err := stack.SearchService.Store.Search(context.Background(), testsupport.WorkspaceID, "Shared", "")
		return err == nil && len(results) > 0
	})

	searchResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/search?workspace_id="+testsupport.WorkspaceID+"&q=Shared", "viewer-token", nil)
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("viewer search status = %d, want 200", searchResp.StatusCode)
	}

	blockedSearch := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/search?workspace_id="+testsupport.WorkspaceID+"&q=Shared", "blocked-token", nil)
	if blockedSearch.StatusCode != http.StatusForbidden {
		t.Fatalf("blocked search status = %d, want 403", blockedSearch.StatusCode)
	}

	viewerSave := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPatch, stack.Gateway.URL+"/api/v1/pages/"+pageID+"/draft", "viewer-token", map[string]any{
		"base_revision_no": 1,
		"document": map[string]any{
			"blocks": []map[string]any{{"id": "blk-1", "type": "paragraph", "text": "viewer edit"}},
		},
	})
	if viewerSave.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer autosave status = %d, want 403", viewerSave.StatusCode)
	}

	blockedGet := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/pages/"+pageID, "blocked-token", nil)
	if blockedGet.StatusCode != http.StatusForbidden {
		t.Fatalf("blocked page get status = %d, want 403", blockedGet.StatusCode)
	}

	stack.Auth.GrantPagePermission(testsupport.AuthPageGrant{
		UserID:      "55555555-5555-5555-5555-555555555555",
		WorkspaceID: testsupport.WorkspaceID,
		PageID:      pageID,
		Permission:  "view",
	})
	grantedGet := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/pages/"+pageID, "page-grant-token", nil)
	if grantedGet.StatusCode != http.StatusOK {
		t.Fatalf("page grant get status = %d, want 200", grantedGet.StatusCode)
	}

	viewerUpload := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/files/uploads", "viewer-token", map[string]any{
		"workspace_id": testsupport.WorkspaceID,
		"page_id":      pageID,
		"filename":     "blocked.txt",
		"content_type": "text/plain",
		"size_bytes":   10,
	})
	if viewerUpload.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer upload status = %d, want 403", viewerUpload.StatusCode)
	}

	embedCreate := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", "viewer-token", map[string]any{
		"workspace_id": testsupport.WorkspaceID,
		"title":        "Embed Attempt",
		"slug":         "embed-attempt",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-embed", "type": "table_embed", "embed": map[string]any{"mws_table_id": "tbl_1", "title": "Restricted"}},
			},
		},
	})
	if embedCreate.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer embed create status = %d, want 403", embedCreate.StatusCode)
	}

	conn := testsupport.DialCollabSocket(t, stack.Gateway.URL, pageID, testsupport.WorkspaceID, "blocked-token", nil)
	if err := conn.WriteJSON(map[string]any{
		"type":       "join_session",
		"request_id": "req-join",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"page_id":      pageID,
			"workspace_id": testsupport.WorkspaceID,
		},
	}); err != nil {
		t.Fatalf("blocked join write: %v", err)
	}
	envelope := testsupport.ReadEnvelope(t, conn)
	if envelope.Type != "error" {
		t.Fatalf("blocked collab join event = %q, want error", envelope.Type)
	}
}
