package tenancy

import "testing"

func TestRolePermissionMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role       string
		permission Permission
		want       bool
	}{
		{"owner", PermissionSettingsWrite, true},
		{"admin", PermissionAuditRead, true},
		{"manager", PermissionReportsRead, true},
		{"manager", PermissionSettingsWrite, false},
		{"manager", PermissionMembersRead, true},
		{"manager", PermissionMembersWrite, false},
		{"sales", PermissionRecordsCreate, true},
		{"sales", PermissionRecordsDelete, false},
		{"sales", PermissionDataExport, false},
		{"viewer", PermissionRecordsRead, true},
		{"viewer", PermissionRecordsUpdate, false},
		{"viewer", PermissionMembersRead, false},
		{"owner", Permission("unknown.permission"), false},
		{"unknown", PermissionRecordsRead, false},
	}
	for _, test := range tests {
		if got := RoleAllows(test.role, test.permission); got != test.want {
			t.Errorf("RoleAllows(%q, %q) = %v, want %v", test.role, test.permission, got, test.want)
		}
	}
}
