package memory

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/ports"
)

type Store struct {
	mu            sync.RWMutex
	pages         map[string]domain.PageProjection
	targetsByPage map[string][]string
}

func NewStore() *Store {
	return &Store{
		pages:         map[string]domain.PageProjection{},
		targetsByPage: map[string][]string{},
	}
}

func (s *Store) UpsertPage(_ context.Context, projection domain.PageProjection, targetPageIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages[projection.PageID] = projection
	s.targetsByPage[projection.PageID] = append([]string(nil), targetPageIDs...)
	return nil
}

func (s *Store) ReplacePageLinks(_ context.Context, workspaceID, sourcePageID string, targetPageIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if page, ok := s.pages[sourcePageID]; ok && page.WorkspaceID == workspaceID {
		s.targetsByPage[sourcePageID] = append([]string(nil), targetPageIDs...)
	}
	return nil
}

func (s *Store) Search(_ context.Context, workspaceID, query, sortKey string) ([]domain.SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	needle := strings.ToLower(strings.TrimSpace(query))
	results := make([]domain.SearchResult, 0)
	for _, page := range s.pages {
		if page.WorkspaceID != workspaceID {
			continue
		}
		matchType := ""
		snippet := ""
		switch {
		case strings.Contains(strings.ToLower(page.Title), needle):
			matchType = "title"
			snippet = page.Title
		case containsAny(page.EmbedTitles, needle):
			matchType = "embed_reference"
			snippet = firstMatch(page.EmbedTitles, needle)
		case containsAny(page.LinkTitles, needle):
			matchType = "link"
			snippet = firstMatch(page.LinkTitles, needle)
		case strings.Contains(strings.ToLower(page.SearchableText), needle):
			matchType = "content"
			snippet = page.SearchableText
		}
		if matchType == "" {
			continue
		}
		results = append(results, domain.SearchResult{
			PageID:    page.PageID,
			Title:     page.Title,
			MatchType: matchType,
			Snippet:   snippet,
			UpdatedAt: page.UpdatedAt,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if sortKey == "updated_at" {
			return results[i].UpdatedAt.After(results[j].UpdatedAt)
		}
		if results[i].MatchType != results[j].MatchType {
			return results[i].MatchType < results[j].MatchType
		}
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	return results, nil
}

func (s *Store) GetBacklinks(_ context.Context, workspaceID, pageID string) (domain.BacklinksPayload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backlinks := make([]domain.BacklinkReference, 0)
	currentTargets := s.targetsByPage[pageID]
	for sourcePageID, targets := range s.targetsByPage {
		if sourcePageID == pageID {
			continue
		}
		if !containsID(targets, pageID) {
			continue
		}
		page := s.pages[sourcePageID]
		if page.WorkspaceID != workspaceID {
			continue
		}
		backlinks = append(backlinks, domain.BacklinkReference{
			PageID:   sourcePageID,
			Title:    page.Title,
			Relation: "backlink",
		})
	}

	related := make([]domain.BacklinkReference, 0)
	for candidatePageID, page := range s.pages {
		if candidatePageID == pageID || page.WorkspaceID != workspaceID {
			continue
		}

		relation := ""
		switch {
		case containsID(currentTargets, candidatePageID):
			relation = "linked_page"
		case containsID(s.targetsByPage[candidatePageID], pageID):
			relation = "linked_from"
		case countSharedIDs(currentTargets, s.targetsByPage[candidatePageID]) > 0:
			relation = "shared_links"
		case strings.Contains(strings.ToLower(page.SearchableText), strings.ToLower(s.pages[pageID].Title)):
			relation = "topical_similarity"
		}
		if relation == "" {
			continue
		}
		related = append(related, domain.BacklinkReference{
			PageID:   candidatePageID,
			Title:    page.Title,
			Relation: relation,
		})
	}

	sort.Slice(backlinks, func(i, j int) bool { return backlinks[i].Title < backlinks[j].Title })
	sort.Slice(related, func(i, j int) bool { return related[i].Title < related[j].Title })
	return domain.BacklinksPayload{
		PageID:       pageID,
		Backlinks:    backlinks,
		RelatedPages: related,
	}, nil
}

func (s *Store) Ping(context.Context) error {
	return nil
}

func containsAny(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func firstMatch(values []string, needle string) string {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return value
		}
	}
	return ""
}

func containsID(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func countSharedIDs(left, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	lookup := make(map[string]struct{}, len(left))
	for _, value := range left {
		lookup[value] = struct{}{}
	}
	count := 0
	for _, value := range right {
		if _, ok := lookup[value]; ok {
			count++
		}
	}
	return count
}

var _ ports.Store = (*Store)(nil)
