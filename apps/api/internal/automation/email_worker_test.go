package automation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

type staticEmailResolver struct {
	message ResolvedAutomationEmail
	called  int
}

func (resolver *staticEmailResolver) Resolve(_ context.Context, _ worker.Job, _ emailJobPayload) (ResolvedAutomationEmail, error) {
	resolver.called++
	return resolver.message, nil
}

type recordingEmailSender struct {
	messages []notifications.EmailMessage
}

func (sender *recordingEmailSender) Send(_ context.Context, message notifications.EmailMessage) error {
	sender.messages = append(sender.messages, message)
	return nil
}

func TestAutomationEmailWorkerUsesStableMessageID(t *testing.T) {
	jobID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b4")
	targetID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	payload, _ := json.Marshal(emailJobPayload{
		TargetType: EntityContact, TargetID: targetID.String(), RecipientField: "email",
		TemplateKey: "followup", TemplateParams: map[string]any{"name": "Ada"},
	})
	resolver := &staticEmailResolver{message: ResolvedAutomationEmail{
		Recipient: "ada@example.test", Subject: "Hello", Body: "Body",
	}}
	sender := &recordingEmailSender{}
	handler := NewEmailWorkerHandler(resolver, sender)
	job := worker.Job{ID: jobID, SchemaVersion: 1, Payload: payload}
	for range 2 {
		if err := handler(context.Background(), worker.Dependencies{}, job); err != nil {
			t.Fatalf("send automation email: %v", err)
		}
	}
	if resolver.called != 2 || len(sender.messages) != 2 {
		t.Fatalf("unexpected calls: resolver=%d messages=%d", resolver.called, len(sender.messages))
	}
	for _, message := range sender.messages {
		if message.ID != jobID.String() {
			t.Fatalf("unstable message ID %q", message.ID)
		}
	}
}

func TestAutomationEmailWorkerRejectsUnsafePayload(t *testing.T) {
	targetID := ids.MustParse("018f47d2-2044-7f25-89b0-85bd4c8ad8b5")
	for _, payload := range []emailJobPayload{
		{TargetType: EntityWorkspace, TargetID: targetID.String(), RecipientField: "email", TemplateKey: "followup"},
		{TargetType: EntityContact, TargetID: targetID.String(), RecipientField: "password", TemplateKey: "followup"},
		{TargetType: EntityContact, TargetID: targetID.String(), RecipientField: "email", TemplateKey: "Bad Key"},
		{TargetType: EntityContact, TargetID: targetID.String(), RecipientField: "email", TemplateKey: "followup", TemplateParams: map[string]any{"nested": map[string]any{"secret": true}}},
	} {
		raw, _ := json.Marshal(payload)
		if _, err := decodeEmailJobPayload(worker.Job{SchemaVersion: 1, Payload: raw}); err == nil {
			t.Fatalf("expected payload rejection: %+v", payload)
		}
	}
}

func TestRenderAutomationTemplateRequiresEveryPlaceholder(t *testing.T) {
	rendered, err := renderAutomationTemplate("Hello {name} from {workspaceName}", map[string]any{
		"name": "Ada", "workspaceName": "Lab",
	})
	if err != nil || rendered != "Hello Ada from Lab" {
		t.Fatalf("unexpected render %q: %v", rendered, err)
	}
	if _, err := renderAutomationTemplate("Hello {name}", map[string]any{}); err == nil {
		t.Fatal("expected missing placeholder failure")
	}
}
