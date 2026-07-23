package customers

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestValidateTypedValuesAcceptsEverySupportedType(t *testing.T) {
	t.Parallel()
	minimum := 1.0
	maximum := 10.0
	definitions := []CustomFieldDefinition{
		definition("note", "text", CustomFieldValidation{Required: true, MinLength: intPointer(2), MaxLength: intPointer(10)}, nil),
		definition("description", "multiline_text", CustomFieldValidation{MaxLength: intPointer(100)}, nil),
		definition("score", "number", CustomFieldValidation{Minimum: &minimum, Maximum: &maximum}, nil),
		definition("budget", "money", CustomFieldValidation{}, nil),
		definition("renewal", "date", CustomFieldValidation{}, nil),
		definition("vip", "boolean", CustomFieldValidation{}, nil),
		definition("segment", "single_select", CustomFieldValidation{}, []CustomFieldOption{{Value: "smb", Label: "SMB"}}),
		definition("regions", "multi_select", CustomFieldValidation{}, []CustomFieldOption{{Value: "emea", Label: "EMEA"}, {Value: "apac", Label: "APAC"}}),
		definition("sponsor", "user_reference", CustomFieldValidation{}, nil),
	}
	values := map[string]any{
		"note": "ready", "description": "First line\nSecond line", "score": 7.5, "budget": map[string]any{"minor": 1250, "currency": "USD"},
		"renewal": "2027-01-31", "vip": true, "segment": "smb", "regions": []string{"emea", "apac"},
		"sponsor": "01982d57-3400-7000-8000-000000000001",
	}
	validated, err := validateTypedValues(definitions, values)
	if err != nil {
		t.Fatalf("validateTypedValues() error = %v", err)
	}
	if len(validated) != len(values) {
		t.Fatalf("validated %d values, want %d", len(validated), len(values))
	}
	var money map[string]any
	if err := json.Unmarshal(validated["budget"], &money); err != nil || money["currency"] != "USD" {
		t.Fatalf("money was not preserved: %s, %v", validated["budget"], err)
	}
}

func TestValidateTypedValuesRejectsUnknownMissingAndMalformedValues(t *testing.T) {
	t.Parallel()
	definitions := []CustomFieldDefinition{
		definition("required_note", "text", CustomFieldValidation{Required: true}, nil),
		definition("budget", "money", CustomFieldValidation{}, nil),
		definition("segment", "single_select", CustomFieldValidation{}, []CustomFieldOption{{Value: "smb", Label: "SMB"}}),
	}
	_, err := validateTypedValues(definitions, map[string]any{
		"unknown": "value", "budget": map[string]any{"minor": 12.5, "currency": "usd"}, "segment": "enterprise",
	})
	var validation *errx.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
	wanted := map[string]bool{
		"validation.custom_field.unknown": false,
		"validation.custom_field.money":   false,
		"validation.custom_field.option":  false,
		"validation.required":             false,
	}
	for _, field := range validation.Fields {
		if _, exists := wanted[field.Code]; exists {
			wanted[field.Code] = true
		}
	}
	for code, found := range wanted {
		if !found {
			t.Errorf("missing validation code %q in %#v", code, validation.Fields)
		}
	}
}

func TestValidateDefinitionRequiresStableSelectOptions(t *testing.T) {
	t.Parallel()
	_, err := validateDefinitionInput(CustomFieldDefinitionInput{
		EntityType: "contact", FieldKey: "segment", Label: "Segment", ValueType: "single_select",
		Options: []CustomFieldOption{{Value: "smb", Label: "SMB"}, {Value: "smb", Label: "Duplicate"}},
	})
	if err == nil {
		t.Fatal("duplicate option value was accepted")
	}
	validated, err := validateDefinitionInput(CustomFieldDefinitionInput{
		EntityType: "contact", FieldKey: "segment", Label: "Segment", ValueType: "single_select",
		Options: []CustomFieldOption{{Value: "smb", Label: "Small business"}},
	})
	if err != nil || validated.Options[0].Value != "smb" {
		t.Fatalf("valid definition rejected: %#v, %v", validated, err)
	}
}

func TestValidateVersionedRecordsIsBoundedAndRejectsDuplicates(t *testing.T) {
	t.Parallel()
	id := ids.MustParse("01982d57-3400-7000-8000-000000000001")
	if err := validateVersionedRecords([]VersionedID{{ID: id, Version: 1}, {ID: id, Version: 2}}); err == nil {
		t.Fatal("duplicate record was accepted")
	}
	records := make([]VersionedID, MaxBulkRecords+1)
	for index := range records {
		records[index] = VersionedID{ID: syntheticID(index), Version: 1}
	}
	if err := validateVersionedRecords(records); err == nil {
		t.Fatal("oversized bulk request was accepted")
	}
}

func TestValidateSavedViewRejectsUnboundedOrUnsafeDefinition(t *testing.T) {
	t.Parallel()
	_, err := validateSavedView(SavedViewInput{
		EntityType: "contact", Name: "Unsafe",
		Definition: SavedViewDefinition{Filters: []SavedViewFilter{{Field: "DROP TABLE", Operator: "eq", Value: json.RawMessage(`"x"`)}}},
	})
	if err == nil {
		t.Fatal("unsafe field was accepted")
	}
	view, err := validateSavedView(SavedViewInput{
		EntityType: "contact", Name: " Hot leads ",
		Definition: SavedViewDefinition{
			Filters: []SavedViewFilter{{Field: "status", Operator: "eq", Value: json.RawMessage(`"active"`)}},
			Sort:    []SavedViewSort{{Field: "updatedAt", Direction: "desc"}}, Columns: []string{"displayName", "email", "custom.segment"},
		},
	})
	if err != nil || view.Name != "Hot leads" {
		t.Fatalf("valid view rejected: %#v, %v", view, err)
	}
}

func definition(key, valueType string, validation CustomFieldValidation, options []CustomFieldOption) CustomFieldDefinition {
	return CustomFieldDefinition{FieldKey: key, ValueType: valueType, Validation: validation, Options: options}
}

func intPointer(value int) *int { return &value }

func syntheticID(index int) ids.UUID {
	var id ids.UUID
	id[0] = 1
	id[14] = byte(index >> 8)
	id[15] = byte(index)
	return id
}
