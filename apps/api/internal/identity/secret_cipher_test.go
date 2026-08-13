package identity

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestAESGCMKeyringRoundTripAndAADBinding(t *testing.T) {
	t.Parallel()
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	keyring, err := NewAESGCMKeyringFromBase64("test-2026-01", encoded)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := keyring.Encrypt(context.Background(), "totp", "user-1", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := keyring.Decrypt(context.Background(), "totp", "user-1", envelope)
	if err != nil || string(plaintext) != "secret" {
		t.Fatalf("round trip = %q, %v", plaintext, err)
	}
	if _, err := keyring.Decrypt(context.Background(), "totp", "user-2", envelope); err == nil {
		t.Fatal("ciphertext was not bound to subject")
	}
	envelope.Ciphertext[0] ^= 0xff
	if _, err := keyring.Decrypt(context.Background(), "totp", "user-1", envelope); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestAESGCMKeyringRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	if _, err := NewAESGCMKeyringFromBase64("key", "not-base64"); err == nil {
		t.Fatal("invalid base64 key accepted")
	}
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := NewAESGCMKeyringFromBase64("key", short); err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("short key error = %v", err)
	}
	valid := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	if _, err := NewAESGCMKeyringFromBase64("bad key id", valid); err == nil {
		t.Fatal("unsafe key id accepted")
	}
}
