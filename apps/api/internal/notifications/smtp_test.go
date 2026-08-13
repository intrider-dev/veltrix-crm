package notifications

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteNotificationEmailHasSafeDeterministicMessageID(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := writeNotificationEmail(&output, "CRM <noreply@example.invalid>", "user@example.invalid", EmailMessage{
		ID: "018f0000-0000-7000-8000-000000000001", Subject: "Напоминание", TextBody: "Line 1\nLine 2",
	})
	if err != nil {
		t.Fatal(err)
	}
	contents := output.String()
	if !strings.Contains(contents, "Message-ID: <018f0000-0000-7000-8000-000000000001@notifications.invalid>\r\n") {
		t.Fatalf("message ID is absent: %s", contents)
	}
	if !strings.Contains(contents, "Line 1\r\nLine 2") {
		t.Fatalf("body is not CRLF-normalized: %q", contents)
	}
}

func TestWriteNotificationEmailRejectsHeaderInjection(t *testing.T) {
	t.Parallel()
	err := writeNotificationEmail(&bytes.Buffer{}, "a@example.invalid", "b@example.invalid", EmailMessage{
		ID: "safe", Subject: "ok\r\nBcc: attacker@example.invalid",
	})
	if err == nil {
		t.Fatal("header injection was accepted")
	}
}
