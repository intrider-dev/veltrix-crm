package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // RFC 4226/6238 requires HMAC-SHA-1 for interoperable defaults.
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	totpPeriodSeconds = int64(30)
	totpDigits        = 6
	recoveryCodeCount = 10
)

func newTOTPSecret() ([]byte, string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate TOTP secret: %w", err)
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return raw, encoded, nil
}

func buildTOTPUri(issuer, account, encodedSecret string) (string, error) {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	if issuer == "" || account == "" {
		return "", errors.New("TOTP issuer and account are required")
	}
	values := url.Values{}
	values.Set("secret", encodedSecret)
	values.Set("issuer", issuer)
	values.Set("algorithm", "SHA1")
	values.Set("digits", strconv.Itoa(totpDigits))
	values.Set("period", strconv.FormatInt(totpPeriodSeconds, 10))
	uri := url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + issuer + ":" + account,
		RawQuery: values.Encode(),
	}
	return uri.String(), nil
}

// matchTOTP returns the matching time step. Calling code must persist and
// compare the step to reject replay of an already accepted code.
func matchTOTP(secret []byte, code string, at time.Time, allowedDrift int) (int64, bool) {
	if allowedDrift < 0 || len(code) != totpDigits {
		return 0, false
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	currentStep := at.Unix() / totpPeriodSeconds
	for offset := -allowedDrift; offset <= allowedDrift; offset++ {
		step := currentStep + int64(offset)
		if step < 0 {
			continue
		}
		expected := hmacOneTimePassword(secret, uint64(step), totpDigits)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func hmacOneTimePassword(secret []byte, counter uint64, digits int) string {
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counterBytes[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	modulus := uint32(1)
	for range digits {
		modulus *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%modulus)
}

func newRecoveryCodes() ([]string, [][32]byte, error) {
	codes := make([]string, 0, recoveryCodeCount)
	hashes := make([][32]byte, 0, recoveryCodeCount)
	for range recoveryCodeCount {
		var raw [10]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		compact := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:])
		code := strings.Join([]string{compact[0:4], compact[4:8], compact[8:12], compact[12:16]}, "-")
		codes = append(codes, code)
		hashes = append(hashes, hashRecoveryCode(code))
	}
	return codes, hashes, nil
}

func hashRecoveryCode(code string) [32]byte {
	normalized := normalizeRecoveryCode(code)
	return sha256.Sum256([]byte("crm-recovery-code-v1\x00" + normalized))
}

func normalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}
