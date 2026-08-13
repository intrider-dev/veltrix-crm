package reporting

import (
	"encoding/json"
	"testing"
	"time"
)

func TestValidatePeriodUsesIANAZoneAndBound(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	period, err := ValidatePeriod(start, start.Add(30*24*time.Hour), "Asia/Qyzylorda")
	if err != nil {
		t.Fatal(err)
	}
	if period.Timezone != "Asia/Qyzylorda" {
		t.Fatalf("timezone = %q", period.Timezone)
	}
	if _, err := ValidatePeriod(start, start.Add(maxReportRange+time.Second), "UTC"); err == nil {
		t.Fatal("range over maximum accepted")
	}
	if _, err := ValidatePeriod(start, start.Add(time.Hour), "../../etc/passwd"); err == nil {
		t.Fatal("invalid timezone accepted")
	}
}

func TestValidatePreferencesRequiresBoundedObject(t *testing.T) {
	t.Parallel()
	zone := "Europe/London"
	got, err := validatePreferences(PreferencesInput{
		Layout: json.RawMessage(`{"cards":["pipeline"]}`), PeriodDays: 90, Timezone: &zone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.PeriodDays != 90 || got.Timezone == nil || *got.Timezone != zone {
		t.Fatalf("validated preferences = %+v", got)
	}
	if _, err := validatePreferences(PreferencesInput{Layout: json.RawMessage(`[]`)}); err == nil {
		t.Fatal("array layout accepted")
	}
}

func TestConversionRateHandlesEmptyDenominator(t *testing.T) {
	t.Parallel()
	if conversionRate(1, 0) != 0 {
		t.Fatal("empty denominator did not return zero")
	}
	if got := conversionRate(1, 4); got != 0.25 {
		t.Fatalf("conversion rate = %v", got)
	}
}

func TestPeriodFromPreferencesUsesLocalCalendarBoundary(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	period, err := PeriodFromPreferences(now, Preferences{
		PeriodDays: 7, EffectiveTimezone: "Asia/Qyzylorda",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := period.Start.Format(time.RFC3339); got != "2026-07-14T19:00:00Z" {
		t.Fatalf("period start = %s", got)
	}
}
