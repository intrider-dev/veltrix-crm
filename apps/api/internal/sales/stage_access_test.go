package sales

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestValidateStageRoleAccess(t *testing.T) {
	first := mustStageAccessTestID(t)
	second := mustStageAccessTestID(t)
	tests := []struct {
		name  string
		rules []StageRoleAccessInput
		code  string
	}{
		{name: "empty is unrestricted", rules: nil},
		{name: "independent flags", rules: []StageRoleAccessInput{{RoleID: first, CanEnter: true}}},
		{name: "multiple roles", rules: []StageRoleAccessInput{{RoleID: first}, {RoleID: second, CanView: true}}},
		{name: "missing role", rules: []StageRoleAccessInput{{}}, code: "validation.required"},
		{name: "duplicate role", rules: []StageRoleAccessInput{{RoleID: first}, {RoleID: first}}, code: "validation.duplicate"},
		{name: "bounded", rules: makeStageAccessRules(t, MaxStageRoleAccessRules+1), code: "validation.max_items"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateStageRoleAccess(test.rules)
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validationError *errx.ValidationError
			if !errors.As(err, &validationError) || len(validationError.Fields) != 1 || validationError.Fields[0].Code != test.code {
				t.Fatalf("error=%v, want validation code %s", err, test.code)
			}
		})
	}
}

func TestStageAccessActionValidation(t *testing.T) {
	for _, action := range []StageAccessAction{StageAccessView, StageAccessEnter, StageAccessLeave} {
		if !validStageAccessAction(action) {
			t.Fatalf("expected %q to be valid", action)
		}
	}
	if validStageAccessAction("delete") {
		t.Fatal("unexpected delete action support")
	}
}

func TestMapStageAccessConstraint(t *testing.T) {
	err := mapStageAccessConstraint(&pgconn.PgError{Code: "23503"})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) || validationError.Fields[0].Code != "validation.reference.invalid" {
		t.Fatalf("error=%v, want invalid reference validation", err)
	}
	original := errors.New("database unavailable")
	if mapped := mapStageAccessConstraint(original); !errors.Is(mapped, original) {
		t.Fatalf("mapped error=%v, want wrapped original", mapped)
	}
}

func makeStageAccessRules(t *testing.T, count int) []StageRoleAccessInput {
	t.Helper()
	rules := make([]StageRoleAccessInput, count)
	for index := range rules {
		rules[index].RoleID = mustStageAccessTestID(t)
	}
	return rules
}

func mustStageAccessTestID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
