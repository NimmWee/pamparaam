package usecase

import (
	"context"
	"errors"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
)

var ErrForbidden = errors.New("forbidden")

type AuthorizationSubject struct {
	ActorUserID   string
	WorkspaceID   string
	PageID        string
	Roles         []string
	Authenticated bool
}

type FileActionAuthorizer struct {
	checker *runtimeauthz.Client
}

func NewFileActionAuthorizer(checker *runtimeauthz.Client) *FileActionAuthorizer {
	return &FileActionAuthorizer{checker: checker}
}

func (a *FileActionAuthorizer) Authorize(ctx context.Context, action authz.Action, subject AuthorizationSubject) error {
	if !subject.Authenticated {
		return nil
	}

	if a.checker != nil && a.checker.Enabled() {
		decision, err := a.checker.Authorize(ctx, runtimeauthz.CheckInput{
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
