package seed

import (
	"strings"
	"testing"
	"time"
)

func TestRequiredProfileCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected Counts
	}{
		{"small", Counts{Contacts: 1_000, Companies: 250, Deals: 500, Activities: 5_000}},
		{"benchmark", Counts{Contacts: 100_000, Companies: 25_000, Deals: 50_000, Activities: 500_000}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			profile, err := ParseProfile(test.name)
			if err != nil {
				t.Fatalf("ParseProfile() error = %v", err)
			}
			if profile.counts() != test.expected {
				t.Fatalf("counts = %+v, want %+v", profile.counts(), test.expected)
			}
		})
	}
}

func TestGeneratorIsDeterministicAndUsesReservedData(t *testing.T) {
	t.Parallel()

	profile, err := ParseProfile("small")
	if err != nil {
		t.Fatal(err)
	}
	owner := stableID("global", "user", 0)
	first := newGenerator(profile, owner)
	second := newGenerator(profile, owner)

	if first.workspaceID != second.workspaceID || first.contactID(999) != second.contactID(999) {
		t.Fatal("deterministic IDs changed between equivalent generators")
	}
	if first.contactID(999) == first.companyID(999) {
		t.Fatal("entity namespaces produced the same ID")
	}
	row := first.contactRow(999)
	email, ok := row[5].(string)
	if !ok || !strings.HasSuffix(email, "@example.invalid") {
		t.Fatalf("contact email %v is not in the reserved synthetic domain", row[5])
	}
	createdAt, ok := row[18].(time.Time)
	if !ok || createdAt.Location() != time.UTC {
		t.Fatalf("created_at = %v, want a UTC time", row[18])
	}
}

func TestGeneratedDealRowsSatisfyOutcomeShape(t *testing.T) {
	t.Parallel()

	profile, err := ParseProfile("small")
	if err != nil {
		t.Fatal(err)
	}
	generator := newGenerator(profile, stableID("test", "owner", 0))
	for _, index := range []int64{0, 1, 13} {
		row := generator.dealRow(index)
		if len(row) != 20 {
			t.Fatalf("deal row %d has %d values, want 20", index, len(row))
		}
		status := row[12].(string)
		forecast := row[14].(string)
		wonAt, lostAt := row[15], row[16]
		switch status {
		case "open":
			if forecast == "closed" || wonAt != nil || lostAt != nil {
				t.Fatalf("open row has closed outcome fields: %#v", row)
			}
		case "won":
			if forecast != "closed" || wonAt == nil || lostAt != nil {
				t.Fatalf("won row has invalid outcome fields: %#v", row)
			}
		case "lost":
			if forecast != "closed" || wonAt != nil || lostAt == nil {
				t.Fatalf("lost row has invalid outcome fields: %#v", row)
			}
		default:
			t.Fatalf("unexpected deal status %q", status)
		}
	}
}

func TestStableIDHasUUIDv7Bits(t *testing.T) {
	t.Parallel()

	id := stableID("benchmark", "activity", 499_999)
	if version := id[6] >> 4; version != 7 {
		t.Fatalf("UUID version = %d, want 7", version)
	}
	if variant := id[8] >> 6; variant != 2 {
		t.Fatalf("UUID variant = %b, want 10", variant)
	}
}

func TestDatasetHashChangesWithProfile(t *testing.T) {
	t.Parallel()

	small, _ := ParseProfile("small")
	benchmark, _ := ParseProfile("benchmark")
	if small.datasetHash() == benchmark.datasetHash() {
		t.Fatal("different seed profiles produced the same dataset hash")
	}
	if len(small.datasetHash()) != 64 {
		t.Fatalf("dataset hash length = %d, want 64", len(small.datasetHash()))
	}
}

func TestParseProfileRejectsUnknownName(t *testing.T) {
	t.Parallel()

	if _, err := ParseProfile("large"); err == nil {
		t.Fatal("ParseProfile() accepted an unknown profile")
	}
}

func TestParseProfileNormalizesCLIInput(t *testing.T) {
	t.Parallel()

	profile, err := ParseProfile(" SMALL ")
	if err != nil {
		t.Fatalf("ParseProfile() error = %v", err)
	}
	if profile.Name != "small" {
		t.Fatalf("profile name = %q, want small", profile.Name)
	}
}

func TestRunRejectsProductionBeforeDatabaseAccess(t *testing.T) {
	t.Parallel()

	_, err := Run(t.Context(), nil, nil, Profile{}, Options{Environment: "production"})
	if err == nil || !strings.Contains(err.Error(), "disabled in production") {
		t.Fatalf("Run() error = %v, want production rejection", err)
	}
}
