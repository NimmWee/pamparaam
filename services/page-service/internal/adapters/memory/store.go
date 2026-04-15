package memory

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/messaging"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

var ErrNotFound = errors.New("page not found")

type Store struct {
	mu            sync.RWMutex
	pages         map[string]domain.Page
	revisions     map[string][]domain.PageRevision
	embedded      map[string][]domain.EmbeddedTableReference
	attachments   map[string][]domain.AttachmentReferenceRecord
	links         map[string][]domain.PageLinkRecord
	idempotencies map[string]domain.DraftIdempotencyRecord
	outbox        []domain.OutboxRecord
}

func NewStore() *Store {
	return &Store{
		pages:         make(map[string]domain.Page),
		revisions:     make(map[string][]domain.PageRevision),
		embedded:      make(map[string][]domain.EmbeddedTableReference),
		attachments:   make(map[string][]domain.AttachmentReferenceRecord),
		links:         make(map[string][]domain.PageLinkRecord),
		idempotencies: make(map[string]domain.DraftIdempotencyRecord),
	}
}

func (s *Store) Execute(_ context.Context, fn func(ports.PageWriter, ports.ProjectionWriter, ports.OutboxWriter) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fn(pageWriter{s}, projectionWriter{s}, outboxWriter{s})
}

func (s *Store) GetPage(_ context.Context, pageID string) (domain.Page, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	page, ok := s.pages[pageID]
	if !ok {
		return domain.Page{}, ErrNotFound
	}
	return page, nil
}

func (s *Store) GetRevision(_ context.Context, pageID string, view domain.RevisionView) (domain.PageRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	revisions := s.revisions[pageID]
	if len(revisions) == 0 {
		return domain.PageRevision{}, ErrNotFound
	}

	page, ok := s.pages[pageID]
	if !ok {
		return domain.PageRevision{}, ErrNotFound
	}

	targetID := page.CurrentDraftRevisionID
	if view == domain.RevisionViewPublished && page.CurrentPublishedRevisionID != "" {
		targetID = page.CurrentPublishedRevisionID
	}

	for _, revision := range revisions {
		if revision.ID == targetID {
			return revision, nil
		}
	}
	return domain.PageRevision{}, ErrNotFound
}

func (s *Store) GetRevisionByID(_ context.Context, pageID string, revisionID string) (domain.PageRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	revisions := s.revisions[pageID]
	for _, revision := range revisions {
		if revision.ID == revisionID {
			return revision, nil
		}
	}
	return domain.PageRevision{}, ErrNotFound
}

func (s *Store) ListRevisions(_ context.Context, pageID string) ([]domain.PageRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	revisions := append([]domain.PageRevision(nil), s.revisions[pageID]...)
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].RevisionNo > revisions[j].RevisionNo
	})
	return revisions, nil
}

func (s *Store) ListEmbeddedTableRefs(_ context.Context, revisionID string) ([]domain.EmbeddedTableReference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]domain.EmbeddedTableReference(nil), s.embedded[revisionID]...), nil
}

func (s *Store) ListAttachmentRefs(_ context.Context, revisionID string) ([]domain.AttachmentReferenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]domain.AttachmentReferenceRecord(nil), s.attachments[revisionID]...), nil
}

func (s *Store) ListPageLinks(_ context.Context, revisionID string) ([]domain.PageLinkRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]domain.PageLinkRecord(nil), s.links[revisionID]...), nil
}

func (s *Store) GetDraftIdempotency(_ context.Context, pageID, idempotencyKey string) (domain.DraftIdempotencyRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.idempotencies[idempotencyMapKey(pageID, idempotencyKey)]
	return record, ok, nil
}

func (s *Store) Ping(_ context.Context) error {
	return nil
}

func (s *Store) ClaimPending(_ context.Context, batchSize int) ([]messaging.OutboxMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]messaging.OutboxMessage, 0, batchSize)
	for _, record := range s.outbox {
		if record.Status != domain.OutboxStatusPending {
			continue
		}
		messages = append(messages, messaging.OutboxMessage{
			ID:          record.ID,
			Subject:     record.EventType,
			Payload:     record.Payload,
			OccurredAt:  record.CreatedAt,
			AvailableAt: record.AvailableAt,
		})
		if len(messages) >= batchSize {
			break
		}
	}
	return messages, nil
}

