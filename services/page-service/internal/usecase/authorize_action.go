package usecase

import (
	"context"
	"errors"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

var ErrForbidden = errors.New("forbidden")

type AuthorizationSubject struct {
	ActorUserID   string
	WorkspaceID   string
	PageID        string
	Roles         []string
	Authenticated bool
}

type PageActionAuthorizer struct {
	checker *runtimeauthz.Client
}

func NewPageActionAuthorizer(checker *runtimeauthz.Client) *PageActionAuthorizer {
	return &PageActionAuthorizer{checker: checker}
}

func (a *PageActionAuthorizer) Allowed(ctx context.Context, action authz.Action, subject AuthorizationSubject) (bool, error) {
	if !subject.Authenticated {
		return true, nil
	}

	if a.checker != nil && a.checker.Enabled() {
		decision, err := a.checker.Authorize(ctx, runtimeauthz.CheckInput{
			ActorUserID: subject.ActorUserID,
			WorkspaceID: subject.WorkspaceID,
			PageID:      subject.PageID,
			Action:      action,
		})
		if err != nil {
			return false, err
		}
		return decision.Allowed, nil
	}

	return authz.Allowed(subject.Roles, action), nil
}

func (a *PageActionAuthorizer) Authorize(ctx context.Context, action authz.Action, subject AuthorizationSubject) error {
	allowed, err := a.Allowed(ctx, action, subject)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	return ErrForbidden
}

func (a *PageActionAuthorizer) AuthorizeEmbedUsage(ctx context.Context, document domain.Document, subject AuthorizationSubject) error {
	if !documentContainsEmbed(document) {
		return nil
	}
	return a.Authorize(ctx, authz.ActionPageEmbedTable, subject)
}

func documentContainsEmbed(document domain.Document) bool {
	for _, block := range document.Blocks {
		if block.Embed != nil && block.Embed.MWSTableID != "" {
			return true
		}
	}
	return false
}
