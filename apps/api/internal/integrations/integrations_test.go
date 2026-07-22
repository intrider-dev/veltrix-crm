package integrations

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestAPIKeyGeneratedOnceAndAuthenticatedByHash(t *testing.T) {
	t.Parallel()
	repository := &apiKeyRepositoryStub{}
	service := NewAPIKeyService(repository)
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	workspaceID := integrationID(t)
	created, err := service.Create(context.Background(), APIKeyCreate{
		WorkspaceID: workspaceID, CreatedBy: integrationID(t), Name: "Reporting export",
		Scopes: []Scope{ScopeReportsRead, ScopeContactsRead, ScopeReportsRead}, Now: now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Token == "" || repository.hash == ([32]byte{}) {
		t.Fatal("Create() did not return a token/store its hash")
	}
	if strings.Contains(string(repository.hash[:]), created.Token) {
		t.Fatal("stored hash contains the plaintext token")
	}
	authenticated, err := service.Authenticate(context.Background(), created.Token, now)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if authenticated.WorkspaceID != workspaceID || !HasScope(authenticated.Scopes, ScopeReportsRead) || repository.touches != 1 {
		t.Fatalf("Authenticate() = %+v touches=%d", authenticated, repository.touches)
	}
	parts := strings.Split(created.Token, ".")
	parts[3] = strings.Repeat("A", len(parts[3]))
	if _, err := service.Authenticate(context.Background(), strings.Join(parts, "."), now); !errors.Is(err, errx.ErrUnauthenticated) {
		t.Fatalf("tampered key error = %v, want unauthenticated", err)
	}
}

func TestExpiredAndRevokedAPIKeysAreRejected(t *testing.T) {
	t.Parallel()
	repository := &apiKeyRepositoryStub{}
	service := NewAPIKeyService(repository)
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	created, err := service.Create(context.Background(), APIKeyCreate{
		WorkspaceID: integrationID(t), CreatedBy: integrationID(t), Name: "Short lived",
		Scopes: []Scope{ScopeContactsRead}, ExpiresAt: &expires, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), created.Token, expires.Add(time.Second)); !errors.Is(err, errx.ErrUnauthenticated) {
		t.Fatalf("expired key error = %v", err)
	}
	repository.credential.ExpiresAt = nil
	revoked := now
	repository.credential.RevokedAt = &revoked
	if _, err := service.Authenticate(context.Background(), created.Token, now); !errors.Is(err, errx.ErrUnauthenticated) {
		t.Fatalf("revoked key error = %v", err)
	}
}

func TestAPIKeyHTTPRequiresWorkspaceAndScope(t *testing.T) {
	t.Parallel()
	repository := &apiKeyRepositoryStub{}
	service := NewAPIKeyService(repository)
	workspaceID := integrationID(t)
	created, err := service.Create(context.Background(), APIKeyCreate{
		WorkspaceID: workspaceID, CreatedBy: integrationID(t), Name: "Contacts reader",
		Scopes: []Scope{ScopeContactsRead}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	authenticator := NewAPIKeyHTTPAuthenticator(service, nil)
	router := chi.NewRouter()
	router.Route("/workspaces/{workspaceId}", func(route chi.Router) {
		route.With(func(next http.Handler) http.Handler { return authenticator.Require(ScopeContactsRead, next) }).
			Get("/contacts", func(writer http.ResponseWriter, request *http.Request) {
				if _, ok := APIKeyFromContext(request.Context()); !ok {
					t.Error("API key context missing")
				}
				if _, ok := httpx.Principal(request.Context()); !ok {
					t.Error("principal context missing")
				}
				writer.WriteHeader(http.StatusNoContent)
			})
		route.With(func(next http.Handler) http.Handler { return authenticator.Require(ScopeDealsWrite, next) }).
			Post("/deals", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })
	})

	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String()+"/contacts", nil)
	request.Header.Set("Authorization", "Bearer "+created.Token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized response = %d", response.Code)
	}

	other := httptest.NewRequest(http.MethodGet, "/workspaces/"+integrationID(t).String()+"/contacts", nil)
	other.Header.Set("Authorization", "Bearer "+created.Token)
	otherResponse := httptest.NewRecorder()
	router.ServeHTTP(otherResponse, other)
	if otherResponse.Code != http.StatusForbidden {
		t.Fatalf("workspace mismatch response = %d", otherResponse.Code)
	}

	denied := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID.String()+"/deals", nil)
	denied.Header.Set("Authorization", "Bearer "+created.Token)
	deniedResponse := httptest.NewRecorder()
	router.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("scope mismatch response = %d", deniedResponse.Code)
	}
}

