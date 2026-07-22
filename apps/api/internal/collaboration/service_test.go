package collaboration

import (
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestDirectConversationKeyIsOrderIndependent(t *testing.T) {
	first := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	second := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	forward := directConversationKey([]ids.UUID{first, second})
	reverse := directConversationKey([]ids.UUID{second, first})
	if forward != reverse || len(forward) != 64 {
		t.Fatalf("unstable direct key: %q %q", forward, reverse)
	}
}
