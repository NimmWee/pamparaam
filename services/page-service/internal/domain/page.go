package domain

import (
	"encoding/json"
	"time"
)

type PageStatus string
type RevisionView string
type RevisionSource string
type PreviewState string
type OutboxStatus string

const (
	PageStatusDraft           PageStatus     = "draft"
	PageStatusPublished       PageStatus     = "published"
	PageStatusArchived        PageStatus     = "archived"
	RevisionViewDraft         RevisionView   = "draft"
	RevisionViewPublished     RevisionView   = "published"
	RevisionSourceAutosave    RevisionSource = "rest_autosave"
	RevisionSourceCreate      RevisionSource = "rest_autosave"
	RevisionSourcePublish     RevisionSource = "publish"
	RevisionSourceRestore     RevisionSource = "restore"
	RevisionSourceCollabPatch RevisionSource = "collab_patch"
	PreviewStateReady         PreviewState   = "ready"
	PreviewStateCached        PreviewState   = "cached"
	PreviewStateDegraded      PreviewState   = "degraded"
	OutboxStatusPending       OutboxStatus   = "pending"
	OutboxStatusPublished     OutboxStatus   = "published"
	OutboxStatusFailed        OutboxStatus   = "failed"
)

type Page struct {
	ID                         string
	WorkspaceID                string
	Slug                       string
	Title                      string
	Status                     PageStatus
	CreatedBy                  string
	UpdatedBy                  string
	CurrentDraftRevisionID     string
	CurrentDraftRevisionNo     int64
	CurrentPublishedRevisionID string
	CurrentPublishedRevisionNo int64
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type PageRevision struct {
	ID                     string
	PageID                 string
	RevisionNo             int64
	RevisionKind           RevisionView
	BaseRevisionID         string
	RestoredFromRevisionID string
	Document               Document
	ExtractedTitle         string
	CreatedBy              string
	CreatedVia             RevisionSource
	CreatedAt              time.Time
}

type EmbeddedTableReference struct {
	ID             string
	PageRevisionID string
	PageID         string
	BlockID        string
	MWSTableID     string
	DisplayConfig  map[string]any
	CreatedAt      time.Time
}

type AttachmentReferenceRecord struct {
	ID             string
	PageRevisionID string
	PageID         string
	BlockID        string
	FileID         string
	CreatedAt      time.Time
}

type PageLinkRecord struct {
	ID             string
	PageRevisionID string
	SourcePageID   string
	TargetPageID   string
	BlockID        string
	LinkKind       string
	CreatedAt      time.Time
}

type OutboxRecord struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	Status        OutboxStatus
	CreatedAt     time.Time
	AvailableAt   time.Time
}

type DraftIdempotencyRecord struct {
	PageID         string
	IdempotencyKey string
	RevisionID     string
	RevisionNo     int64
	CreatedAt      time.Time
}

type PageViewPayload struct {
	PageID                     string                 `json:"page_id"`
	WorkspaceID                string                 `json:"workspace_id"`
	Title                      string                 `json:"title"`
	Slug                       string                 `json:"slug"`
	Status                     PageStatus             `json:"status"`
	CurrentDraftRevisionNo     int64                  `json:"current_draft_revision_no"`
	CurrentDraftRevisionID     string                 `json:"current_draft_revision_id"`
	CurrentPublishedRevisionNo *int64                 `json:"current_published_revision_no,omitempty"`
	CurrentPublishedRevisionID string                 `json:"current_published_revision_id,omitempty"`
	Document                   Document               `json:"document"`
	EmbeddedTables             []TableEmbedDescriptor `json:"embedded_tables"`
}

type DraftSavePayload struct {
	PageID             string                 `json:"page_id"`
	AcceptedRevisionNo int64                  `json:"accepted_revision_no"`
	AcceptedRevisionID string                 `json:"accepted_revision_id"`
	Status             string                 `json:"status"`
	Document           Document               `json:"document"`
	EmbeddedTables     []TableEmbedDescriptor `json:"embedded_tables"`
}

type DraftRecoveryPayload struct {
	PageID             string   `json:"page_id"`
	AcceptedRevisionNo int64    `json:"accepted_revision_no"`
	AcceptedRevisionID string   `json:"accepted_revision_id"`
	Document           Document `json:"document"`
}

