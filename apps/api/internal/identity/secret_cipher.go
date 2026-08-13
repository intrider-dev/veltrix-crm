package identity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// SecretEnvelope is safe to persist. Plaintext secrets and encryption keys must
// never be stored alongside it or written to logs.
type SecretEnvelope struct {
	Ciphertext []byte
	Nonce      []byte
	KeyID      string
}

// SecretCipher keeps encryption policy outside identity business logic and
// allows key rotation by retaining decrypt-only historical keys.
type SecretCipher interface {
	Encrypt(ctx context.Context, purpose, subject string, plaintext []byte) (SecretEnvelope, error)
	Decrypt(ctx context.Context, purpose, subject string, envelope SecretEnvelope) ([]byte, error)
}

type AESGCMKeyring struct {
	activeKeyID string
	keys        map[string][32]byte
}

// NewAESGCMKeyring builds an AES-256-GCM keyring from base64-encoded 32-byte
// keys. The active key encrypts new values; all keys may decrypt existing data.
func NewAESGCMKeyring(activeKeyID string, encodedKeys map[string]string) (*AESGCMKeyring, error) {
	if activeKeyID == "" {
		return nil, errors.New("active encryption key id is required")
	}
	keys := make(map[string][32]byte, len(encodedKeys))
	for keyID, encoded := range encodedKeys {
		if !validKeyID(keyID) {
			return nil, errors.New("encryption key id must use 1-64 ASCII letters, digits, dot, dash, or underscore")
		}
		decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("encryption key %q must be standard base64 encoding of 32 bytes", keyID)
		}
		var key [32]byte
		copy(key[:], decoded)
		keys[keyID] = key
		clear(decoded)
	}
	if _, ok := keys[activeKeyID]; !ok {
		return nil, fmt.Errorf("active encryption key %q is not present", activeKeyID)
	}
	return &AESGCMKeyring{activeKeyID: activeKeyID, keys: keys}, nil
}

func validKeyID(keyID string) bool {
	if len(keyID) < 1 || len(keyID) > 64 {
		return false
	}
	for _, character := range keyID {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '.' && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func NewAESGCMKeyringFromBase64(keyID, encodedKey string) (*AESGCMKeyring, error) {
	return NewAESGCMKeyring(keyID, map[string]string{keyID: encodedKey})
}

func (keyring *AESGCMKeyring) Encrypt(
	_ context.Context,
	purpose string,
	subject string,
	plaintext []byte,
) (SecretEnvelope, error) {
	key, ok := keyring.keys[keyring.activeKeyID]
	if !ok {
		return SecretEnvelope{}, errors.New("active encryption key is unavailable")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return SecretEnvelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return SecretEnvelope{}, fmt.Errorf("generate encryption nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, secretAAD(purpose, subject))
	return SecretEnvelope{Ciphertext: ciphertext, Nonce: nonce, KeyID: keyring.activeKeyID}, nil
}

func (keyring *AESGCMKeyring) Decrypt(
	_ context.Context,
	purpose string,
	subject string,
	envelope SecretEnvelope,
) ([]byte, error) {
	key, ok := keyring.keys[envelope.KeyID]
	if !ok {
		return nil, errors.New("secret encryption key is unavailable")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(envelope.Nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid secret nonce")
	}
	plaintext, err := gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, secretAAD(purpose, subject))
	if err != nil {
		return nil, errors.New("secret authentication failed")
	}
	return plaintext, nil
}

func newGCM(key [32]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM cipher: %w", err)
	}
	return gcm, nil
}

func secretAAD(purpose, subject string) []byte {
	return []byte("crm-secret-v1\x00" + purpose + "\x00" + subject)
}
