package app

import (
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func TestAttachmentPermissionUsesDealSpecificMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		entityType string
		operation  attachmentOperation
		want       tenancy.Permission
	}{
		{"deal", attachmentRead, tenancy.PermissionDealsRead},
		{"deal", attachmentWrite, tenancy.PermissionDealsUpdate},
		{"deal", attachmentDelete, tenancy.PermissionDealsDelete},
		{"  deal  ", attachmentRead, tenancy.PermissionDealsRead},
		{"contact", attachmentRead, tenancy.PermissionRecordsRead},
		{"company", attachmentWrite, tenancy.PermissionRecordsCreate},
		{"activity", attachmentDelete, tenancy.PermissionRecordsDelete},
	}
	for _, test := range tests {
		if got := attachmentPermission(test.entityType, test.operation); got != test.want {
			t.Errorf("attachmentPermission(%q, %d)=%q, want %q",
				test.entityType, test.operation, got, test.want)
		}
	}
}