type PagePublishPayload struct {
	PageID              string    `json:"page_id"`
	PublishedRevisionNo int64     `json:"published_revision_no"`
	PublishedRevisionID string    `json:"published_revision_id"`
	PublishedAt         time.Time `json:"published_at"`
}

type PageArchivePayload struct {
	PageID                 string     `json:"page_id"`
	Status                 PageStatus `json:"status"`
	CurrentDraftRevisionNo int64      `json:"current_draft_revision_no"`
	CurrentDraftRevisionID string     `json:"current_draft_revision_id"`
	ArchivedAt             time.Time  `json:"archived_at"`
}

type PageRevisionSummary struct {
	RevisionID   string         `json:"revision_id"`
	RevisionNo   int64          `json:"revision_no"`
	RevisionKind RevisionView   `json:"revision_kind"`
	CreatedBy    string         `json:"created_by"`
	CreatedVia   RevisionSource `json:"created_via"`
	CreatedAt    time.Time      `json:"created_at"`
}

type PageVersionListPayload struct {
	Revisions []PageRevisionSummary `json:"revisions"`
}

func BuildPageView(page Page, revision PageRevision, descriptors map[string]TableEmbedDescriptor) PageViewPayload {
	embeddedTables := make([]TableEmbedDescriptor, 0, len(descriptors))
	for _, block := range revision.Document.Blocks {
		if descriptor, ok := descriptors[block.ID]; ok {
			embeddedTables = append(embeddedTables, descriptor)
		}
	}

	var publishedRevisionNo *int64
	if page.CurrentPublishedRevisionID != "" {
		value := page.CurrentPublishedRevisionNo
		publishedRevisionNo = &value
	}

	return PageViewPayload{
		PageID:                     page.ID,
		WorkspaceID:                page.WorkspaceID,
		Title:                      page.Title,
		Slug:                       page.Slug,
		Status:                     page.Status,
		CurrentDraftRevisionNo:     page.CurrentDraftRevisionNo,
		CurrentDraftRevisionID:     page.CurrentDraftRevisionID,
		CurrentPublishedRevisionNo: publishedRevisionNo,
		CurrentPublishedRevisionID: page.CurrentPublishedRevisionID,
		Document:                   revision.Document.HydrateEmbeds(descriptors),
		EmbeddedTables:             embeddedTables,
	}
}

func BuildDraftSavePayload(pageID string, revision PageRevision, descriptors map[string]TableEmbedDescriptor) DraftSavePayload {
	embeddedTables := make([]TableEmbedDescriptor, 0, len(descriptors))
	for _, block := range revision.Document.Blocks {
		if descriptor, ok := descriptors[block.ID]; ok {
			embeddedTables = append(embeddedTables, descriptor)
		}
	}

	return DraftSavePayload{
		PageID:             pageID,
		AcceptedRevisionNo: revision.RevisionNo,
		AcceptedRevisionID: revision.ID,
		Status:             "draft_saved",
		Document:           revision.Document.HydrateEmbeds(descriptors),
		EmbeddedTables:     embeddedTables,
	}
}

func BuildDraftRecoveryPayload(pageID string, revision PageRevision, descriptors map[string]TableEmbedDescriptor) DraftRecoveryPayload {
	return DraftRecoveryPayload{
		PageID:             pageID,
		AcceptedRevisionNo: revision.RevisionNo,
		AcceptedRevisionID: revision.ID,
		Document:           revision.Document.HydrateEmbeds(descriptors),
	}
}

func BuildVersionListPayload(revisions []PageRevision) PageVersionListPayload {
	summaries := make([]PageRevisionSummary, 0, len(revisions))
	for _, revision := range revisions {
		summaries = append(summaries, PageRevisionSummary{
			RevisionID:   revision.ID,
			RevisionNo:   revision.RevisionNo,
			RevisionKind: revision.RevisionKind,
			CreatedBy:    revision.CreatedBy,
			CreatedVia:   revision.CreatedVia,
			CreatedAt:    revision.CreatedAt,
		})
	}
	return PageVersionListPayload{Revisions: summaries}
}

func NextRevisionNo(page Page) int64 {
	if page.CurrentDraftRevisionNo >= page.CurrentPublishedRevisionNo {
		return page.CurrentDraftRevisionNo + 1
	}
	return page.CurrentPublishedRevisionNo + 1
}
