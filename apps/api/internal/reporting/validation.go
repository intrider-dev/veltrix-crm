package reporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

const maxReportRange = 366 * 24 * time.Hour

func ValidatePeriod(start, end time.Time, timezone string) (Period, error) {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return Period{}, validation("/query/timezone", "validation.timezone.invalid")
	}
	start = start.UTC()
	end = end.UTC()
	if start.IsZero() || end.IsZero() || !start.Before(end) || end.Sub(start) > maxReportRange {
		return Period{}, validation("/query/end", "validation.range")
	}
	return Period{Start: start, End: end, Timezone: timezone}, nil
}

func validatePreferences(input PreferencesInput) (PreferencesInput, error) {
	if input.PeriodDays == 0 {
		input.PeriodDays = 30
	}
	if input.PeriodDays != 7 && input.PeriodDays != 30 &&
		input.PeriodDays != 90 && input.PeriodDays != 365 {
		return PreferencesInput{}, validation("/periodDays", "validation.enum")
	}
	if len(input.Layout) == 0 {
		input.Layout = json.RawMessage(`{}`)
	}
	trimmedLayout := bytes.TrimSpace(input.Layout)
	if len(input.Layout) > 32_768 || len(trimmedLayout) == 0 ||
		!json.Valid(trimmedLayout) || trimmedLayout[0] != '{' {
		return PreferencesInput{}, validation("/layout", "validation.object.invalid")
	}
	input.Layout = append(json.RawMessage(nil), trimmedLayout...)
	if input.Timezone != nil {
		trimmed := strings.TrimSpace(*input.Timezone)
		if trimmed == "" {
			input.Timezone = nil
		} else {
			if _, err := time.LoadLocation(trimmed); err != nil {
				return PreferencesInput{}, validation("/timezone", "validation.timezone.invalid")
			}
			input.Timezone = &trimmed
		}
	}
	return input, nil
}

func conversionRate(converted, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(converted) / float64(total)
}

func PeriodFromPreferences(now time.Time, preferences Preferences) (Period, error) {
	location, err := time.LoadLocation(preferences.EffectiveTimezone)
	if err != nil {
		return Period{}, validation("/timezone", "validation.timezone.invalid")
	}
	localNow := now.In(location)
	startLocal := time.Date(
		localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location,
	).AddDate(0, 0, -int(preferences.PeriodDays)+1)
	return ValidatePeriod(startLocal.UTC(), now.UTC(), preferences.EffectiveTimezone)
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