func (s *Store) MarkPublished(_ context.Context, id string, publishedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.outbox {
		if s.outbox[index].ID == id {
			s.outbox[index].Status = domain.OutboxStatusPublished
			s.outbox[index].AvailableAt = publishedAt
			return nil
		}
	}
	return nil
}

func (s *Store) MarkFailed(_ context.Context, id string, _ string, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.outbox {
		if s.outbox[index].ID == id {
			s.outbox[index].Status = domain.OutboxStatusFailed
			s.outbox[index].AvailableAt = nextAttemptAt
			return nil
		}
	}
	return nil
}

type pageWriter struct {
	store *Store
}

func (w pageWriter) CreatePage(_ context.Context, page domain.Page, revision domain.PageRevision) error {
	w.store.pages[page.ID] = page
	w.store.revisions[page.ID] = append(w.store.revisions[page.ID], revision)
	sort.Slice(w.store.revisions[page.ID], func(i, j int) bool {
		return w.store.revisions[page.ID][i].RevisionNo < w.store.revisions[page.ID][j].RevisionNo
	})
	return nil
}

func (w pageWriter) SaveDraftRevision(_ context.Context, expectedBaseRevisionNo int64, page domain.Page, revision domain.PageRevision, idempotency *domain.DraftIdempotencyRecord) error {
	current, ok := w.store.pages[page.ID]
	if !ok {
		return ErrNotFound
	}
	if current.CurrentDraftRevisionNo != expectedBaseRevisionNo {
		return ErrStaleRevision
	}

	w.store.pages[page.ID] = page
	w.store.revisions[page.ID] = append(w.store.revisions[page.ID], revision)
	sort.Slice(w.store.revisions[page.ID], func(i, j int) bool {
		return w.store.revisions[page.ID][i].RevisionNo < w.store.revisions[page.ID][j].RevisionNo
	})
	if idempotency != nil && idempotency.IdempotencyKey != "" {
		w.store.idempotencies[idempotencyMapKey(page.ID, idempotency.IdempotencyKey)] = *idempotency
	}
	return nil
}

func (w pageWriter) SavePublishedRevision(_ context.Context, expectedBaseRevisionNo int64, page domain.Page, revision domain.PageRevision) error {
	current, ok := w.store.pages[page.ID]
	if !ok {
		return ErrNotFound
	}
	if current.CurrentDraftRevisionNo != expectedBaseRevisionNo {
		return ErrStaleRevision
	}

	w.store.pages[page.ID] = page
	w.store.revisions[page.ID] = append(w.store.revisions[page.ID], revision)
	sort.Slice(w.store.revisions[page.ID], func(i, j int) bool {
		return w.store.revisions[page.ID][i].RevisionNo < w.store.revisions[page.ID][j].RevisionNo
	})
	return nil
}

func (w pageWriter) ArchivePage(_ context.Context, expectedBaseRevisionNo int64, page domain.Page) error {
	current, ok := w.store.pages[page.ID]
	if !ok {
		return ErrNotFound
	}
	if current.CurrentDraftRevisionNo != expectedBaseRevisionNo {
		return ErrStaleRevision
	}
	w.store.pages[page.ID] = page
	return nil
}

type projectionWriter struct {
	store *Store
}

func (w projectionWriter) ReplaceEmbeddedTableRefs(_ context.Context, revisionID string, refs []domain.EmbeddedTableReference) error {
	w.store.embedded[revisionID] = append([]domain.EmbeddedTableReference(nil), refs...)
	return nil
}

func (w projectionWriter) ReplaceAttachmentRefs(_ context.Context, revisionID string, refs []domain.AttachmentReferenceRecord) error {
	w.store.attachments[revisionID] = append([]domain.AttachmentReferenceRecord(nil), refs...)
	return nil
}

func (w projectionWriter) ReplacePageLinks(_ context.Context, revisionID string, refs []domain.PageLinkRecord) error {
	w.store.links[revisionID] = append([]domain.PageLinkRecord(nil), refs...)
	return nil
}

type outboxWriter struct {
	store *Store
}

func (w outboxWriter) Add(_ context.Context, records []domain.OutboxRecord) error {
	w.store.outbox = append(w.store.outbox, records...)
	return nil
}

var ErrStaleRevision = errors.New("stale revision")

func idempotencyMapKey(pageID, idempotencyKey string) string {
	return pageID + "::" + idempotencyKey
}
