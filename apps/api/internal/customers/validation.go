package customers

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

var (
	fieldKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)
	colorPattern    = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

var customFieldTypes = map[string]struct{}{
	"text": {}, "multiline_text": {}, "number": {}, "money": {}, "date": {}, "boolean": {},
	"single_select": {}, "multi_select": {}, "user_reference": {},
}

func validateTagInput(input TagInput) (TagInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Color = strings.TrimSpace(input.Color)
	if input.Color == "" {
		input.Color = "#64748b"
	}
	fields := make([]errx.FieldError, 0, 2)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 80 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if !colorPattern.MatchString(input.Color) {
		fields = append(fields, errx.FieldError{Pointer: "/color", Code: "validation.color.invalid"})
	}
	if len(fields) > 0 {
		return TagInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validateCompanyUpdate(input CompanyUpdateInput) (CompanyUpdateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Domain = trimPointer(input.Domain)
	input.Industry = trimPointer(input.Industry)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "active"
	}
	fields := make([]errx.FieldError, 0, 5)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if input.Domain != nil && utf8.RuneCountInString(*input.Domain) > 253 {
		fields = append(fields, errx.FieldError{Pointer: "/domain", Code: "validation.length"})
	}
	if input.Industry != nil && utf8.RuneCountInString(*input.Industry) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/industry", Code: "validation.length"})
	}
	if input.Status != "active" && input.Status != "inactive" {
		fields = append(fields, errx.FieldError{Pointer: "/status", Code: "validation.enum"})
	}
	address, marshalErr := json.Marshal(input.Address)
	if marshalErr != nil || len(address) > 8192 {
		fields = append(fields, errx.FieldError{Pointer: "/address", Code: "validation.address.invalid"})
	}
	for key, value := range input.Address {
		if len(key) > 80 || len(value) > 500 {
			fields = append(fields, errx.FieldError{Pointer: "/address", Code: "validation.address.invalid"})
			break
		}
	}
	if len(fields) > 0 {
		return CompanyUpdateInput{}, &errx.ValidationError{Fields: fields}
	}
	if input.Address == nil {
		input.Address = map[string]string{}
	}
	if input.CustomFields == nil {
		input.CustomFields = map[string]any{}
	}
	return input, nil
}

