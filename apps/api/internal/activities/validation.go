package activities

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

const (
	maxCommentMentions = 20
	maxCalendarRange   = 45 * 24 * time.Hour
)

func validateActivityInput(input AdvancedInput) (AdvancedInput, error) {
	input.Type = strings.TrimSpace(input.Type)
	input.Title = strings.TrimSpace(input.Title)
	input.Status = strings.TrimSpace(input.Status)
	input.Priority = strings.TrimSpace(input.Priority)
	input.Location = trimPointer(input.Location)
	input.Body = trimPointer(input.Body)
	input.VisibilityScope = strings.TrimSpace(input.VisibilityScope)
	if input.Status == "" {
		input.Status = "open"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	if input.VisibilityScope == "" {
		input.VisibilityScope = "workspace"
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	} else {
		input.OccurredAt = input.OccurredAt.UTC()
	}
	var fields []errx.FieldError
	if !oneOf(input.Type, "task", "call", "meeting", "note") {
		fields = append(fields, field("/type", "validation.enum"))
	}
	if len(input.Title) < 1 || len(input.Title) > 240 {
		fields = append(fields, field("/title", "validation.length"))
	}
	if input.Body != nil && len(*input.Body) > 20_000 {
		fields = append(fields, field("/body", "validation.length"))
	}
	if !oneOf(input.Status, "open", "completed", "cancelled") {
		fields = append(fields, field("/status", "validation.enum"))
	}
	if !oneOf(input.Priority, "low", "normal", "high") {
		fields = append(fields, field("/priority", "validation.enum"))
	}
	if (input.RelatedType == nil) != (input.RelatedID == nil) {
		fields = append(fields, field("/relatedId", "validation.reference.incomplete"))
	} else if input.RelatedType != nil && !oneOf(*input.RelatedType, "contact", "company", "deal", "project") {
		fields = append(fields, field("/relatedType", "validation.enum"))
	}
	if !oneOf(input.VisibilityScope, "workspace", "department", "user") {
		fields = append(fields, field("/visibilityScope", "validation.enum"))
	} else if input.VisibilityScope == "workspace" && (input.ScopeDepartmentID != nil || input.ScopeUserID != nil) {
		fields = append(fields, field("/visibilityScope", "validation.reference.incomplete"))
	} else if input.VisibilityScope == "department" && (input.ScopeDepartmentID == nil || input.ScopeUserID != nil) {
		fields = append(fields, field("/scopeDepartmentId", "validation.reference.incomplete"))
	} else if input.VisibilityScope == "user" && (input.ScopeUserID == nil || input.ScopeDepartmentID != nil) {
		fields = append(fields, field("/scopeUserId", "validation.reference.incomplete"))
	}
	if input.EndsAt != nil {
		end := input.EndsAt.UTC()
		input.EndsAt = &end
		if end.Before(input.OccurredAt) {
			fields = append(fields, field("/endsAt", "validation.range"))
		}
	}
	if input.DueAt != nil {
		due := input.DueAt.UTC()
		input.DueAt = &due
	}
	if input.Location != nil && len(*input.Location) > 500 {
		fields = append(fields, field("/location", "validation.length"))
	}
	recurrence, recurrenceErr := normalizeRecurrence(input.RecurrenceRule)
	if recurrenceErr != nil {
		fields = append(fields, field("/recurrenceRule", "validation.recurrence.invalid"))
	} else {
		input.RecurrenceRule = recurrence
	}
	if len(fields) > 0 {
		return AdvancedInput{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

// normalizeRecurrence intentionally accepts a small, documented subset of
// RFC 5545. This keeps recurrence predictable until exception dates and full
// expansion semantics are implemented.
func normalizeRecurrence(value *string) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	raw := strings.ToUpper(strings.TrimSpace(*value))
	if len(raw) > 200 {
		return nil, errors.New("recurrence rule is too long")
	}
	parts := strings.Split(raw, ";")
	values := make(map[string]string, len(parts))
	for _, part := range parts {
		key, item, found := strings.Cut(part, "=")
		if !found || key == "" || item == "" {
			return nil, errors.New("malformed recurrence part")
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate recurrence field %s", key)
		}
		values[key] = item
	}
	if !oneOf(values["FREQ"], "DAILY", "WEEKLY", "MONTHLY") {
		return nil, errors.New("unsupported recurrence frequency")
	}
	if interval := values["INTERVAL"]; interval != "" {
		parsed, err := strconv.Atoi(interval)
		if err != nil || parsed < 1 || parsed > 365 {
			return nil, errors.New("invalid recurrence interval")
		}
	}
	if count := values["COUNT"]; count != "" {
		parsed, err := strconv.Atoi(count)
		if err != nil || parsed < 1 || parsed > 100 {
			return nil, errors.New("invalid recurrence count")
		}
	}
	if until := values["UNTIL"]; until != "" {
		if _, err := time.Parse("20060102T150405Z", until); err != nil {
			return nil, errors.New("invalid recurrence end")
		}
	}
	if values["COUNT"] != "" && values["UNTIL"] != "" {
		return nil, errors.New("COUNT and UNTIL are mutually exclusive")
	}
	for key := range values {
		if !oneOf(key, "FREQ", "INTERVAL", "COUNT", "UNTIL") {
			return nil, fmt.Errorf("unsupported recurrence field %s", key)
		}
	}
	order := []string{"FREQ", "INTERVAL", "COUNT", "UNTIL"}
	canonical := make([]string, 0, len(values))
	for _, key := range order {
		if item := values[key]; item != "" {
			canonical = append(canonical, key+"="+item)
		}
	}
	result := strings.Join(canonical, ";")
	return &result, nil
}

func normalizeMentions(mentions []string) ([]string, error) {
	if len(mentions) > maxCommentMentions {
		return nil, errors.New("too many mentions")
	}
	seen := make(map[string]struct{}, len(mentions))
	normalized := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		mention = strings.ToLower(strings.TrimSpace(mention))
		if mention == "" {
			return nil, errors.New("empty mention")
		}
		if _, exists := seen[mention]; exists {
			continue
		}
		seen[mention] = struct{}{}
		normalized = append(normalized, mention)
	}
	if len(normalized) > maxCommentMentions {
		return nil, errors.New("too many mentions")
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validateCalendarRange(start, end time.Time) error {
	if start.IsZero() || end.IsZero() || !start.Before(end) || end.Sub(start) > maxCalendarRange {
		return &errx.ValidationError{Fields: []errx.FieldError{
			field("/query/end", "validation.range"),
		}}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func field(pointer, code string) errx.FieldError {
	return errx.FieldError{Pointer: pointer, Code: code}
}
