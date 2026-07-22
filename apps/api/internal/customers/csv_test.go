package customers

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCSVHeaderRecordIsNotOverwrittenWhenReaderReusesRows(t *testing.T) {
	t.Parallel()
	reader := csv.NewReader(strings.NewReader("firstName,lastName,email\nAda,Lovelace,ada@example.invalid\n"))
	reader.ReuseRecord = true

	headers, err := readOwnedCSVRecord(reader)
	if err != nil {
		t.Fatalf("readOwnedCSVRecord() error = %v", err)
	}
	if _, err := reader.Read(); err != nil {
		t.Fatalf("read data row: %v", err)
	}

	want := []string{"firstName", "lastName", "email"}
	for index := range want {
		if headers[index] != want[index] {
			t.Fatalf("headers[%d] = %q after reading data row, want %q", index, headers[index], want[index])
		}
	}
}

func TestCSVHeadersAndSuggestedMappingSupportEnglishAndRussian(t *testing.T) {
	t.Parallel()
	headers := []string{"Имя", "Фамилия", "Email", "Компания"}
	if err := validateCSVHeaders(headers); err != nil {
		t.Fatalf("validateCSVHeaders() error = %v", err)
	}
	mapping := suggestedContactMapping(headers)
	want := map[string]string{"firstName": "Имя", "lastName": "Фамилия", "email": "Email", "companyName": "Компания"}
	for key, value := range want {
		if mapping[key] != value {
			t.Errorf("mapping[%q] = %q, want %q", key, mapping[key], value)
		}
	}
}

func TestCSVHeadersRejectCaseInsensitiveDuplicates(t *testing.T) {
	t.Parallel()
	if err := validateCSVHeaders([]string{"Email", " email "}); err == nil {
		t.Fatal("case-insensitive duplicate header was accepted")
	}
}

func TestSpreadsheetSafeNeutralizesFormulasWithoutChangingText(t *testing.T) {
	t.Parallel()
	for _, dangerous := range []string{"=cmd()", "+1+1", " -2+3", "@SUM(A1:A2)"} {
		if got := spreadsheetSafe(dangerous); !strings.HasPrefix(got, "'") {
			t.Errorf("spreadsheetSafe(%q) = %q, want apostrophe prefix", dangerous, got)
		}
	}
	if got := spreadsheetSafe("Alice"); got != "Alice" {
		t.Fatalf("plain value changed to %q", got)
	}
}

func TestImportMappingRequiresKnownUniqueHeaders(t *testing.T) {
	t.Parallel()
	headers := []string{"first", "last", "email"}
	if err := validateImportMapping(ContactImportMapping{FirstName: "first", LastName: "last", Email: "email"}, headers); err != nil {
		t.Fatalf("valid mapping rejected: %v", err)
	}
	if err := validateImportMapping(ContactImportMapping{FirstName: "first", LastName: "first"}, headers); err == nil {
		t.Fatal("reused source header was accepted")
	}
	if err := validateImportMapping(ContactImportMapping{FirstName: "first", LastName: "missing"}, headers); err == nil {
		t.Fatal("unknown source header was accepted")
	}
}

func TestComplexCSVCustomValuesAreTyped(t *testing.T) {
	t.Parallel()
	money, err := parseCSVCustomValue(CustomFieldDefinition{ValueType: "money"}, `{"minor":1250,"currency":"USD"}`)
	if err != nil {
		t.Fatalf("money parse failed: %v", err)
	}
	encoded, _ := json.Marshal(money)
	if !strings.Contains(string(encoded), `"currency":"USD"`) {
		t.Fatalf("unexpected money JSON: %s", encoded)
	}
	values, err := parseCSVCustomValue(CustomFieldDefinition{ValueType: "multi_select"}, "emea; apac")
	if err != nil || len(values.([]string)) != 2 {
		t.Fatalf("multi select parse = %#v, %v", values, err)
	}
}

func TestContactImportJobPayloadRejectsMalformedIDs(t *testing.T) {
	t.Parallel()
	if _, err := DecodeContactImportJobPayload(json.RawMessage(`{"importSessionId":"bad","actorUserId":"also-bad"}`)); err == nil {
		t.Fatal("malformed job payload was accepted")
	}
	valid := json.RawMessage(`{"importSessionId":"01982d57-3400-7000-8000-000000000001","actorUserId":"01982d57-3400-7000-8000-000000000002"}`)
	if _, err := DecodeContactImportJobPayload(valid); err != nil {
		t.Fatalf("valid job payload rejected: %v", err)
	}
}

func TestCustomerMigrationContainsRLSAndImportResumeGuards(t *testing.T) {
	t.Parallel()
	contents, err := os.ReadFile("../../migrations/000004_customers_advanced.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(contents))
	for _, required := range []string{
		"alter table customers.import_rows force row level security",
		"create policy tenant_scope on customers.import_rows",
		"create table customers.record_merges",
		"custom_field_values_value_gin_idx",
		"source_id <> target_id",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("customer migration is missing %q", required)
		}
	}
}
