package domain

import (
	"slices"
	"time"
)

type WorkspaceRole string

const (
	RoleOwner     WorkspaceRole = "owner"
	RoleAdmin     WorkspaceRole = "admin"
	RoleEditor    WorkspaceRole = "editor"
	RoleCommenter WorkspaceRole = "commenter"
	RoleViewer    WorkspaceRole = "viewer"
	RoleUnknown   WorkspaceRole = ""
)

type Action string

const (
	ActionPageView       Action = "page.view"
	ActionPageEdit       Action = "page.edit"
	ActionPageArchive    Action = "page.archive"
	ActionPagePublish    Action = "page.publish"
	ActionPageRestore    Action = "page.restore"
	ActionPageEmbedTable Action = "page.embed_table"
	ActionFileUpload     Action = "file.upload"
	ActionFileRead       Action = "file.read"
	ActionSearchQuery    Action = "search.query"
)

type PagePermission string

const (
	PagePermissionView PagePermission = "view"
	PagePermissionEdit PagePermission = "edit"
)

type Workspace struct {
	ID   string
	Name string
	Slug string
}

type User struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	Status       string
}

type Membership struct {
	WorkspaceID string
	UserID      string
	Role        WorkspaceRole
}

type PageGrant struct {
	PageID        string
	WorkspaceID   string
	SubjectUserID string
	Permission    PagePermission
}

type RefreshSession struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	UserAgent string
	ClientIP  string
}

type AuthorizationDecision struct {
	Allowed                  bool
	EffectiveWorkspaceRole   WorkspaceRole
	EffectivePagePermissions []string
	DenialReason             string
}

func (r WorkspaceRole) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleEditor, RoleCommenter, RoleViewer:
		return true
	default:
		return false
	}
}

func (r WorkspaceRole) permits(action Action) bool {
	switch r {
	case RoleOwner, RoleAdmin:
		return true
	case RoleEditor:
		return action == ActionPageView ||
			action == ActionPageEdit ||
			action == ActionPageArchive ||
			action == ActionPagePublish ||
			action == ActionPageRestore ||
			action == ActionPageEmbedTable ||
			action == ActionFileUpload ||
			action == ActionFileRead ||
			action == ActionSearchQuery
	case RoleCommenter:
		return action == ActionPageView ||
			action == ActionFileRead ||
			action == ActionSearchQuery
	case RoleViewer:
		return action == ActionPageView ||
			action == ActionFileRead ||
			action == ActionSearchQuery
	default:
		return false
	}
}

func EvaluateAuthorization(role WorkspaceRole, grants []PageGrant, action Action) AuthorizationDecision {
	decision := AuthorizationDecision{
		EffectiveWorkspaceRole: role,
	}

	for _, grant := range grants {
		decision.EffectivePagePermissions = append(decision.EffectivePagePermissions, string(grant.Permission))
	}

	if role.permits(action) {
		decision.Allowed = true
		return decision
	}

	if action == ActionPageView && hasGrant(grants, PagePermissionView, PagePermissionEdit) {
		decision.Allowed = true
		return decision
	}

	if (action == ActionPageEdit || action == ActionPageArchive || action == ActionPageEmbedTable || action == ActionFileUpload) && hasGrant(grants, PagePermissionEdit) {
		decision.Allowed = true
		return decision
	}

	decision.DenialReason = "permission_denied"
	return decision
}

func hasGrant(grants []PageGrant, expected ...PagePermission) bool {
	for _, grant := range grants {
		if slices.Contains(expected, grant.Permission) {
			return true
		}
	}

	return false
}