func TestWebhookSignatureTimestampAndReplayProtection(t *testing.T) {
	t.Parallel()
	secret := []byte("01234567890123456789012345678901")
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"data":{"status":"won"}}`)
	signature := SignWebhook(secret, now, "evt-1", body)
	replay := &replayStoreStub{}
	if err := VerifyWebhook(context.Background(), secret, now, 5*time.Minute, "evt-1", "1784635200", signature, body, replay); err != nil {
		t.Fatalf("VerifyWebhook() error = %v", err)
	}
	if err := VerifyWebhook(context.Background(), secret, now, 5*time.Minute, "evt-1", "1784635200", signature, body, replay); err == nil {
		t.Fatal("VerifyWebhook() accepted replay")
	}
	if err := VerifyWebhook(context.Background(), secret, now, 5*time.Minute, "evt-2", "1784635200", signature, body, nil); err == nil {
		t.Fatal("VerifyWebhook() accepted signature for another event")
	}
	if err := VerifyWebhook(context.Background(), secret, now, 5*time.Minute, "evt-3", "1784635200",
		"v1="+strings.Repeat("0", 64)+","+SignWebhook(secret, now, "evt-3", body), body, nil); err != nil {
		t.Fatalf("VerifyWebhook() rejected a valid rotation signature: %v", err)
	}
}

func TestURLValidatorRejectsPrivateAndMixedDNSAnswers(t *testing.T) {
	t.Parallel()
	validator := URLValidator{Resolver: resolverStub{addresses: []netip.Addr{netip.MustParseAddr("10.0.0.4")}}}
	if _, err := validator.Validate(context.Background(), "https://hooks.example.test/crm"); err == nil {
		t.Fatal("Validate() accepted private address")
	}
	validator.Resolver = resolverStub{addresses: []netip.Addr{
		netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("127.0.0.1"),
	}}
	if _, err := validator.Validate(context.Background(), "https://hooks.example.test/crm"); err == nil {
		t.Fatal("Validate() accepted mixed public/private DNS response")
	}
	validator.Resolver = resolverStub{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}
	endpoint, err := validator.Validate(context.Background(), "https://hooks.example.test/crm")
	if err != nil || endpoint.Port != "443" {
		t.Fatalf("Validate() = %+v, %v", endpoint, err)
	}
}

func TestWebhookResponseSummaryRedactsSecretsAndPII(t *testing.T) {
	t.Parallel()
	input := `{"authorization":"Bearer bearer-secret-value","message":"Bearer standalone-bearer-secret","password":"correct-horse-battery-staple",` +
		`"api_key":"api-key-value","email":"ada@example.test","phone":"+1 (555) 123-4567",` +
		`"webhook":"whsec_abcdefghijklmnopqrstuvwxyz012345"}`
	summary := sanitizeResponseSummary(input)
	for _, sensitive := range []string{
		"bearer-secret-value", "standalone-bearer-secret", "correct-horse-battery-staple", "api-key-value",
		"ada@example.test", "+1 (555) 123-4567", "whsec_abcdefghijklmnopqrstuvwxyz012345",
	} {
		if strings.Contains(summary, sensitive) {
			t.Fatalf("sanitizeResponseSummary() retained sensitive value %q in %q", sensitive, summary)
		}
	}
	for _, marker := range []string{`"authorization":"[redacted]"`, "Bearer [redacted]", `"password":"[redacted]"`, "[email redacted]", "[phone redacted]", "whsec_[redacted]"} {
		if !strings.Contains(summary, marker) {
			t.Fatalf("sanitizeResponseSummary() = %q, missing %q", summary, marker)
		}
	}
}

func TestWebhookResponseSummaryIsPrintableBoundedUTF8(t *testing.T) {
	t.Parallel()
	input := " accepted\r\n\t" + strings.Repeat("Ж", maxWebhookResponseSummaryBytes) + "\x00tail"
	summary := sanitizeResponseSummary(input)
	if !utf8.ValidString(summary) {
		t.Fatalf("sanitizeResponseSummary() returned invalid UTF-8")
	}
	if len(summary) > maxWebhookResponseSummaryBytes {
		t.Fatalf("sanitizeResponseSummary() length = %d, want <= %d", len(summary), maxWebhookResponseSummaryBytes)
	}
	if strings.ContainsAny(summary, "\r\n\t\x00") {
		t.Fatalf("sanitizeResponseSummary() retained control characters: %q", summary)
	}
}

func TestWebhookSecretIsEncryptedAndRotationReturnsOnlyNewSecret(t *testing.T) {
	t.Parallel()
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	cipher, err := identity.NewAESGCMKeyringFromBase64("primary", key)
	if err != nil {
		t.Fatal(err)
	}
	repository := &webhookRepositoryStub{}
	service := NewWebhookService(repository, cipher, URLValidator{
		Resolver: resolverStub{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}},
	})
	created, err := service.Create(context.Background(), WebhookCreate{
		WorkspaceID: integrationID(t), CreatedBy: integrationID(t), URL: "https://hooks.example.test/crm",
		EventTypes: []string{"sales.deal.stage_changed"}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(created.SigningSecret, "whsec_") || string(repository.envelope.Ciphertext) == created.SigningSecret {
		t.Fatal("webhook secret was not returned/encrypted as expected")
	}
	rotated, err := service.RotateSecret(context.Background(), created.Subscription.WorkspaceID, created.Subscription.ID, created.Subscription.Version, time.Hour)
	if err != nil {
		t.Fatalf("RotateSecret() error = %v", err)
	}
	if rotated.SigningSecret == created.SigningSecret || repository.overlap != time.Hour {
		t.Fatal("RotateSecret() did not create a fresh overlapping secret")
	}
}

func TestWebhookDeliveryLogIsBoundedAndCursorEncoded(t *testing.T) {
	t.Parallel()
	workspaceID := integrationID(t)
	subscriptionID := integrationID(t)
	now := time.Now().UTC()
	repository := &webhookRepositoryStub{deliveries: []WebhookDeliveryLog{
		{ID: integrationID(t), SubscriptionID: subscriptionID, EventID: integrationID(t), Status: "succeeded", CreatedAt: now},
		{ID: integrationID(t), SubscriptionID: subscriptionID, EventID: integrationID(t), Status: "failed", CreatedAt: now.Add(-time.Second)},
		{ID: integrationID(t), SubscriptionID: subscriptionID, EventID: integrationID(t), Status: "queued", CreatedAt: now.Add(-2 * time.Second)},
	}}
	service := NewWebhookService(repository, nil, URLValidator{})
	page, err := service.ListDeliveries(context.Background(), workspaceID, &subscriptionID, "", 2)
	if err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if len(page.Items) != 2 || page.NextCursor == "" || repository.deliveryLimit != 3 {
		t.Fatalf("unexpected page: items=%d cursor=%q repositoryLimit=%d", len(page.Items), page.NextCursor, repository.deliveryLimit)
	}
	if _, err := service.ListDeliveries(context.Background(), workspaceID, &subscriptionID, "invalid", 2); err == nil {
		t.Fatal("invalid delivery cursor was accepted")
	}
}

type apiKeyRepositoryStub struct {
	credential APIKeyCredential
	hash       [32]byte
	key        APIKey
	touches    int
}

func (repository *apiKeyRepositoryStub) CreateAPIKey(_ context.Context, key APIKey, hash [32]byte) (APIKey, error) {
	repository.key, repository.hash = key, hash
	repository.credential = APIKeyCredential{
		WorkspaceID: key.WorkspaceID, ID: key.ID, Prefix: key.Prefix, SecretHash: hash,
		Scopes: key.Scopes, CreatedBy: key.CreatedBy, ExpiresAt: key.ExpiresAt,
	}
	return key, nil
}
func (repository *apiKeyRepositoryStub) ListAPIKeys(context.Context, ids.UUID, int) ([]APIKey, error) {
	return []APIKey{repository.key}, nil
}
func (repository *apiKeyRepositoryStub) LookupAPIKey(_ context.Context, workspaceID ids.UUID, prefix string) (APIKeyCredential, bool, error) {
	return repository.credential, repository.credential.WorkspaceID == workspaceID && repository.credential.Prefix == prefix, nil
}
func (repository *apiKeyRepositoryStub) TouchAPIKey(context.Context, ids.UUID, ids.UUID) error {
	repository.touches++
	return nil
}
func (repository *apiKeyRepositoryStub) RevokeAPIKey(context.Context, ids.UUID, ids.UUID) (bool, error) {
	return true, nil
}

type resolverStub struct {
	addresses []netip.Addr
	err       error
}

func (resolver resolverStub) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return resolver.addresses, resolver.err
}

type replayStoreStub struct{ reserved bool }

func (store *replayStoreStub) Reserve(context.Context, string, time.Time) (bool, error) {
	if store.reserved {
		return false, nil
	}
	store.reserved = true
	return true, nil
}

type webhookRepositoryStub struct {
	subscription  WebhookSubscription
	envelope      identity.SecretEnvelope
	overlap       time.Duration
	deliveries    []WebhookDeliveryLog
	deliveryLimit int
}

func (repository *webhookRepositoryStub) CreateWebhook(_ context.Context, subscription WebhookSubscription, envelope identity.SecretEnvelope) (WebhookSubscription, error) {
	repository.subscription, repository.envelope = subscription, envelope
	return subscription, nil
}
func (repository *webhookRepositoryStub) ListWebhooks(context.Context, ids.UUID, int) ([]WebhookSubscription, error) {
	return []WebhookSubscription{repository.subscription}, nil
}
func (repository *webhookRepositoryStub) RotateWebhookSecret(_ context.Context, _, _ ids.UUID, _ int64, envelope identity.SecretEnvelope, overlap time.Duration) (WebhookSubscription, error) {
	repository.envelope, repository.overlap = envelope, overlap
	repository.subscription.Version++
	repository.subscription.SecretVersion++
	return repository.subscription, nil
}
func (repository *webhookRepositoryStub) SetWebhookEnabled(context.Context, ids.UUID, ids.UUID, int64, bool) (WebhookSubscription, bool, error) {
	return repository.subscription, true, nil
}
func (repository *webhookRepositoryStub) RetryWebhookDelivery(context.Context, ids.UUID, ids.UUID) (bool, error) {
	return true, nil
}
func (repository *webhookRepositoryStub) ListWebhookDeliveries(_ context.Context, _ ids.UUID, _ *ids.UUID, _ time.Time, _ ids.UUID, limit int) ([]WebhookDeliveryLog, error) {
	repository.deliveryLimit = limit
	return repository.deliveries, nil
}

func integrationID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
