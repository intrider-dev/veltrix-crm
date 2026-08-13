package identity

import (
	"testing"
	"unicode/utf8"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hasher := NewPasswordHasher(1)
	encoded, err := hasher.Hash("Demo123!")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := hasher.Verify("Demo123!", encoded)
	if err != nil || !valid {
		t.Fatalf("valid password rejected: valid=%v err=%v", valid, err)
	}
	valid, err = hasher.Verify("wrong-password", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("wrong password accepted")
	}
}

func TestBoundedUserAgentPreservesUTF8AndRuneLimit(t *testing.T) {
	t.Parallel()
	input := "\xffЖ"
	for range 600 {
		input += "界"
	}
	got := boundedUserAgent(input)
	if !utf8.ValidString(got) {
		t.Fatal("bounded user agent is not valid UTF-8")
	}
	if count := utf8.RuneCountInString(got); count != 512 {
		t.Fatalf("got %d runes, want 512", count)
	}
}

func TestPasswordHashRejectsUnsafeInput(t *testing.T) {
	hasher := NewPasswordHasher(1)
	if _, err := hasher.Hash("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if _, err := hasher.Verify("x", "not-a-hash"); err == nil {
		t.Fatal("malformed hash accepted")
	}
	tooLong := string(make([]byte, 1025))
	if valid, err := hasher.Verify(tooLong, "$argon2id$v=19$m=32768,t=2,p=1$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"); err != nil || valid {
		t.Fatalf("oversized candidate: valid=%v err=%v", valid, err)
	}
}
