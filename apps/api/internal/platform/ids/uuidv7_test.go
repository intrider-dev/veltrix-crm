package ids

import (
	"testing"
	"time"
)

func TestUUIDV7RoundTripAndBits(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, time.July, 21, 10, 20, 30, 0, time.UTC)
	id, err := NewV7At(when)
	if err != nil {
		t.Fatal(err)
	}
	if id[6]>>4 != 7 {
		t.Fatalf("version = %d, want 7", id[6]>>4)
	}
	if id[8]>>6 != 2 {
		t.Fatalf("variant bits = %b, want 10", id[8]>>6)
	}
	parsed, err := Parse(id.String())
	if err != nil {
		t.Fatal(err)
	}
	if parsed != id {
		t.Fatalf("round trip mismatch: %s != %s", parsed, id)
	}
}

func TestParseRejectsMalformedUUID(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "abc", "018f-0000", "018f0000-0000-7000-8000-00000000000z"} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) succeeded", value)
		}
	}
}
