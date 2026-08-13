package calls

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDisabledProviderNeverIssuesGrant(t *testing.T) {
	provider := DisabledProvider{}
	if provider.Enabled() {
		t.Fatal("disabled provider reported enabled")
	}
	if _, err := provider.JoinToken("room", "user", "User"); !errors.Is(err, ErrProviderDisabled) {
		t.Fatalf("JoinToken error = %v, want disabled", err)
	}
}

func TestLiveKitProviderIssuesShortLivedRoomScopedGrant(t *testing.T) {
	provider, err := NewLiveKitProvider("wss://calls.example.test/", "public-key", "private-secret", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	grant, err := provider.JoinToken("room-7", "user-9", "Pat Example")
	if err != nil {
		t.Fatal(err)
	}
	if grant.URL != "wss://calls.example.test" {
		t.Fatalf("URL = %q", grant.URL)
	}
	claims := &liveKitClaims{}
	parsed, err := jwt.ParseWithClaims(grant.Token, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			t.Fatalf("signing method = %v", token.Method.Alg())
		}
		return []byte("private-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse grant: token valid=%v error=%v", parsed != nil && parsed.Valid, err)
	}
	if claims.Issuer != "public-key" || claims.Subject != "user-9" || claims.Name != "Pat Example" {
		t.Fatalf("identity claims = %+v", claims)
	}
	if !claims.Video.RoomJoin || claims.Video.Room != "room-7" || !claims.Video.CanPublish || !claims.Video.CanSubscribe {
		t.Fatalf("video grant = %+v", claims.Video)
	}
	if grant.ExpiresAt.Before(before.Add(119*time.Second)) || grant.ExpiresAt.After(before.Add(121*time.Second)) {
		t.Fatalf("expiry = %s, want approximately two minutes", grant.ExpiresAt)
	}
}

func TestLiveKitProviderRejectsUnsafeTTL(t *testing.T) {
	for _, ttl := range []time.Duration{30 * time.Second, 11 * time.Minute} {
		if _, err := NewLiveKitProvider("wss://calls.example.test", "key", "secret", ttl); err == nil {
			t.Fatalf("TTL %s accepted", ttl)
		}
	}
}
