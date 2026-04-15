package authz

import "slices"

type Role string
type Action string

const (
	RoleOwner     Role = "owner"
	RoleAdmin     Role = "admin"
	RoleEditor    Role = "editor"
	RoleCommenter Role = "commenter"
	RoleViewer    Role = "viewer"

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

func Allowed(roles []string, action Action) bool {
	for _, rawRole := range roles {
		role := Role(rawRole)
		switch role {
		case RoleOwner, RoleAdmin:
			return true
		case RoleEditor:
			if slices.Contains([]Action{
				ActionPageView,
				ActionPageEdit,
				ActionPageArchive,
				ActionPagePublish,
				ActionPageRestore,
				ActionPageEmbedTable,
				ActionFileUpload,
				ActionFileRead,
				ActionSearchQuery,
			}, action) {
				return true
			}
		case RoleCommenter, RoleViewer:
			if slices.Contains([]Action{
				ActionPageView,
				ActionFileRead,
				ActionSearchQuery,
			}, action) {
				return true
			}
		}
	}
	return false
}
