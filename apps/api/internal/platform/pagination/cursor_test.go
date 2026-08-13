package pagination

import (
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestCursorRoundTripAndFilterBinding(t *testing.T) {
	t.Parallel()
	timestamp := time.Date(2026, 7, 21, 12, 0, 0, 123, time.UTC)
	id := ids.MustParse("01982d57-3400-7000-8000-000000000001")
	encoded, err := Encode(timestamp, id, "q=a&status=active")
	if err != nil {
		t.Fatal(err)
	}
	decodedTime, decodedID, err := Decode(encoded, "q=a&status=active")
	if err != nil {
		t.Fatal(err)
	}
	if !decodedTime.Equal(timestamp) || decodedID != id {
		t.Fatalf("decoded cursor differs: %s %s", decodedTime, decodedID)
	}
	if _, _, err := Decode(encoded, "q=b&status=active"); err == nil {
		t.Fatal("cursor accepted for a different filter")
	}
}
