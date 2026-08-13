package app

import (
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestSSELiveNotificationIsVisibleOnlyToItsRecipient(t *testing.T) {
	recipient := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	otherMember := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	event := notifications.Event{
		Type:     "notification.created",
		Data:     []byte(`{"messageKey":"notifications.activity.assigned"}`),
		Audience: notifications.AudienceUser, RecipientUserID: recipient.String(),
	}
	if !visibleSSEEvent(recipient, event) {
		t.Fatal("recipient could not receive own live notification")
	}
	if visibleSSEEvent(otherMember, event) {
		t.Fatal("workspace member received another member's live notification")
	}
}

func TestSSEReplayNotificationIsVisibleOnlyToItsRecipient(t *testing.T) {
	recipient := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	otherMember := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	replayed := notifications.Event{
		Type:     "notification.created",
		Audience: notifications.AudienceUser, RecipientUserID: recipient.String(),
	}
	if !visibleSSEEvent(recipient, replayed) || visibleSSEEvent(otherMember, replayed) {
		t.Fatal("replay visibility boundary failed")
	}
	if !visibleSSEEvent(otherMember, notifications.Event{
		Type: "customers.contact.updated", Data: []byte(`{}`), Audience: notifications.AudienceWorkspace,
	}) {
		t.Fatal("general workspace events must remain visible to active members")
	}
}

func TestSSETargetedNotificationFailsClosed(t *testing.T) {
	member := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	for _, event := range []notifications.Event{
		{Type: "notification.created"},
		{Type: "notification.created", Audience: notifications.AudienceWorkspace},
		{Type: "notification.created", Audience: notifications.AudienceUser},
		{Type: "notification.created", Audience: notifications.AudienceUser, RecipientUserID: "invalid"},
	} {
		if visibleSSEEvent(member, event) {
			t.Fatalf("malformed targeted event was visible: %+v", event)
		}
	}
}
