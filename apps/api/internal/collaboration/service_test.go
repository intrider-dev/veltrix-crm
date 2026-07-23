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

func TestConversationRecipientCountAllowsNewEntityDiscussion(t *testing.T) {
	for _, test := range []struct {
		name  string
		count int
		valid bool
	}{
		{name: "empty", count: 0, valid: false},
		{name: "entity owner only", count: 1, valid: true},
		{name: "direct chat", count: 2, valid: true},
		{name: "bounded group", count: maxConversationMembers, valid: true},
		{name: "oversized group", count: maxConversationMembers + 1, valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validConversationRecipientCount(test.count); got != test.valid {
				t.Fatalf("validConversationRecipientCount(%d)=%t, want %t", test.count, got, test.valid)
			}
		})
	}
}
