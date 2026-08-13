package projects

import (
	"errors"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

func TestValidateProjectPlanningWindow(t *testing.T) {
	start := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, -2)
	_, err := validate(Input{Name: "Launch", Status: "active", Visibility: "workspace",
		PlannedStartDate: &start, TargetEndDate: &end})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) || validationError.Fields[0].Pointer != "/plannedStartDate" {
		t.Fatalf("expected date range validation, got %v", err)
	}
}

func TestValidateProjectEnums(t *testing.T) {
	for _, input := range []Input{
		{Name: "Launch", Status: "unknown", Visibility: "workspace"},
		{Name: "Launch", Status: "active", Visibility: "public"},
	} {
		if _, err := validate(input); err == nil {
			t.Fatalf("expected invalid input to fail: %#v", input)
		}
	}
}
