package usecase

import (
	"context"
	"errors"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
)

var ErrForbidden = errors.New("forbidden")

type AuthorizationSubject struct {
	ActorUserID   string
	WorkspaceID   string
	PageID        string
	Roles         []string
	Authenticated bool
}

type ResultFilter struct {
	checker *runtimeauthz.Client
}

func NewResultFilter(checker *runtimeauthz.Client) *ResultFilter {
	return &ResultFilter{checker: checker}
}

func (f *ResultFilter) Require(ctx context.Context, action authz.Action, subject AuthorizationSubject) error {
	if !subject.Authenticated {
		return nil
	}

	if f.checker != nil && f.checker.Enabled() {
		decision, err := f.checker.Authorize(ctx, runtimeauthz.CheckInput{
			ActorUserID: subject.ActorUserID,
			WorkspaceID: subject.WorkspaceID,
			PageID:      subject.PageID,
			Action:      action,
		})
		if err != nil {
			return err
		}
		if decision.Allowed {
			return nil
		}
		return ErrForbidden
	}

	if authz.Allowed(subject.Roles, action) {
		return nil
	}
	return ErrForbidden
}

func (f *ResultFilter) FilterSearchResults(ctx context.Context, subject AuthorizationSubject, results []domain.SearchResult) ([]domain.SearchResult, error) {
	if len(results) == 0 || !subject.Authenticated {
		return results, nil
	}
	if f.checker != nil && f.checker.Enabled() {
		checks := make([]runtimeauthz.CheckInput, 0, len(results))
		for _, result := range results {
			checks = append(checks, runtimeauthz.CheckInput{
				ActorUserID: subject.ActorUserID,
				WorkspaceID: subject.WorkspaceID,
				PageID:      result.PageID,
				Action:      authz.ActionPageView,
			})
		}
		decisions, err := f.checker.BatchAuthorize(ctx, checks)
		if err != nil {
			return nil, err
		}
		if len(decisions) == 0 {
			return results, nil
		}
		filtered := make([]domain.SearchResult, 0, len(results))
		for index, result := range results {
			if index < len(decisions) && decisions[index].Allowed {
				filtered = append(filtered, result)
			}
		}
		return filtered, nil
	}

	if !authz.Allowed(subject.Roles, authz.ActionPageView) {
		return nil, ErrForbidden
	}
	return results, nil
}

func (f *ResultFilter) FilterBacklinks(ctx context.Context, subject AuthorizationSubject, payload domain.BacklinksPayload) (domain.BacklinksPayload, error) {
	if !subject.Authenticated {
		return payload, nil
	}
	references := make([]domain.BacklinkReference, 0, len(payload.Backlinks)+len(payload.RelatedPages))
	references = append(references, payload.Backlinks...)
	references = append(references, payload.RelatedPages...)
	if len(references) == 0 {
		return payload, nil
	}
	if f.checker != nil && f.checker.Enabled() {
		checks := make([]runtimeauthz.CheckInput, 0, len(references))
		for _, reference := range references {
			checks = append(checks, runtimeauthz.CheckInput{
				ActorUserID: subject.ActorUserID,
				WorkspaceID: subject.WorkspaceID,
				PageID:      reference.PageID,
				Action:      authz.ActionPageView,
			})
		}
		decisions, err := f.checker.BatchAuthorize(ctx, checks)
		if err != nil {
			return domain.BacklinksPayload{}, err
		}
		filteredBacklinks := make([]domain.BacklinkReference, 0, len(payload.Backlinks))
		filteredRelated := make([]domain.BacklinkReference, 0, len(payload.RelatedPages))
		offset := 0
		for _, reference := range payload.Backlinks {
			if offset < len(decisions) && decisions[offset].Allowed {
				filteredBacklinks = append(filteredBacklinks, reference)
			}
			offset++
		}
		for _, reference := range payload.RelatedPages {
			if offset < len(decisions) && decisions[offset].Allowed {
				filteredRelated = append(filteredRelated, reference)
			}
			offset++
		}
		payload.Backlinks = filteredBacklinks
		payload.RelatedPages = filteredRelated
		return payload, nil
	}

	if !authz.Allowed(subject.Roles, authz.ActionPageView) {
		return domain.BacklinksPayload{}, ErrForbidden
	}
	return payload, nil
}
