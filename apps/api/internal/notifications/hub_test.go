package notifications

import (
	"testing"
	"time"
)

func TestParseNotification(t *testing.T) {
	t.Parallel()
	workspace, event, ok := parseNotification("workspace:event:contact.updated")
	if !ok || workspace != "workspace" || event.ID != "event" || event.Type != "contact.updated" {
		t.Fatalf("unexpected parse result: %q %#v %v", workspace, event, ok)
	}
	if _, _, ok := parseNotification("malformed"); ok {
		t.Fatal("malformed notification accepted")
	}
}

func TestHubDoesNotPublishTargetedNotificationToAnotherMember(t *testing.T) {
	hub := NewHub(nil)
	workspaceID := "018f47d2-2044-7f25-89b0-85bd4c8ad8b3"
	recipientID := "018f47d2-2044-7f25-89b0-85bd4c8ad8b4"
	otherID := "018f47d2-2044-7f25-89b0-85bd4c8ad8b5"
	recipient, unsubscribeRecipient := hub.Subscribe(workspaceID, recipientID)
	defer unsubscribeRecipient()
	other, unsubscribeOther := hub.Subscribe(workspaceID, otherID)
	defer unsubscribeOther()
	event := Event{
		ID: "018f47d2-2044-7f25-89b0-85bd4c8ad8b6", Type: "notification.created",
		Data: []byte(`{"messageKey":"notifications.activity.assigned"}`), Audience: AudienceUser,
		RecipientUserID: recipientID,
	}
	hub.publish(workspaceID, event)
	select {
	case got := <-recipient:
		if got.ID != event.ID {
			t.Fatalf("recipient got unexpected event %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("recipient did not receive own notification")
	}
	select {
	case got := <-other:
		t.Fatalf("other member received targeted notification %+v", got)
	default:
	}
	hub.publish(workspaceID, Event{
		ID: "general", Type: "customers.contact.updated", Data: []byte(`{}`), Audience: AudienceWorkspace,
	})
	select {
	case <-other:
	case <-time.After(time.Second):
		t.Fatal("other member did not receive general workspace event")
	}
}

func TestHubDoesNotPublishTargetedDomainEventToAnotherMember(t *testing.T) {
	hub := NewHub(nil)
	workspaceID := "018f47d2-2044-7f25-89b0-85bd4c8ad8b3"
	recipientID := "018f47d2-2044-7f25-89b0-85bd4c8ad8b4"
	otherID := "018f47d2-2044-7f25-89b0-85bd4c8ad8b5"
	other, unsubscribe := hub.Subscribe(workspaceID, otherID)
	defer unsubscribe()
	hub.publish(workspaceID, Event{
		ID: "private-chat", Type: "chat.message.created", Audience: AudienceUser,
		RecipientUserID: recipientID, Data: []byte(`{"body":"private"}`),
	})
	select {
	case event := <-other:
		t.Fatalf("other member received private domain event %+v", event)
	default:
	}
}
