package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/ports"
)

type PageEventProjector struct {
	store ports.Store
}

func NewPageEventProjector(store ports.Store) *PageEventProjector {
	return &PageEventProjector{store: store}
}

func (p *PageEventProjector) Project(ctx context.Context, message messaging.OutboxMessage) error {
	switch message.Subject {
	case "page.draft.saved", "page.published":
		return p.projectSearchDocument(ctx, message)
	case "page.links.extracted":
		return p.projectPageLinks(ctx, message)
	case "page.created", "page.attachments.changed":
		return nil
	default:
		return nil
	}
}

func (p *PageEventProjector) projectSearchDocument(ctx context.Context, message messaging.OutboxMessage) error {
	var payload struct {
		PageID         string              `json:"page_id"`
		WorkspaceID    string              `json:"workspace_id"`
		Title          string              `json:"title"`
		SearchableText string              `json:"searchable_text"`
		EmbeddedTitles []string            `json:"embedded_titles"`
		CanonicalLinks []map[string]string `json:"canonical_links"`
		UpdatedAt      string              `json:"updated_at"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return err
	}

	linkTitles := make([]string, 0, len(payload.CanonicalLinks))
	targetPageIDs := make([]string, 0, len(payload.CanonicalLinks))
	for _, link := range payload.CanonicalLinks {
		if title := strings.TrimSpace(link["target_title"]); title != "" {
			linkTitles = append(linkTitles, title)
		}
		if target := strings.TrimSpace(link["target_page_id"]); target != "" {
			targetPageIDs = append(targetPageIDs, target)
		}
	}

	updatedAt := message.OccurredAt
	if payload.UpdatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, payload.UpdatedAt); err == nil {
			updatedAt = parsed
		}
	}
	return p.store.UpsertPage(ctx, domain.PageProjection{
		PageID:         payload.PageID,
		WorkspaceID:    payload.WorkspaceID,
		Title:          payload.Title,
		SearchableText: payload.SearchableText,
		LinkTitles:     linkTitles,
		EmbedTitles:    payload.EmbeddedTitles,
		UpdatedAt:      updatedAt,
	}, targetPageIDs)
}

func (p *PageEventProjector) projectPageLinks(ctx context.Context, message messaging.OutboxMessage) error {
	var payload struct {
		PageID      string `json:"page_id"`
		WorkspaceID string `json:"workspace_id"`
		Links       []struct {
			TargetPageID string `json:"target_page_id"`
		} `json:"links"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return err
	}
	targetPageIDs := make([]string, 0, len(payload.Links))
	for _, link := range payload.Links {
		if target := strings.TrimSpace(link.TargetPageID); target != "" {
			targetPageIDs = append(targetPageIDs, target)
		}
	}
	return p.store.ReplacePageLinks(ctx, payload.WorkspaceID, payload.PageID, targetPageIDs)
}
