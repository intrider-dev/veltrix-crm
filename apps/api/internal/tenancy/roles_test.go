package tenancy

import "testing"

func TestValidateWorkspaceRoleInputEnforcesBaseRoleCeiling(t *testing.T) {
	t.Parallel()
	valid, err := validateWorkspaceRoleInput(WorkspaceRoleInput{
		Name: "  Regional sales  ", BaseRole: "sales",
		Permissions: []Permission{PermissionRecordsUpdate, PermissionRecordsRead, PermissionRecordsRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	if valid.Name != "Regional sales" || len(valid.Permissions) != 2 ||
		valid.Permissions[0] != PermissionRecordsRead || valid.Permissions[1] != PermissionRecordsUpdate {
		t.Fatalf("unexpected normalized role: %+v", valid)
	}

	for _, input := range []WorkspaceRoleInput{
		{Name: "Owner clone", BaseRole: "owner", Permissions: []Permission{PermissionRecordsRead}},
		{Name: "Escalated sales", BaseRole: "sales", Permissions: []Permission{PermissionSettingsWrite}},
		{Name: "Role manager", BaseRole: "admin", Permissions: []Permission{PermissionRolesWrite}},
		{Name: "Unknown grant", BaseRole: "admin", Permissions: []Permission{"records.superuser"}},
	} {
		if _, err := validateWorkspaceRoleInput(input); err == nil {
			t.Fatalf("validateWorkspaceRoleInput(%+v) succeeded", input)
		}
	}
}
