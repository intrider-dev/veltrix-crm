package calls

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrProviderDisabled = errors.New("calls provider is disabled")

type JoinGrant struct {
	URL       string
	Token     string
	ExpiresAt time.Time
}

type Provider interface {
	Enabled() bool
	JoinToken(roomName, identity, displayName string) (JoinGrant, error)
}

type DisabledProvider struct{}

func (DisabledProvider) Enabled() bool { return false }
func (DisabledProvider) JoinToken(_, _, _ string) (JoinGrant, error) {
	return JoinGrant{}, ErrProviderDisabled
}

type LiveKitProvider struct {
	publicURL string
	apiKey    string
	secret    string
	ttl       time.Duration
}

func NewLiveKitProvider(publicURL, apiKey, secret string, ttl time.Duration) (*LiveKitProvider, error) {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if publicURL == "" || strings.TrimSpace(apiKey) == "" || secret == "" {
		return nil, errors.New("LiveKit URL, API key, and secret are required")
	}
	if ttl < time.Minute || ttl > 10*time.Minute {
		return nil, errors.New("LiveKit token TTL must be between 1m and 10m")
	}
	return &LiveKitProvider{publicURL: publicURL, apiKey: apiKey, secret: secret, ttl: ttl}, nil
}

func (*LiveKitProvider) Enabled() bool { return true }

func (provider *LiveKitProvider) JoinToken(roomName, identity, displayName string) (JoinGrant, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(provider.ttl)
	claims := liveKitClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: provider.apiKey, Subject: identity,
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Name: displayName,
		Video: liveKitVideoGrant{
			RoomJoin: true, Room: roomName, CanPublish: true, CanSubscribe: true,
		},
	}
	encoded, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(provider.secret))
	if err != nil {
		return JoinGrant{}, err
	}
	return JoinGrant{URL: provider.publicURL, Token: encoded, ExpiresAt: expiresAt}, nil
}

type liveKitClaims struct {
	jwt.RegisteredClaims
	Name  string            `json:"name,omitempty"`
	Video liveKitVideoGrant `json:"video"`
}

type liveKitVideoGrant struct {
	RoomJoin     bool   `json:"roomJoin"`
	Room         string `json:"room"`
	CanPublish   bool   `json:"canPublish"`
	CanSubscribe bool   `json:"canSubscribe"`
}
