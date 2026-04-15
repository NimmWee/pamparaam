package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrUnavailable = errors.New("mws unavailable")
	ErrForbidden   = errors.New("mws access denied")
	ErrNotFound    = errors.New("mws table not found")
)

type MWSClient struct {
	baseURL string
	client  *http.Client
}

func NewMWSClient(baseURL string, client *http.Client) *MWSClient {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &MWSClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

func (c *MWSClient) ValidateAccess(ctx context.Context, accessToken string, tableID string) error {
	request, err := c.newRequest(ctx, http.MethodGet, tablePath(tableID, "access"), accessToken)
	if err != nil {
		return err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusForbidden, http.StatusUnauthorized:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return ErrUnavailable
	}
}

func (c *MWSClient) FetchSchema(ctx context.Context, accessToken string, tableID string) (map[string]any, error) {
	var schema map[string]any
	if err := c.fetchJSON(ctx, tablePath(tableID, "schema"), accessToken, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

func (c *MWSClient) FetchPreview(ctx context.Context, accessToken string, tableID string) ([]map[string]any, error) {
	var rows []map[string]any
	if err := c.fetchJSON(ctx, tablePath(tableID, "preview"), accessToken, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *MWSClient) fetchJSON(ctx context.Context, path, accessToken string, target any) error {
	request, err := c.newRequest(ctx, http.MethodGet, path, accessToken)
	if err != nil {
		return err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return json.NewDecoder(response.Body).Decode(target)
	case http.StatusForbidden, http.StatusUnauthorized:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return ErrUnavailable
	}
}

func (c *MWSClient) newRequest(ctx context.Context, method, path, accessToken string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return request, nil
}

func tablePath(tableID, suffix string) string {
	return fmt.Sprintf("/tables/%s/%s", tableID, suffix)
}
