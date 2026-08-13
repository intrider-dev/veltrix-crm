package app

import (
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestCustomFieldReadPermissionsDoNotBroadenSplitRoles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		entityType string
		want       tenancy.Permission
	}{
		{"contact", tenancy.PermissionRecordsRead},
		{"company", tenancy.PermissionRecordsRead},
		{"lead", tenancy.PermissionLeadsRead},
		{"deal", tenancy.PermissionDealsRead},
		{"", tenancy.PermissionSettingsWrite},
	}
	for _, test := range tests {
		permissions, err := customFieldReadPermissions(test.entityType)
		if err != nil {
			t.Fatalf("customFieldReadPermissions(%q): %v", test.entityType, err)
		}
		if len(permissions) != 1 || permissions[0] != test.want {
			t.Errorf("customFieldReadPermissions(%q)=%v, want only %q",
				test.entityType, permissions, test.want)
		}
	}
	if _, err := customFieldReadPermissions("unknown"); err == nil {
		t.Fatal("unknown entity type must be rejected before opening a tenant transaction")
	}
}
