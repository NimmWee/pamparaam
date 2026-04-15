package contract_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

const (
	workspaceID = "11111111-1111-1111-1111-111111111111"
	userID      = "22222222-2222-2222-2222-222222222222"
)

func TestPageCRUDContract(t *testing.T) {
	t.Helper()

	_, gatewayServer := newGatewayBackedPageStack(t)

	createBody := map[string]any{
		"workspace_id": workspaceID,
		"title":        "Product Overview",
		"slug":         "product-overview",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{
					"id":   "blk-1",
					"type": "paragraph",
					"text": "Connected knowledge lives here.",
				},
			},
		},
	}

	createResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/v1/pages", createBody)
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create page status = %d, want %d; body=%s", createResponse.StatusCode, http.StatusCreated, string(readBody(t, createResponse)))
	}

	var created map[string]any
	decodeBody(t, createResponse, &created)

	pageID, _ := created["page_id"].(string)
	if pageID == "" {
		t.Fatalf("create page response missing page_id: %#v", created)
	}
	if got, _ := created["workspace_id"].(string); got != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", got, workspaceID)
	}
	if got, _ := created["title"].(string); got != "Product Overview" {
		t.Fatalf("title = %q, want %q", got, "Product Overview")
	}
	if got, _ := created["status"].(string); got != "draft" {
		t.Fatalf("status = %q, want %q", got, "draft")
	}
	if _, ok := created["current_draft_revision_id"].(string); !ok {
		t.Fatalf("current_draft_revision_id missing in response: %#v", created)
	}
	baseRevisionNo, ok := created["current_draft_revision_no"].(float64)
	if !ok {
		t.Fatalf("current_draft_revision_no missing in response: %#v", created)
	}

	getResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/v1/pages/"+pageID+"?view=draft", nil)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get page status = %d, want %d; body=%s", getResponse.StatusCode, http.StatusOK, string(readBody(t, getResponse)))
	}

	var page map[string]any
	decodeBody(t, getResponse, &page)

	if got, _ := page["page_id"].(string); got != pageID {
		t.Fatalf("page_id = %q, want %q", got, pageID)
	}
	if got, _ := page["slug"].(string); got != "product-overview" {
		t.Fatalf("slug = %q, want %q", got, "product-overview")
	}

	document, ok := page["document"].(map[string]any)
	if !ok {
		t.Fatalf("document missing in page response: %#v", page)
	}
	blocks, ok := document["blocks"].([]any)
	if !ok || len(blocks) != 1 {
		t.Fatalf("document blocks = %#v, want 1 block", document["blocks"])
	}

	archiveResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodPost, gatewayServer.URL+"/api/v1/pages/"+pageID+"/archive", map[string]any{
		"base_revision_no": int64(baseRevisionNo),
	})
	if archiveResponse.StatusCode != http.StatusOK {
		t.Fatalf("archive page status = %d, want %d; body=%s", archiveResponse.StatusCode, http.StatusOK, string(readBody(t, archiveResponse)))
	}

	var archived map[string]any
	decodeBody(t, archiveResponse, &archived)
	if got, _ := archived["status"].(string); got != "archived" {
		t.Fatalf("archive status = %q, want %q", got, "archived")
	}

	archivedGetResponse := performJSONRequest(t, gatewayServer.Client(), http.MethodGet, gatewayServer.URL+"/api/v1/pages/"+pageID+"?view=draft", nil)
	if archivedGetResponse.StatusCode != http.StatusOK {
		t.Fatalf("get archived page status = %d, want %d; body=%s", archivedGetResponse.StatusCode, http.StatusOK, string(readBody(t, archivedGetResponse)))
	}

	var archivedPage map[string]any
	decodeBody(t, archivedGetResponse, &archivedPage)
	if got, _ := archivedPage["status"].(string); got != "archived" {
		t.Fatalf("page status after archive = %q, want %q", got, "archived")
	}

	saveAfterArchive := performJSONRequest(t, gatewayServer.Client(), http.MethodPatch, gatewayServer.URL+"/api/v1/pages/"+pageID+"/draft", map[string]any{
		"base_revision_no": int64(baseRevisionNo),
		"document": map[string]any{
			"blocks": []map[string]any{
				{
					"id":   "blk-1",
					"type": "paragraph",
					"text": "should fail",
				},
			},
		},
	})
	if saveAfterArchive.StatusCode != http.StatusConflict {
		t.Fatalf("autosave archived page status = %d, want %d; body=%s", saveAfterArchive.StatusCode, http.StatusConflict, string(readBody(t, saveAfterArchive)))
	}
}

func performJSONRequest(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	return performJSONRequestWithHeaders(t, client, method, url, body, nil)
}

func decodeBody(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func readBody(t *testing.T, response *http.Response) []byte {
	t.Helper()
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body
}
