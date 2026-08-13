package automation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

const (
	maxConditionDepth      = 5
	maxConditionPredicates = 64
	maxActions             = 20
)

var (
	fieldNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){0,3}$`)
	keyPattern       = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,159}$`)
	blockedFields    = map[string]struct{}{
		"id": {}, "workspace_id": {}, "version": {}, "created_at": {}, "updated_at": {},
		"password": {}, "password_hash": {}, "token": {}, "secret": {}, "secret_hash": {},
	}
)

func ValidateRule(spec RuleSpec) error {
	fields := make([]errx.FieldError, 0)
	name := strings.TrimSpace(spec.Name)
	if len(name) < 1 || len(name) > 160 {
		fields = append(fields, fieldError("/name", "validation.length"))
	}
	if !validTrigger(spec.Trigger) {
		fields = append(fields, fieldError("/trigger", "validation.enum"))
	}
	if !validEntityType(spec.EntityType) ||
		((spec.Trigger == TriggerDealStageChanged || spec.Trigger == TriggerDealWon || spec.Trigger == TriggerDealLost) && spec.EntityType != EntityDeal) ||
		(spec.Trigger == TriggerTaskOverdue && spec.EntityType != EntityActivity) ||
		(spec.Trigger == TriggerScheduled && spec.EntityType != EntityWorkspace) {
		fields = append(fields, fieldError("/entityType", "validation.enum"))
	}
	if spec.RateLimitPerHour < 1 || spec.RateLimitPerHour > 100000 {
		fields = append(fields, fieldError("/rateLimitPerHour", "validation.range"))
	}
	predicates := 0
	validateCondition(spec.Conditions, "/conditions", 0, &predicates, &fields)
	if len(spec.Actions) < 1 || len(spec.Actions) > maxActions {
		fields = append(fields, fieldError("/actions", "validation.array.size"))
	}
	for index, action := range spec.Actions {
		if err := ValidateAction(action); err != nil {
			if validation, ok := err.(*errx.ValidationError); ok {
				for _, field := range validation.Fields {
					field.Pointer = fmt.Sprintf("/actions/%d%s", index, field.Pointer)
					fields = append(fields, field)
				}
			}
		}
	}
	if len(fields) > 0 {
		return &errx.ValidationError{Fields: fields}
	}
	return nil
}

func validEntityType(entityType EntityType) bool {
	switch entityType {
	case EntityContact, EntityCompany, EntityLead, EntityDeal, EntityActivity, EntityWorkspace:
		return true
	default:
		return false
	}
}

func validTrigger(trigger TriggerType) bool {
	switch trigger {
	case TriggerRecordCreated, TriggerRecordUpdated, TriggerDealStageChanged,
		TriggerDealWon, TriggerDealLost, TriggerTaskOverdue, TriggerScheduled:
		return true
	default:
		return false
	}
}

func validateCondition(condition Condition, pointer string, depth int, predicates *int, fields *[]errx.FieldError) {
	if depth > maxConditionDepth {
		*fields = append(*fields, fieldError(pointer, "automation.condition.depth"))
		return
	}
	groupCount := 0
	if condition.All != nil {
		groupCount++
	}
	if condition.Any != nil {
		groupCount++
	}
	predicate := condition.Field != "" || condition.Operator != "" || condition.Value != nil
	if predicate {
		groupCount++
	}
	if groupCount != 1 {
		*fields = append(*fields, fieldError(pointer, "automation.condition.shape"))
		return
	}
	if condition.All != nil || condition.Any != nil {
		children := condition.All
		name := "all"
		if condition.Any != nil {
			children = condition.Any
			name = "any"
		}
		if len(children) < 1 || len(children) > maxConditionPredicates {
			*fields = append(*fields, fieldError(pointer+"/"+name, "validation.array.size"))
			return
		}
		for index, child := range children {
			validateCondition(child, fmt.Sprintf("%s/%s/%d", pointer, name, index), depth+1, predicates, fields)
		}
		return
	}
	*predicates++
	if *predicates > maxConditionPredicates {
		*fields = append(*fields, fieldError(pointer, "automation.condition.count"))
		return
	}
	if !fieldNamePattern.MatchString(condition.Field) {
		*fields = append(*fields, fieldError(pointer+"/field", "validation.format"))
	}
	if !validComparator(condition.Operator) {
		*fields = append(*fields, fieldError(pointer+"/operator", "validation.enum"))
		return
	}
	if len(condition.Value) == 0 || len(condition.Value) > 8192 || !json.Valid(condition.Value) {
		*fields = append(*fields, fieldError(pointer+"/value", "validation.json"))
		return
	}
	if err := validatePredicateValue(condition.Operator, condition.Value); err != nil {
		*fields = append(*fields, fieldError(pointer+"/value", err.Error()))
	}
}

func validComparator(comparator Comparator) bool {
	switch comparator {
	case ComparatorEquals, ComparatorNotEquals, ComparatorContains,
		ComparatorGreaterThan, ComparatorGreaterOrEqual, ComparatorLessThan,
		ComparatorLessOrEqual, ComparatorDateBefore, ComparatorDateAfter,
		ComparatorTagPresent, ComparatorOwnerEquals, ComparatorTeamEquals:
		return true
	default:
		return false
	}
}

