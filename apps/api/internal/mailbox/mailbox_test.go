package mailbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/mail"
	"strings"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestEndpointPolicyRejectsUnsafeTargetsAndPorts(t *testing.T) {
	t.Parallel()
	policy := DefaultEndpointPolicy()
	for _, test := range []struct {
		host     string
		port     int
		protocol string
	}{
		{"http://mail.example", 993, "imap"},
		{"mail.example/path", 993, "imap"},
		{"mail.example", 25, "smtp"},
		{"mail.example", 993, "smtp"},
		{"mail..example", 993, "imap"},
		{"127.0.0.1", 993, "imap"},
	} {
		if err := policy.Validate(test.host, test.port, test.protocol); err == nil {
			t.Errorf("Validate(%q,%d,%q) unexpectedly succeeded", test.host, test.port, test.protocol)
		}
	}
	if err := policy.Validate("imap.example.test", 993, "imap"); err != nil {
		t.Fatalf("valid IMAP endpoint rejected: %v", err)
	}
	if err := policy.Validate("smtp.example.test", 587, "smtp"); err != nil {
		t.Fatalf("valid SMTP endpoint rejected: %v", err)
	}
	for _, raw := range []string{"127.0.0.1", "::1", "169.254.169.254", "224.0.0.1", "0.0.0.0"} {
		if endpointIPAllowed(net.ParseIP(raw), true) {
			t.Errorf("unsafe IP %s was allowed", raw)
		}
	}
	if endpointIPAllowed(net.ParseIP("10.1.2.3"), false) {
		t.Fatal("private IP allowed without explicit policy")
	}
	if !endpointIPAllowed(net.ParseIP("10.1.2.3"), true) {
		t.Fatal("explicit private corporate policy did not allow private IP")
	}
}

func TestCredentialCiphertextIsBoundToAccountOwner(t *testing.T) {
	t.Parallel()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x72}, 32))
	cipher, err := identity.NewAESGCMKeyringFromBase64("test-key", key)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{cipher: cipher}
	workspaceID, accountID, ownerID := mustTestID(t), mustTestID(t), mustTestID(t)
	envelope, err := service.encryptCredential(context.Background(), workspaceID, accountID, ownerID, "app-password")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(envelope.Ciphertext, []byte("app-password")) {
		t.Fatal("ciphertext contains plaintext password")
	}
	if _, err := cipher.Decrypt(context.Background(), credentialPurpose,
		credentialSubject(workspaceID, accountID, mustTestID(t)), envelope); err == nil {
		t.Fatal("another user could decrypt the account credential")
	}
}

func TestParsePlainTextUsesTextPartAndNeverReturnsRemoteHTML(t *testing.T) {
	t.Parallel()
	raw := strings.Join([]string{
		"From: Alice <alice@example.test>",
		"To: Bob <bob@example.test>",
		"Subject: Safe body",
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="safe-boundary"`,
		"", "--safe-boundary", "Content-Type: text/html; charset=UTF-8", "",
		`<img src="https://tracker.invalid/pixel"><script>alert(1)</script>`,
		"--safe-boundary", "Content-Type: text/plain; charset=UTF-8", "", "Hello", "World",
		"--safe-boundary--", "",
	}, "\r\n")
	plain, err := parsePlainText(strings.NewReader(raw), MaxMessageBytes, MaxBodyBytes)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "Hello\nWorld" || strings.Contains(plain, "script") || strings.Contains(plain, "tracker") {
		t.Fatalf("unexpected safe body %q", plain)
	}

	htmlOnly := "MIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n<script>alert(1)</script>"
	plain, err = parsePlainText(strings.NewReader(htmlOnly), MaxMessageBytes, MaxBodyBytes)
	if err != nil || plain != "" {
		t.Fatalf("HTML-only body = %q, %v; want empty safe text", plain, err)
	}
}

func TestMIMEAndSubmissionBoundsRejectInjectionAndOversize(t *testing.T) {
	t.Parallel()
	if _, err := buildPlainMessage(mail.Address{Address: "from@example.test"}, "id@example.test",
		SendInput{Recipients: RecipientSet{To: []Address{{Address: "to@example.test"}}}, Subject: "ok\r\nBcc: stolen@example.test"}); !errors.Is(err, ErrMalformedMessage) {
		t.Fatalf("header injection error=%v", err)
	}
	if _, err := validatedRecipients(RecipientSet{To: []Address{{Address: "to@example.test\r\nRCPT TO:<other@example.test>"}}}); !errors.Is(err, ErrMalformedMessage) {
		t.Fatalf("recipient injection error=%v", err)
	}
	large := "Content-Type: text/plain\r\n\r\n" + strings.Repeat("x", 1025)
	if _, err := parsePlainText(strings.NewReader(large), 2048, 1024); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("oversize MIME error=%v", err)
	}

	payload, err := buildPlainMessage(mail.Address{Name: "Sender", Address: "from@example.test"}, "id@example.test",
		SendInput{Recipients: RecipientSet{To: []Address{{Name: "Recipient", Address: "to@example.test"}}}, Subject: "Hello", PlainText: "Safe body"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if subject := parsed.Header.Get("Subject"); subject != "Hello" {
		t.Fatalf("subject=%q", subject)
	}
}

func TestPersistenceStatesAreMappedToStableAPIVocabulary(t *testing.T) {
	t.Parallel()
	for databaseState, expected := range map[string]string{
		"pending": "idle", "disabled": "idle", "syncing": "syncing", "ready": "ready", "error": "error",
	} {
		if actual := publicSyncState(databaseState); actual != expected {
			t.Fatalf("publicSyncState(%q)=%q, want %q", databaseState, actual, expected)
		}
	}
	for databaseState, expected := range map[string]string{
		"missing": "metadata", "queued": "metadata", "ready": "cached", "error": "unavailable",
	} {
		if actual := publicBodyState(databaseState); actual != expected {
			t.Fatalf("publicBodyState(%q)=%q, want %q", databaseState, actual, expected)
		}
	}
}

func mustTestID(t *testing.T) ids.UUID {
	t.Helper()
	value, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
