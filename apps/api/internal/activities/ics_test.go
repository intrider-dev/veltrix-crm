package activities

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestWriteICSEscapesTextAndUsesCRLF(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	var output bytes.Buffer
	err := WriteICS(&output, CalendarExport{
		ProductName: "CRM, Lab", ProductID: "veltrix-crm", GeneratedAt: now,
		Items: []CalendarItem{{
			ID: "activity@example.invalid", Type: "meeting", Title: "A;B\nInjected: no",
			Body: "slash \\ and comma,", Location: "Room; 1", Start: now,
			UpdatedAt: now,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, expected := range []string{
		"BEGIN:VCALENDAR\r\n", "SUMMARY:A\\;B\\nInjected: no\r\n",
		"DESCRIPTION:slash \\\\ and comma\\,\r\n", "END:VCALENDAR\r\n",
	} {
		if !strings.Contains(got, expected) {
			t.Errorf("ICS output does not contain %q:\n%s", expected, got)
		}
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Fatal("ICS contains bare LF")
	}
}

func TestWriteICSFoldsUtf8LinesAt75Octets(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	var output bytes.Buffer
	err := WriteICS(&output, CalendarExport{
		ProductName: "CRM", ProductID: "crm", GeneratedAt: now,
		Items: []CalendarItem{{
			ID: "id", Type: "task", Title: strings.Repeat("я", 80), Start: now,
			UpdatedAt: now,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\r\n"), "\r\n") {
		if !utf8.ValidString(line) {
			t.Fatalf("folded line is invalid UTF-8: %q", line)
		}
		if len([]byte(line)) > 75 {
			t.Fatalf("folded line is %d octets: %q", len([]byte(line)), line)
		}
	}
}