func validatePredicateValue(comparator Comparator, raw json.RawMessage) error {
	switch comparator {
	case ComparatorGreaterThan, ComparatorGreaterOrEqual, ComparatorLessThan, ComparatorLessOrEqual:
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value json.Number
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("validation.number")
		}
		if _, err := value.Float64(); err != nil {
			return fmt.Errorf("validation.number")
		}
	case ComparatorDateBefore, ComparatorDateAfter:
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return fmt.Errorf("validation.date")
		}
		if _, err := parseDate(value); err != nil {
			return fmt.Errorf("validation.date")
		}
	case ComparatorContains:
		var value string
		if json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value) == "" || len(value) > 500 {
			return fmt.Errorf("validation.string")
		}
	case ComparatorTagPresent, ComparatorOwnerEquals, ComparatorTeamEquals:
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return fmt.Errorf("validation.uuid.invalid")
		}
		if _, err := ids.Parse(value); err != nil {
			return fmt.Errorf("validation.uuid.invalid")
		}
	default:
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			return fmt.Errorf("validation.json")
		}
		switch value.(type) {
		case nil, bool, string, json.Number:
		default:
			return fmt.Errorf("validation.scalar")
		}
	}
	return nil
}

func ValidateAction(action Action) error {
	fields := make([]errx.FieldError, 0)
	if len(action.Params) == 0 || len(action.Params) > 16384 || !json.Valid(action.Params) {
		return &errx.ValidationError{Fields: []errx.FieldError{fieldError("/params", "validation.json")}}
	}
	switch action.Type {
	case ActionCreateTask:
		var params CreateTaskParams
		if decodeStrict(action.Params, &params) != nil || !keyPattern.MatchString(params.TitleKey) ||
			params.DueInHours < 0 || params.DueInHours > 8760 ||
			(params.Priority != "" && params.Priority != "low" && params.Priority != "normal" && params.Priority != "high") {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		}
		if params.AssigneeID != "" {
			if _, err := ids.Parse(params.AssigneeID); err != nil {
				fields = append(fields, fieldError("/params/assigneeId", "validation.uuid.invalid"))
			}
		}
	case ActionAssignOwner:
		var params AssignOwnerParams
		if decodeStrict(action.Params, &params) != nil {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		} else if _, err := ids.Parse(params.OwnerID); err != nil {
			fields = append(fields, fieldError("/params/ownerId", "validation.uuid.invalid"))
		}
	case ActionAddTag, ActionRemoveTag:
		var params TagParams
		if decodeStrict(action.Params, &params) != nil {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		} else if _, err := ids.Parse(params.TagID); err != nil {
			fields = append(fields, fieldError("/params/tagId", "validation.uuid.invalid"))
		}
	case ActionCreateNotification:
		var params CreateNotificationParams
		if decodeStrict(action.Params, &params) != nil || !keyPattern.MatchString(params.MessageKey) {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		} else if _, err := ids.Parse(params.RecipientID); err != nil {
			fields = append(fields, fieldError("/params/recipientId", "validation.uuid.invalid"))
		}
	case ActionSendEmail:
		var params SendEmailParams
		if decodeStrict(action.Params, &params) != nil || !fieldNamePattern.MatchString(params.RecipientField) ||
			!keyPattern.MatchString(params.TemplateKey) || len(params.TemplateKey) > 152 ||
			len(params.TemplateParams) > maxAutomationEmailParams {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		} else if !emailRecipientField(params.RecipientField) {
			fields = append(fields, fieldError("/params/recipientField", "validation.enum"))
		} else {
			for name, value := range params.TemplateParams {
				if !fieldNamePattern.MatchString(name) || !validTemplateValueForRender(value) {
					fields = append(fields, fieldError("/params/templateParams", "automation.action.invalid"))
					break
				}
			}
		}
	case ActionInvokeWebhook:
		var params InvokeWebhookParams
		if decodeStrict(action.Params, &params) != nil {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		} else if _, err := ids.Parse(params.SubscriptionID); err != nil {
			fields = append(fields, fieldError("/params/subscriptionId", "validation.uuid.invalid"))
		}
	case ActionUpdateField:
		var params UpdateFieldParams
		if decodeStrict(action.Params, &params) != nil || !fieldNamePattern.MatchString(params.Field) || params.Value == nil {
			fields = append(fields, fieldError("/params", "automation.action.invalid"))
		} else if _, blocked := blockedFields[params.Field]; blocked {
			fields = append(fields, fieldError("/params/field", "automation.field.protected"))
		}
	default:
		fields = append(fields, fieldError("/type", "validation.enum"))
	}
	if len(fields) > 0 {
		return &errx.ValidationError{Fields: fields}
	}
	return nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON")
		}
		return err
	}
	return nil
}

func parseDate(value string) (time.Time, error) {
	if instant, err := time.Parse(time.RFC3339, value); err == nil {
		return instant.UTC(), nil
	}
	return time.Parse("2006-01-02", value)
}

func fieldError(pointer, code string) errx.FieldError {
	return errx.FieldError{Pointer: pointer, Code: code}
}
