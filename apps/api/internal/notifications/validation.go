package notifications

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

var messageKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._:-][a-z0-9]+)*$`)

func validateInput(input Input) (Input, []byte, error) {
	input.MessageKey = strings.TrimSpace(input.MessageKey)
	if input.TemplateVersion == 0 {
		input.TemplateVersion = 1
	}
	if input.Delivery == "" {
		input.Delivery = DeliveryInApp
	}
	var fields []errx.FieldError
	if len(input.MessageKey) > 160 || !messageKeyPattern.MatchString(input.MessageKey) {
		fields = append(fields, errx.FieldError{Pointer: "/messageKey", Code: "validation.format"})
	}
	if input.TemplateVersion < 1 {
		fields = append(fields, errx.FieldError{Pointer: "/templateVersion", Code: "validation.range"})
	}
	if input.Delivery != DeliveryInApp && input.Delivery != DeliveryEmail && input.Delivery != DeliveryBoth {
		fields = append(fields, errx.FieldError{Pointer: "/delivery", Code: "validation.enum"})
	}
	if (input.EntityType == nil) != (input.EntityID == nil) {
		fields = append(fields, errx.FieldError{Pointer: "/entityId", Code: "validation.reference.incomplete"})
	}
	if input.EntityType != nil {
		value := strings.TrimSpace(*input.EntityType)
		if len(value) < 1 || len(value) > 80 || !messageKeyPattern.MatchString(value) {
			fields = append(fields, errx.FieldError{Pointer: "/entityType", Code: "validation.format"})
		} else {
			input.EntityType = &value
		}
	}
	params := input.MessageParams
	if params == nil {
		params = map[string]any{}
		input.MessageParams = params
	}
	encoded, err := json.Marshal(params)
	if err != nil || len(encoded) > 32_768 {
		fields = append(fields, errx.FieldError{Pointer: "/messageParams", Code: "validation.object.invalid"})
	}
	if len(fields) > 0 {
		return Input{}, nil, &errx.ValidationError{Fields: fields}
	}
	return input, encoded, nil
}
