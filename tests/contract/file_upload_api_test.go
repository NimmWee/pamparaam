package contract_test

import (
	"net/http"
	"testing"

	"github.com/mtc/wiki-editor-backend/tests/testsupport"
)

func TestFileUploadContract(t *testing.T) {
	stack := testsupport.NewUS4Stack(t, testsupport.TokenValidator{})

	startResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/files/uploads", "editor-token", map[string]any{
		"workspace_id": workspaceID,
		"page_id":      "00000000-0000-0000-0000-000000000000",
		"filename":     "diagram.png",
		"content_type": "image/png",
		"size_bytes":   1024,
		"checksum":     "abc123",
	})
	if startResp.StatusCode != http.StatusCreated {
		t.Fatalf("start upload status = %d", startResp.StatusCode)
	}

	var session map[string]any
	decodeBody(t, startResp, &session)
	uploadID := session["upload_id"].(string)
	fileID := session["file_id"].(string)
	if session["upload_url"].(string) == "" {
		t.Fatalf("upload_url missing")
	}

	completeResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/files/uploads/"+uploadID+"/complete", "editor-token", map[string]any{
		"page_id":  "00000000-0000-0000-0000-000000000000",
		"checksum": "abc123",
	})
	if completeResp.StatusCode != http.StatusOK {
		t.Fatalf("complete upload status = %d", completeResp.StatusCode)
	}

	var file map[string]any
	decodeBody(t, completeResp, &file)
	if got := file["file_id"].(string); got != fileID {
		t.Fatalf("file_id = %q, want %q", got, fileID)
	}
	if got := file["status"].(string); got != "ready" {
		t.Fatalf("status = %q, want ready", got)
	}
	if file["download_url"].(string) == "" {
		t.Fatalf("download_url missing")
	}

	getFileResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/files/"+fileID, "editor-token", nil)
	if getFileResp.StatusCode != http.StatusOK {
		t.Fatalf("get file status = %d", getFileResp.StatusCode)
	}
	var fetchedFile map[string]any
	decodeBody(t, getFileResp, &fetchedFile)
	if got := fetchedFile["filename"].(string); got != "diagram.png" {
		t.Fatalf("filename = %q, want diagram.png", got)
	}
	if fetchedFile["download_url"].(string) == "" {
		t.Fatalf("get file download_url missing")
	}

	createResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodPost, stack.Gateway.URL+"/api/v1/pages", "editor-token", map[string]any{
		"workspace_id": workspaceID,
		"title":        "Attachment Page",
		"slug":         "attachment-page",
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "file", "attachment": map[string]any{"file_id": fileID}},
			},
		},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create page with attachment status = %d", createResp.StatusCode)
	}
	var created map[string]any
	decodeBody(t, createResp, &created)

	getResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/pages/"+created["page_id"].(string), "editor-token", nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get page status = %d", getResp.StatusCode)
	}
	var page map[string]any
	decodeBody(t, getResp, &page)
	document := page["document"].(map[string]any)
	block := document["blocks"].([]any)[0].(map[string]any)
	attachment := block["attachment"].(map[string]any)
	if got := attachment["filename"].(string); got != "diagram.png" {
		t.Fatalf("attachment filename = %q, want diagram.png", got)
	}

	deleteResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodDelete, stack.Gateway.URL+"/api/v1/files/"+fileID, "editor-token", nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete file status = %d", deleteResp.StatusCode)
	}
	var deletedFile map[string]any
	decodeBody(t, deleteResp, &deletedFile)
	if got := deletedFile["status"].(string); got != "deleted" {
		t.Fatalf("deleted status = %q, want deleted", got)
	}

	getDeletedResp := testsupport.JSONRequest(t, stack.Gateway.Client(), http.MethodGet, stack.Gateway.URL+"/api/v1/files/"+fileID, "editor-token", nil)
	if getDeletedResp.StatusCode != http.StatusNotFound {
		t.Fatalf("get deleted file status = %d, want 404", getDeletedResp.StatusCode)
	}
}
