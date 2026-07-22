package identity

import (
	"bytes"
	"strings"
	"testing"
)

func TestWritePlainTextEmailEncodesUnicodeSubject(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := writePlainTextEmail(&output, "CRM <no-reply@example.test>", "User <user@example.test>", "Сброс пароля", "Line one\nLine two")
	if err != nil {
		t.Fatal(err)
	}
	raw := output.String()
	if !strings.Contains(raw, "Subject: =?UTF-8?") || !strings.Contains(raw, "Line one\r\nLine two") {
		t.Fatalf("unexpected message:\n%s", raw)
	}
}

func TestWritePlainTextEmailRejectsHeaderInjection(t *testing.T) {
	t.Parallel()
	if err := writePlainTextEmail(&bytes.Buffer{}, "sender@example.test", "user@example.test", "safe\r\nBcc: attacker@example.test", "body"); err == nil {
		t.Fatal("header injection was accepted")
	}
}