func validateDefinitionInput(input CustomFieldDefinitionInput) (CustomFieldDefinitionInput, error) {
	input.EntityType = strings.TrimSpace(input.EntityType)
	input.FieldKey = strings.TrimSpace(input.FieldKey)
	input.Label = strings.TrimSpace(input.Label)
	input.ValueType = strings.TrimSpace(input.ValueType)
	fields := make([]errx.FieldError, 0, 6)
	if !allowedEntityType(input.EntityType) {
		fields = append(fields, errx.FieldError{Pointer: "/entityType", Code: "validation.enum"})
	}
	if !fieldKeyPattern.MatchString(input.FieldKey) {
		fields = append(fields, errx.FieldError{Pointer: "/fieldKey", Code: "validation.format"})
	}
	if utf8.RuneCountInString(input.Label) < 1 || utf8.RuneCountInString(input.Label) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/label", Code: "validation.length"})
	}
	if _, ok := customFieldTypes[input.ValueType]; !ok {
		fields = append(fields, errx.FieldError{Pointer: "/valueType", Code: "validation.enum"})
	}
	validation := input.Validation
	if validation.MinLength != nil && (*validation.MinLength < 0 || *validation.MinLength > 4096) {
		fields = append(fields, errx.FieldError{Pointer: "/validation/minLength", Code: "validation.range"})
	}
	if validation.MaxLength != nil && (*validation.MaxLength < 1 || *validation.MaxLength > 4096) {
		fields = append(fields, errx.FieldError{Pointer: "/validation/maxLength", Code: "validation.range"})
	}
	if validation.MinLength != nil && validation.MaxLength != nil && *validation.MinLength > *validation.MaxLength {
		fields = append(fields, errx.FieldError{Pointer: "/validation", Code: "validation.range"})
	}
	if validation.Minimum != nil && (!isFinite(*validation.Minimum)) {
		fields = append(fields, errx.FieldError{Pointer: "/validation/minimum", Code: "validation.number.invalid"})
	}
	if validation.Maximum != nil && (!isFinite(*validation.Maximum)) {
		fields = append(fields, errx.FieldError{Pointer: "/validation/maximum", Code: "validation.number.invalid"})
	}
	if validation.Minimum != nil && validation.Maximum != nil && *validation.Minimum > *validation.Maximum {
		fields = append(fields, errx.FieldError{Pointer: "/validation", Code: "validation.range"})
	}
	if input.ValueType != "single_select" && input.ValueType != "multi_select" && len(input.Options) > 0 {
		fields = append(fields, errx.FieldError{Pointer: "/options", Code: "validation.not_allowed"})
	}
	if (input.ValueType == "single_select" || input.ValueType == "multi_select") && len(input.Options) == 0 {
		fields = append(fields, errx.FieldError{Pointer: "/options", Code: "validation.required"})
	}
	if len(input.Options) > 100 {
		fields = append(fields, errx.FieldError{Pointer: "/options", Code: "validation.max_items"})
	}
	seenOptions := make(map[string]struct{}, len(input.Options))
	for index := range input.Options {
		input.Options[index].Value = strings.TrimSpace(input.Options[index].Value)
		input.Options[index].Label = strings.TrimSpace(input.Options[index].Label)
		option := input.Options[index]
		if option.Value == "" || len(option.Value) > 80 || option.Label == "" || utf8.RuneCountInString(option.Label) > 120 {
			fields = append(fields, errx.FieldError{Pointer: "/options", Code: "validation.option.invalid"})
			break
		}
		if _, exists := seenOptions[option.Value]; exists {
			fields = append(fields, errx.FieldError{Pointer: "/options", Code: "validation.duplicate"})
			break
		}
		seenOptions[option.Value] = struct{}{}
	}
	if len(fields) > 0 {
		return CustomFieldDefinitionInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validateTypedValues(definitions []CustomFieldDefinition, supplied map[string]any) (map[string]json.RawMessage, error) {
	byKey := make(map[string]CustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		byKey[definition.FieldKey] = definition
	}
	result := make(map[string]json.RawMessage, len(supplied))
	fields := make([]errx.FieldError, 0)
	for key, value := range supplied {
		definition, exists := byKey[key]
		if !exists {
			fields = append(fields, errx.FieldError{Pointer: "/customFields/" + key, Code: "validation.custom_field.unknown"})
			continue
		}
		raw, err := canonicalJSON(value)
		if err != nil || len(raw) > 65536 {
			fields = append(fields, errx.FieldError{Pointer: "/customFields/" + key, Code: "validation.custom_field.invalid"})
			continue
		}
		if bytes.Equal(raw, []byte("null")) {
			continue
		}
		if code := validateTypedValue(definition, raw); code != "" {
			fields = append(fields, errx.FieldError{Pointer: "/customFields/" + key, Code: code})
			continue
		}
		result[key] = raw
	}
	for _, definition := range definitions {
		if definition.Validation.Required {
			if _, exists := result[definition.FieldKey]; !exists {
				fields = append(fields, errx.FieldError{Pointer: "/customFields/" + definition.FieldKey, Code: "validation.required"})
			}
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > 65536 {
		fields = append(fields, errx.FieldError{Pointer: "/customFields", Code: "validation.custom_fields.invalid"})
	}
	if len(fields) > 0 {
		return nil, &errx.ValidationError{Fields: fields}
	}
	return result, nil
}

func validateTypedValue(definition CustomFieldDefinition, raw json.RawMessage) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return "validation.custom_field.invalid"
	}
	switch definition.ValueType {
	case "text", "multiline_text":
		text, ok := value.(string)
		if !ok || utf8.RuneCountInString(text) > 4096 {
			return "validation.custom_field.text"
		}
		length := utf8.RuneCountInString(text)
		if definition.Validation.MinLength != nil && length < *definition.Validation.MinLength {
			return "validation.custom_field.min_length"
		}
		if definition.Validation.MaxLength != nil && length > *definition.Validation.MaxLength {
			return "validation.custom_field.max_length"
		}
	case "number":
		number, ok := jsonNumber(value)
		if !ok || !numberWithin(number, definition.Validation) {
			return "validation.custom_field.number"
		}
	case "money":
		object, ok := value.(map[string]any)
		if !ok || len(object) != 2 {
			return "validation.custom_field.money"
		}
		minor, ok := jsonNumber(object["minor"])
		currency, currencyOK := object["currency"].(string)
		if !ok || math.Trunc(minor) != minor || math.Abs(minor) > 9_007_199_254_740_991 || !currencyOK || !currencyPattern.MatchString(currency) {
			return "validation.custom_field.money"
		}
	case "date":
		value, ok := value.(string)
		if !ok {
			return "validation.custom_field.date"
		}
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil || parsed.Format("2006-01-02") != value {
			return "validation.custom_field.date"
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return "validation.custom_field.boolean"
		}
	case "single_select":
		option, ok := value.(string)
		if !ok || !definitionHasOption(definition, option) {
			return "validation.custom_field.option"
		}
	case "multi_select":
		values, ok := value.([]any)
		if !ok || len(values) > 100 {
			return "validation.custom_field.options"
		}
		seen := make(map[string]struct{}, len(values))
		for _, rawOption := range values {
			option, ok := rawOption.(string)
			if !ok || !definitionHasOption(definition, option) {
				return "validation.custom_field.options"
			}
			if _, exists := seen[option]; exists {
				return "validation.custom_field.options"
			}
			seen[option] = struct{}{}
		}
	case "user_reference":
		value, ok := value.(string)
		if !ok {
			return "validation.custom_field.user_reference"
		}
		if _, err := ids.Parse(value); err != nil {
			return "validation.custom_field.user_reference"
		}
	default:
		return "validation.custom_field.invalid"
	}
	return ""
}

func validateSavedView(input SavedViewInput) (SavedViewInput, error) {
	input.EntityType = strings.TrimSpace(input.EntityType)
	input.Name = strings.TrimSpace(input.Name)
	fields := make([]errx.FieldError, 0)
	if !allowedEntityType(input.EntityType) && input.EntityType != "activity" {
		fields = append(fields, errx.FieldError{Pointer: "/entityType", Code: "validation.enum"})
	}
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if len(input.Definition.Filters) > 20 || len(input.Definition.Sort) > 5 || len(input.Definition.Columns) > 30 {
		fields = append(fields, errx.FieldError{Pointer: "/definition", Code: "validation.max_items"})
	}
	operators := map[string]struct{}{"eq": {}, "neq": {}, "contains": {}, "starts_with": {}, "gt": {}, "gte": {}, "lt": {}, "lte": {}, "in": {}, "is_empty": {}}
	for index, filter := range input.Definition.Filters {
		if !safeViewField(filter.Field) {
			fields = append(fields, errx.FieldError{Pointer: "/definition/filters", Code: "validation.field.invalid"})
			break
		}
		if _, ok := operators[filter.Operator]; !ok || len(filter.Value) > 8192 {
			fields = append(fields, errx.FieldError{Pointer: "/definition/filters", Code: "validation.operator.invalid", Params: map[string]any{"index": index}})
			break
		}
	}
	for _, sort := range input.Definition.Sort {
		if !safeViewField(sort.Field) || (sort.Direction != "asc" && sort.Direction != "desc") {
			fields = append(fields, errx.FieldError{Pointer: "/definition/sort", Code: "validation.sort.invalid"})
			break
		}
	}
	seen := make(map[string]struct{}, len(input.Definition.Columns))
	for _, column := range input.Definition.Columns {
		if !safeViewField(column) {
			fields = append(fields, errx.FieldError{Pointer: "/definition/columns", Code: "validation.field.invalid"})
			break
		}
		if _, exists := seen[column]; exists {
			fields = append(fields, errx.FieldError{Pointer: "/definition/columns", Code: "validation.duplicate"})
			break
		}
		seen[column] = struct{}{}
	}
	encoded, err := json.Marshal(input.Definition)
	if err != nil || len(encoded) > 65536 {
		fields = append(fields, errx.FieldError{Pointer: "/definition", Code: "validation.saved_view.invalid"})
	}
	if len(fields) > 0 {
		return SavedViewInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validateVersionedRecords(records []VersionedID) error {
	if len(records) < 1 || len(records) > MaxBulkRecords {
		return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/records", Code: "validation.items.range", Params: map[string]any{"max": MaxBulkRecords}}}}
	}
	seen := make(map[ids.UUID]struct{}, len(records))
	for _, record := range records {
		if record.ID == (ids.UUID{}) || record.Version < 1 {
			return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/records", Code: "validation.record.invalid"}}}
		}
		if _, exists := seen[record.ID]; exists {
			return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/records", Code: "validation.duplicate"}}}
		}
		seen[record.ID] = struct{}{}
	}
	return nil
}

func canonicalJSON(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var canonical any
	if err := decoder.Decode(&canonical); err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

func jsonNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil && isFinite(parsed)
	case float64:
		return number, isFinite(number)
	default:
		return 0, false
	}
}

func numberWithin(number float64, validation CustomFieldValidation) bool {
	if !isFinite(number) {
		return false
	}
	if validation.Minimum != nil && number < *validation.Minimum {
		return false
	}
	return validation.Maximum == nil || number <= *validation.Maximum
}

func definitionHasOption(definition CustomFieldDefinition, value string) bool {
	for _, option := range definition.Options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func allowedEntityType(value string) bool {
	switch value {
	case "contact", "company", "lead", "deal":
		return true
	default:
		return false
	}
}

func safeViewField(value string) bool {
	if strings.HasPrefix(value, "custom.") {
		return fieldKeyPattern.MatchString(strings.TrimPrefix(value, "custom."))
	}
	switch value {
	case "displayName", "name", "email", "phone", "domain", "status", "source", "ownerId", "teamId", "companyId", "amountMinor", "currency", "expectedCloseDate", "dueAt", "priority", "createdAt", "updatedAt", "tags":
		return true
	default:
		return false
	}
}

func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
