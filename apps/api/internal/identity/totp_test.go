package identity

import (
	"encoding/base32"
	"net/url"
	"testing"
	"time"
)

func TestHOTPMatchesRFC4226Vectors(t *testing.T) {
	t.Parallel()
	secret := []byte("12345678901234567890")
	want := []string{"755224", "287082", "359152", "969429", "338314", "254676", "287922", "162583", "399871", "520489"}
	for counter, expected := range want {
		if got := hmacOneTimePassword(secret, uint64(counter), 6); got != expected {
			t.Errorf("counter %d = %s, want %s", counter, got, expected)
		}
	}
}

func TestTOTPMatchesRFC6238SHA1Vectors(t *testing.T) {
	t.Parallel()
	secret := []byte("12345678901234567890")
	tests := []struct {
		unix int64
		want string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, test := range tests {
		step := uint64(test.unix / totpPeriodSeconds)
		if got := hmacOneTimePassword(secret, step, 8); got != test.want {
			t.Errorf("unix %d = %s, want %s", test.unix, got, test.want)
		}
	}
}

func TestMatchTOTPDriftAndShape(t *testing.T) {
	t.Parallel()
	secret := []byte("12345678901234567890")
	at := time.Unix(1_700_000_000, 0)
	previousCode := hmacOneTimePassword(secret, uint64(at.Unix()/30-1), 6)
	step, ok := matchTOTP(secret, previousCode, at, 1)
	if !ok || step != at.Unix()/30-1 {
		t.Fatalf("previous step rejected: step=%d ok=%v", step, ok)
	}
	for _, code := range []string{"12345", "1234567", "12x456", " 123456"} {
		if _, ok := matchTOTP(secret, code, at, 1); ok {
			t.Fatalf("malformed code %q accepted", code)
		}
	}
}

func TestBuildTOTPURIIsInteroperable(t *testing.T) {
	t.Parallel()
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	raw, err := buildTOTPUri("CRM Lab", "user@example.test", secret)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "otpauth" || parsed.Host != "totp" || parsed.Query().Get("secret") != secret || parsed.Query().Get("issuer") != "CRM Lab" {
		t.Fatalf("unexpected URI: %s", raw)
	}
}

func TestRecoveryCodesNormalizeAndHash(t *testing.T) {
	t.Parallel()
	left := hashRecoveryCode("abcd-efgh-2345-6789")
	right := hashRecoveryCode(" ABCDEFGH23456789 ")
	if left != right {
		t.Fatal("equivalent recovery codes have different hashes")
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != recoveryCodeCount || len(hashes) != recoveryCodeCount {
		t.Fatalf("got %d codes and %d hashes", len(codes), len(hashes))
	}
	seen := map[string]struct{}{}
	for _, code := range codes {
		if len(code) != 19 {
			t.Fatalf("unexpected recovery code shape %q", code)
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate recovery code %q", code)
		}
		seen[code] = struct{}{}
	}
}
