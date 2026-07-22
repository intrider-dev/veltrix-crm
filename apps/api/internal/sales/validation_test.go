package sales

import (
	"errors"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestValidateLeadInputNormalizesEmailAndPhone(t *testing.T) {
	email := " Sales@Example.COM "
	phone := "+1 (202) 555-0142"
	validated, normalizedEmail, normalizedPhone, custom, err := validateLeadInput(LeadInput{
		Name: "  Morgan Lee ", Email: &email, Phone: &phone, Status: "qualified",
	})
	if err != nil {
		t.Fatal(err)
	}
	if validated.Name != "Morgan Lee" || normalizedEmail != "sales@example.com" || normalizedPhone != "+12025550142" {
		t.Fatalf("unexpected normalized lead: %#v %q %q", validated, normalizedEmail, normalizedPhone)
	}
	if string(custom) != "{}" {
		t.Fatalf("unexpected custom fields: %s", custom)
	}
}

func TestValidateDealOutcomeRequiresLostReason(t *testing.T) {
	_, err := validateDealOutcome(DealOutcomeInput{Status: "lost"})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) || validationError.Fields[0].Pointer != "/lostReason" {
		t.Fatalf("expected lost reason validation, got %v", err)
	}

	reason := "Budget"
	validated, err := validateDealOutcome(DealOutcomeInput{Status: "lost", LostReason: &reason})
	if err != nil {
		t.Fatal(err)
	}
	if validated.ForecastCategory != "closed" {
		t.Fatalf("expected closed forecast, got %q", validated.ForecastCategory)
	}
}

func TestValidateDealUpdateRejectsInvertedPlanningWindow(t *testing.T) {
	pipelineID, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	stageID, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	closeDate := start.AddDate(0, 0, -1)
	_, _, err = validateDealUpdate(DealUpdateInput{
		Name: "Renewal", PipelineID: pipelineID, StageID: stageID,
		Currency: "USD", ForecastCategory: "pipeline", CustomFields: map[string]any{},
		PlannedStartDate: &start, ExpectedCloseDate: &closeDate,
	})
	var validationError *errx.ValidationError
	if !errors.As(err, &validationError) || validationError.Fields[0].Pointer != "/plannedStartDate" {
		t.Fatalf("expected planning date range validation, got %v", err)
	}
}

func TestValidateStageOrderRejectsDuplicate(t *testing.T) {
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	err = validateStageOrder([]StageOrderItem{{ID: id, Version: 1}, {ID: id, Version: 1}})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestKanbanCursorIsFilterBound(t *testing.T) {
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := encodeKanbanCursor(42, id, "pipeline=a&stage=b")
	if err != nil {
		t.Fatal(err)
	}
	position, decoded, err := decodeKanbanCursor(cursor, "pipeline=a&stage=b")
	if err != nil || position != 42 || decoded != id {
		t.Fatalf("unexpected round trip: %d %s %v", position, decoded, err)
	}
	if _, _, err := decodeKanbanCursor(cursor, "pipeline=a&stage=c"); err == nil {
		t.Fatal("cursor unexpectedly accepted for another stage")
	}
}

func TestValidateLineItemRejectsZeroAndExcessPrecision(t *testing.T) {
	for _, quantity := range []string{"0", "0.0000", "1.00001"} {
		_, err := validateLineItem(LineItemInput{Name: "Seat", Quantity: quantity, Currency: "USD"})
		if err == nil {
			t.Fatalf("expected %q to be rejected", quantity)
		}
	}
	if _, err := validateLineItem(LineItemInput{Name: "Seat", Quantity: "1.2500", Currency: "usd", Position: 1}); err != nil {
		t.Fatal(err)
	}
}
