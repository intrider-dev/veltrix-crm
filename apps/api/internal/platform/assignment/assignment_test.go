package assignment

import (
	"errors"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestValidateRejectsDuplicateAndInvalidPrimaryAssignments(t *testing.T) {
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	tests := [][]Input{
		{{Kind: "watcher", SubjectType: "user", SubjectID: id, IsPrimary: true}},
		{
			{Kind: "responsible", SubjectType: "user", SubjectID: id, IsPrimary: true},
			{Kind: "responsible", SubjectType: "department", SubjectID: id, IsPrimary: true},
		},
		{
			{Kind: "watcher", SubjectType: "user", SubjectID: id},
			{Kind: "watcher", SubjectType: "user", SubjectID: id},
		},
	}
	for _, items := range tests {
		var validationError *errx.ValidationError
		if err := Validate(items); !errors.As(err, &validationError) {
			t.Fatalf("Validate(%v) error=%v, want validation error", items, err)
		}
	}
}
