package activities

import (
	"errors"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

func TestNormalizeRecurrenceCanonicalizesSupportedSubset(t *testing.T) {
	t.Parallel()
	raw := " count=8;freq=weekly;interval=2 "
	got, err := normalizeRecurrence(&raw)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != "FREQ=WEEKLY;INTERVAL=2;COUNT=8" {
		t.Fatalf("normalized recurrence = %v", got)
	}
}

func TestNormalizeRecurrenceRejectsUnboundedOrUnsupportedRules(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"FREQ=HOURLY", "FREQ=DAILY;INTERVAL=0", "FREQ=WEEKLY;BYDAY=MO",
		"FREQ=MONTHLY;COUNT=101", "FREQ=DAILY;COUNT=2;UNTIL=20261201T000000Z",
	} {
		raw := raw
		if _, err := normalizeRecurrence(&raw); err == nil {
			t.Errorf("normalizeRecurrence(%q) succeeded", raw)
		}
	}
}

func TestValidateActivityInputRejectsInconsistentCalendarTimes(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	end := start.Add(-time.Minute)
	_, err := validateActivityInput(AdvancedInput{
		Type: "meeting", Title: "Review", OccurredAt: start, EndsAt: &end,
	})
	var validation *errx.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want validation error", err)
	}
}

func TestNormalizeMentionsDeduplicatesDeterministically(t *testing.T) {
	t.Parallel()
	mentions, err := normalizeMentions([]string{"B", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mentions) != 2 || mentions[0] != "a" || mentions[1] != "b" {
		t.Fatalf("mentions = %v", mentions)
	}
}

func TestCalendarRangeIsBounded(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := validateCalendarRange(start, start.Add(45*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := validateCalendarRange(start, start.Add(45*24*time.Hour+time.Second)); err == nil {
		t.Fatal("range longer than 45 days accepted")
	}
}
