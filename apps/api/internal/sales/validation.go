package sales

import (
	"encoding/json"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

var (
	currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)
	quantityPattern     = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,9})(?:\.[0-9]{1,4})?$`)
)

func validatePipelineInput(input PipelineInput) (PipelineInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 120 {
		return PipelineInput{}, validation("/name", "validation.length")
	}
	return input, nil
}

func validateStageInput(input PipelineStageInput) (PipelineStageInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ForecastCategory = strings.TrimSpace(input.ForecastCategory)
	fields := make([]errx.FieldError, 0, 3)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 100 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if input.Probability < 0 || input.Probability > 100 {
		fields = append(fields, errx.FieldError{Pointer: "/probability", Code: "validation.range"})
	}
	if !validForecastCategory(input.ForecastCategory) {
		fields = append(fields, errx.FieldError{Pointer: "/forecastCategory", Code: "validation.enum"})
	}
	if len(fields) > 0 {
		return PipelineStageInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validateStageOrder(order []StageOrderItem) error {
	if len(order) < 1 || len(order) > MaxPipelineStages {
		return validation("/stages", "validation.items.range")
	}
	seen := make(map[[16]byte]struct{}, len(order))
	for _, stage := range order {
		if stage.ID == ([16]byte{}) || stage.Version < 1 {
			return validation("/stages", "validation.record.invalid")
		}
		if _, exists := seen[stage.ID]; exists {
			return validation("/stages", "validation.duplicate")
		}
		seen[stage.ID] = struct{}{}
	}
	return nil
}

func validateLeadInput(input LeadInput) (LeadInput, string, string, []byte, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.CompanyName = cleanOptional(input.CompanyName)
	input.JobTitle = cleanOptional(input.JobTitle)
	input.Source = cleanOptional(input.Source)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "new"
	}
	fields := make([]errx.FieldError, 0, 9)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	emailNormalized := ""
	input.Email = cleanOptional(input.Email)
	if input.Email != nil {
		parsed, err := mail.ParseAddress(*input.Email)
		if err != nil || parsed.Address != *input.Email || utf8.RuneCountInString(*input.Email) > 254 {
			fields = append(fields, errx.FieldError{Pointer: "/email", Code: "validation.email.invalid"})
		} else {
			emailNormalized = strings.ToLower(parsed.Address)
			input.Email = &emailNormalized
		}
	}
	phoneNormalized := ""
	input.Phone = cleanOptional(input.Phone)
	if input.Phone != nil {
		phoneNormalized = normalizePhone(*input.Phone)
		if len(phoneNormalized) < 7 || len(phoneNormalized) > 16 {
			fields = append(fields, errx.FieldError{Pointer: "/phone", Code: "validation.phone.invalid"})
		}
	}
	if input.CompanyName != nil && utf8.RuneCountInString(*input.CompanyName) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/companyName", Code: "validation.length"})
	}
	if input.JobTitle != nil && utf8.RuneCountInString(*input.JobTitle) > 160 {
		fields = append(fields, errx.FieldError{Pointer: "/jobTitle", Code: "validation.length"})
	}
	if input.Source != nil && utf8.RuneCountInString(*input.Source) > 120 {
		fields = append(fields, errx.FieldError{Pointer: "/source", Code: "validation.length"})
	}
	if !validLeadStatus(input.Status) || input.Status == "converted" {
		fields = append(fields, errx.FieldError{Pointer: "/status", Code: "validation.enum"})
	}
	custom, err := boundedCustomFields(input.CustomFields)
	if err != nil {
		fields = append(fields, errx.FieldError{Pointer: "/customFields", Code: "validation.custom_fields.invalid"})
	}
	if len(fields) > 0 {
		return LeadInput{}, "", "", nil, &errx.ValidationError{Fields: fields}
	}
	return input, emailNormalized, phoneNormalized, custom, nil
}

func validateLeadFilter(filter LeadListFilter) (LeadListFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Status = strings.TrimSpace(filter.Status)
	if utf8.RuneCountInString(filter.Query) > 120 {
		return LeadListFilter{}, validation("/query/query", "validation.length")
	}
	if filter.Status != "" && !validLeadStatus(filter.Status) {
		return LeadListFilter{}, validation("/query/status", "validation.enum")
	}
	filter.Limit = clampPageSize(filter.Limit, DefaultPageSize, MaxPageSize)
	return filter, nil
}

func validateDealUpdate(input DealUpdateInput) (DealUpdateInput, []byte, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.ForecastCategory = strings.TrimSpace(input.ForecastCategory)
	fields := make([]errx.FieldError, 0, 6)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if input.PipelineID == ([16]byte{}) {
		fields = append(fields, errx.FieldError{Pointer: "/pipelineId", Code: "validation.required"})
	}
	if input.StageID == ([16]byte{}) {
		fields = append(fields, errx.FieldError{Pointer: "/stageId", Code: "validation.required"})
	}
	if input.AmountMinor < 0 {
		fields = append(fields, errx.FieldError{Pointer: "/amountMinor", Code: "validation.minimum"})
	}
	if !currencyCodePattern.MatchString(input.Currency) {
		fields = append(fields, errx.FieldError{Pointer: "/currency", Code: "validation.currency.invalid"})
	}
	if !validForecastCategory(input.ForecastCategory) {
		fields = append(fields, errx.FieldError{Pointer: "/forecastCategory", Code: "validation.enum"})
	}
	if input.PlannedStartDate != nil && input.ExpectedCloseDate != nil && input.PlannedStartDate.After(*input.ExpectedCloseDate) {
		fields = append(fields, errx.FieldError{Pointer: "/plannedStartDate", Code: "validation.date.range"})
	}
	custom, err := boundedCustomFields(input.CustomFields)
	if err != nil {
		fields = append(fields, errx.FieldError{Pointer: "/customFields", Code: "validation.custom_fields.invalid"})
	}
	if len(fields) > 0 {
		return DealUpdateInput{}, nil, &errx.ValidationError{Fields: fields}
	}
	return input, custom, nil
}

func validateDealFilter(filter DealListFilter) (DealListFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Status = strings.TrimSpace(filter.Status)
	if utf8.RuneCountInString(filter.Query) > 120 {
		return DealListFilter{}, validation("/query/query", "validation.length")
	}
	if filter.Status != "" && !validDealStatus(filter.Status) {
		return DealListFilter{}, validation("/query/status", "validation.enum")
	}
	filter.Limit = clampPageSize(filter.Limit, DefaultPageSize, MaxPageSize)
	return filter, nil
}

func validateDealOutcome(input DealOutcomeInput) (DealOutcomeInput, error) {
	input.Status = strings.TrimSpace(input.Status)
	input.ForecastCategory = strings.TrimSpace(input.ForecastCategory)
	input.LostReason = cleanOptional(input.LostReason)
	fields := make([]errx.FieldError, 0, 3)
	if !validDealStatus(input.Status) {
		fields = append(fields, errx.FieldError{Pointer: "/status", Code: "validation.enum"})
	}
	if input.Status == "lost" {
		if input.LostReason == nil || utf8.RuneCountInString(*input.LostReason) > 500 {
			fields = append(fields, errx.FieldError{Pointer: "/lostReason", Code: "validation.required"})
		}
	} else if input.LostReason != nil {
		fields = append(fields, errx.FieldError{Pointer: "/lostReason", Code: "validation.not_allowed"})
	}
	if input.Status == "open" {
		if !validForecastCategory(input.ForecastCategory) || input.ForecastCategory == "closed" {
			fields = append(fields, errx.FieldError{Pointer: "/forecastCategory", Code: "validation.enum"})
		}
	} else {
		input.ForecastCategory = "closed"
	}
	if len(fields) > 0 {
		return DealOutcomeInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validateLineItem(input LineItemInput) (LineItemInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Quantity = strings.TrimSpace(input.Quantity)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	fields := make([]errx.FieldError, 0, 5)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if !quantityPattern.MatchString(input.Quantity) || input.Quantity == "0" || strings.HasPrefix(input.Quantity, "0.") && strings.Trim(input.Quantity[2:], "0") == "" {
		fields = append(fields, errx.FieldError{Pointer: "/quantity", Code: "validation.number.invalid"})
	}
	if input.UnitPriceMinor < 0 {
		fields = append(fields, errx.FieldError{Pointer: "/unitPriceMinor", Code: "validation.minimum"})
	}
	if !currencyCodePattern.MatchString(input.Currency) {
		fields = append(fields, errx.FieldError{Pointer: "/currency", Code: "validation.currency.invalid"})
	}
	if input.Position < 0 || input.Position >= MaxDealLineItems {
		fields = append(fields, errx.FieldError{Pointer: "/position", Code: "validation.range"})
	}
	if len(fields) > 0 {
		return LineItemInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validateParticipant(input DealParticipantInput) (DealParticipantInput, error) {
	input.Role = cleanOptional(input.Role)
	if input.ContactID == ([16]byte{}) {
		return DealParticipantInput{}, validation("/contactId", "validation.required")
	}
	if input.Role != nil && utf8.RuneCountInString(*input.Role) > 120 {
		return DealParticipantInput{}, validation("/role", "validation.length")
	}
	return input, nil
}

func boundedCustomFields(values map[string]any) ([]byte, error) {
	if values == nil {
		values = map[string]any{}
	}
	if len(values) > 100 {
		return nil, validation("/customFields", "validation.max_items")
	}
	for key := range values {
		if len(key) < 1 || len(key) > 63 {
			return nil, validation("/customFields", "validation.field.invalid")
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil || len(encoded) > 65536 {
		return nil, validation("/customFields", "validation.custom_fields.invalid")
	}
	return encoded, nil
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePhone(value string) string {
	var result strings.Builder
	for index, item := range strings.TrimSpace(value) {
		if item == '+' && index == 0 {
			result.WriteRune(item)
			continue
		}
		if unicode.IsDigit(item) && item <= unicode.MaxASCII {
			result.WriteRune(item)
		}
	}
	return result.String()
}

func validLeadStatus(status string) bool {
	switch status {
	case "new", "qualified", "converted", "disqualified":
		return true
	default:
		return false
	}
}

func validDealStatus(status string) bool {
	return status == "open" || status == "won" || status == "lost"
}

func validForecastCategory(category string) bool {
	switch category {
	case "pipeline", "best_case", "commit", "closed":
		return true
	default:
		return false
	}
}

func clampPageSize(value, fallback, maximum int) int {
	if value < 1 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
