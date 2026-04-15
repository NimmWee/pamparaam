package domain

import "testing"

func TestEvaluateAuthorizationByRole(t *testing.T) {
	t.Parallel()

	decision := EvaluateAuthorization(RoleEditor, nil, ActionPagePublish)
	if !decision.Allowed {
		t.Fatalf("expected editor to publish page")
	}

	decision = EvaluateAuthorization(RoleViewer, nil, ActionPageEdit)
	if decision.Allowed {
		t.Fatalf("expected viewer edit to be denied")
	}
}

func TestEvaluateAuthorizationWithPageGrant(t *testing.T) {
	t.Parallel()

	decision := EvaluateAuthorization(RoleViewer, []PageGrant{
		{Permission: PagePermissionEdit},
	}, ActionPageEdit)
	if !decision.Allowed {
		t.Fatalf("expected edit grant to allow page edit")
	}
	if len(decision.EffectivePagePermissions) != 1 || decision.EffectivePagePermissions[0] != string(PagePermissionEdit) {
		t.Fatalf("unexpected effective permissions: %#v", decision.EffectivePagePermissions)
	}
}
