package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	workspaceID = "11111111-1111-1111-1111-111111111111"
	editorEmail = "editor@example.com"
	viewerEmail = "viewer@example.com"
	password    = "demo-password"
)

type authTokens struct {
	AccessToken string `json:"access_token"`
}

type pageResponse struct {
	PageID                 string `json:"page_id"`
	CurrentDraftRevisionNo int64  `json:"current_draft_revision_no"`
}

type uploadSessionResponse struct {
	UploadID  string `json:"upload_id"`
	FileID    string `json:"file_id"`
	UploadURL string `json:"upload_url"`
}

type fileObjectResponse struct {
	FileID      string `json:"file_id"`
	Filename    string `json:"filename"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
}

type searchResponse struct {
	Results []struct {
		PageID string `json:"page_id"`
		Title  string `json:"title"`
	} `json:"results"`
}

type websocketEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func main() {
	baseURL := getenv("GATEWAY_BASE_URL", "http://localhost:8080/api/v1")
	wsBaseURL := strings.TrimSuffix(baseURL, "/api/v1")
	client := &http.Client{Timeout: 15 * time.Second}

	editorToken := login(client, baseURL, editorEmail, password)
	viewerToken := login(client, baseURL, viewerEmail, password)

	pageID, revisionNo := createPage(client, baseURL, editorToken)
	assertViewerCreateForbidden(client, baseURL, viewerToken)
	waitForSearch(client, baseURL, editorToken, pageID)

	fileID := uploadAndFinalizeFile(client, baseURL, editorToken, pageID)
	verifyFileRead(client, baseURL, editorToken, fileID)
	verifyAttachmentHydration(client, baseURL, editorToken, pageID, revisionNo, fileID)
	verifyRealtimeJoin(wsBaseURL, editorToken, pageID)

	fmt.Println("compose smoke probe passed")
}

func login(client *http.Client, baseURL, email, password string) string {
	var response authTokens
	doJSON(client, http.MethodPost, baseURL+"/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	}, http.StatusOK, &response)
	if response.AccessToken == "" {
		fail("login returned empty access token for %s", email)
	}
	return response.AccessToken
}

func createPage(client *http.Client, baseURL, token string) (string, int64) {
	slug := fmt.Sprintf("compose-smoke-%d", time.Now().UnixNano())
	var response pageResponse
	doJSON(client, http.MethodPost, baseURL+"/pages", token, map[string]any{
		"workspace_id": workspaceID,
		"title":        "Compose Smoke Page",
		"slug":         slug,
		"initial_document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "compose smoke searchable body"},
			},
		},
	}, http.StatusCreated, &response)
	if response.PageID == "" || response.CurrentDraftRevisionNo == 0 {
		fail("create page returned invalid payload: %+v", response)
	}
	return response.PageID, response.CurrentDraftRevisionNo
}

func assertViewerCreateForbidden(client *http.Client, baseURL, token string) {
	request := map[string]any{
		"workspace_id": workspaceID,
		"title":        "Viewer Must Fail",
		"slug":         fmt.Sprintf("viewer-forbidden-%d", time.Now().UnixNano()),
	}
	status := doJSONStatus(client, http.MethodPost, baseURL+"/pages", token, request, nil)
	if status != http.StatusForbidden {
		fail("viewer create page status=%d want=%d", status, http.StatusForbidden)
	}
}

func waitForSearch(client *http.Client, baseURL, token, pageID string) {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var response searchResponse
		status := doJSONStatus(client, http.MethodGet, baseURL+"/search?workspace_id="+workspaceID+"&q=compose%20smoke", token, nil, &response)
		if status == http.StatusOK {
			for _, result := range response.Results {
				if result.PageID == pageID {
					return
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	fail("search did not return page %s within timeout", pageID)
}

func uploadAndFinalizeFile(client *http.Client, baseURL, token, pageID string) string {
	var session uploadSessionResponse
	doJSON(client, http.MethodPost, baseURL+"/files/uploads", token, map[string]any{
		"workspace_id": workspaceID,
		"page_id":      pageID,
		"filename":     "compose-smoke.txt",
		"content_type": "text/plain",
		"size_bytes":   int64(len("compose smoke attachment")),
		"checksum":     "smoke-checksum",
	}, http.StatusCreated, &session)

	putRequest, err := http.NewRequest(http.MethodPut, rewriteObjectURL(session.UploadURL), bytes.NewBufferString("compose smoke attachment"))
	if err != nil {
		fail("build minio upload request: %v", err)
	}
	putRequest.Header.Set("Content-Type", "text/plain")
	putResponse, err := client.Do(putRequest)
	if err != nil {
		fail("upload file to presigned url: %v", err)
	}
	defer putResponse.Body.Close()
	if putResponse.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(putResponse.Body)
		fail("upload file status=%d body=%s", putResponse.StatusCode, string(body))
	}

	var file fileObjectResponse
	doJSON(client, http.MethodPost, baseURL+"/files/uploads/"+session.UploadID+"/complete", token, map[string]any{
		"page_id":   pageID,
		"checksum":  "smoke-checksum",
	}, http.StatusOK, &file)
	if file.FileID == "" || file.Status != "ready" {
		fail("complete upload returned invalid payload: %+v", file)
	}
	return file.FileID
}

func verifyFileRead(client *http.Client, baseURL, token, fileID string) {
	var file fileObjectResponse
	doJSON(client, http.MethodGet, baseURL+"/files/"+fileID, token, nil, http.StatusOK, &file)
	if file.DownloadURL == "" || !strings.Contains(file.DownloadURL, "localhost:9000") {
		fail("download url %q does not expose host-reachable minio endpoint", file.DownloadURL)
	}
}

func verifyAttachmentHydration(client *http.Client, baseURL, token, pageID string, revisionNo int64, fileID string) {
	doJSON(client, http.MethodPatch, baseURL+"/pages/"+pageID+"/draft", token, map[string]any{
		"base_revision_no": revisionNo,
		"document": map[string]any{
			"blocks": []map[string]any{
				{"id": "blk-1", "type": "paragraph", "text": "compose smoke searchable body"},
				{"id": "blk-file", "type": "file", "attachment": map[string]any{"file_id": fileID}},
			},
		},
	}, http.StatusOK, nil)

	var page map[string]any
	doJSON(client, http.MethodGet, baseURL+"/pages/"+pageID, token, nil, http.StatusOK, &page)
	document, _ := page["document"].(map[string]any)
	blocks, _ := document["blocks"].([]any)
	for _, rawBlock := range blocks {
		block, _ := rawBlock.(map[string]any)
		attachment, _ := block["attachment"].(map[string]any)
		if attachment == nil {
			continue
		}
		if attachment["file_id"] == fileID && attachment["filename"] == "compose-smoke.txt" {
			return
		}
	}
	fail("page attachment metadata was not hydrated through file-service grpc")
}

func verifyRealtimeJoin(wsBaseURL, token, pageID string) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("X-Workspace-Id", workspaceID)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(wsBaseURL, "http")+"/ws/collab?page_id="+pageID+"&workspace_id="+workspaceID,
		headers,
	)
	if err != nil {
		fail("dial collab websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"type":       "join_session",
		"request_id": "compose-smoke-join",
		"sent_at":    time.Now().UTC(),
		"payload": map[string]any{
			"page_id":      pageID,
			"workspace_id": workspaceID,
		},
	}); err != nil {
		fail("write join_session: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var joined websocketEnvelope
	if err := conn.ReadJSON(&joined); err != nil {
		fail("read session_joined: %v", err)
	}
	if joined.Type != "session_joined" {
		fail("websocket event=%q want=session_joined", joined.Type)
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var presence websocketEnvelope
	if err := conn.ReadJSON(&presence); err != nil {
		fail("read presence_state: %v", err)
	}
	if presence.Type != "presence_state" {
		fail("websocket event=%q want=presence_state", presence.Type)
	}
}

func doJSON(client *http.Client, method, url, token string, body any, wantStatus int, out any) {
	status := doJSONStatus(client, method, url, token, body, out)
	if status != wantStatus {
		fail("%s %s status=%d want=%d", method, url, status, wantStatus)
	}
}

func doJSONStatus(client *http.Client, method, url, token string, body any, out any) int {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			fail("marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		fail("create request %s %s: %v", method, url, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Workspace-Id", workspaceID)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := client.Do(request)
	if err != nil {
		fail("request %s %s failed: %v", method, url, err)
	}
	defer response.Body.Close()

	if out != nil {
		if err := json.NewDecoder(response.Body).Decode(out); err != nil {
			fail("decode response for %s %s: %v", method, url, err)
		}
	} else {
		_, _ = io.Copy(io.Discard, response.Body)
	}

	return response.StatusCode
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func rewriteObjectURL(raw string) string {
	internalBase := os.Getenv("SMOKE_MINIO_INTERNAL_BASE_URL")
	if internalBase == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" {
		return raw
	}

	base, err := url.Parse(internalBase)
	if err != nil {
		return raw
	}
	parsed.Scheme = base.Scheme
	parsed.Host = base.Host
	return parsed.String()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
