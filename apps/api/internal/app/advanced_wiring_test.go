package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func TestAPIKeyScopeMappingIsExplicit(t *testing.T) {
	workspaceID := "01900000-0000-7000-8000-000000000001"
	tests := []struct {
		method string
		path   string
		want   integrations.Scope
		ok     bool
	}{
		{http.MethodGet, "/api/v1/workspaces/" + workspaceID + "/contacts", integrations.ScopeContactsRead, true},
		{http.MethodPost, "/api/v1/workspaces/" + workspaceID + "/contacts", integrations.ScopeContactsWrite, true},
		{http.MethodPatch, "/api/v1/workspaces/" + workspaceID + "/companies/1", integrations.ScopeCompaniesWrite, true},
		{http.MethodGet, "/api/v1/workspaces/" + workspaceID + "/pipelines", integrations.ScopeDealsRead, true},
		{http.MethodPut, "/api/v1/workspaces/" + workspaceID + "/activities/1", integrations.ScopeActivitiesWrite, true},
		{http.MethodGet, "/api/v1/workspaces/" + workspaceID + "/reports/period", integrations.ScopeReportsRead, true},
		{http.MethodPost, "/api/v1/workspaces/" + workspaceID + "/webhooks", integrations.ScopeWebhooksWrite, true},
		{http.MethodPost, "/api/v1/workspaces/" + workspaceID + "/api-keys", "", false},
		{http.MethodPost, "/api/v1/workspaces/" + workspaceID + "/automations", "", false},
		{http.MethodPost, "/api/v1/auth/logout", "", false},
		{http.MethodTrace, "/api/v1/workspaces/" + workspaceID + "/contacts", integrations.ScopeContactsWrite, false},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			got, ok := apiKeyScopeForRequest(test.method, test.path)
			if got != test.want || ok != test.ok {
				t.Fatalf("scope = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestValidatedAPIKeyBypassesCookieCSRF(t *testing.T) {
	workspaceID := ids.MustParse("01900000-0000-7000-8000-000000000001")
	actorID := ids.MustParse("01900000-0000-7000-8000-000000000002")
	repository := &wiringAPIKeyRepository{}
	service := integrations.NewAPIKeyService(repository)
	generated, err := service.Create(context.Background(), integrations.APIKeyCreate{
		WorkspaceID: workspaceID, CreatedBy: actorID, Name: "test",
		Scopes: []integrations.Scope{integrations.ScopeContactsWrite}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application := &Application{
		logger:     logger,
		apiKeyAuth: integrations.NewAPIKeyHTTPAuthenticator(service, logger),
	}
	called := false
	router := chi.NewRouter()
	router.Use(application.authenticateCredential)
	router.Use(application.csrf)
	router.Post("/api/v1/workspaces/{workspaceId}/contacts", func(writer http.ResponseWriter, _ *http.Request) {
		called = true
		writer.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+workspaceID.String()+"/contacts", nil)
	request.Header.Set("Authorization", "Bearer "+generated.Token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("status = %d, called = %v, body = %s", response.Code, called, response.Body.String())
	}

	called = false
	request = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+workspaceID.String()+"/contacts", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code == http.StatusNoContent || called {
		t.Fatalf("invalid key reached handler: status = %d", response.Code)
	}
}

func TestBuildWorkerHandlersMergesBoundedRegistries(t *testing.T) {
	handlers, err := BuildWorkerHandlers(config.Config{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{
		"automation.dispatch", "automation.execute", "automation.email.send",
		"notification.dispatch", "search.sync", "webhook.dispatch", "webhook.deliver",
		"customers.import.contacts", "activity.reminder", "notification.email",
	} {
		if handlers[kind] == nil {
			t.Errorf("handler %q is not registered", kind)
		}
	}
	if err := mergeWorkerHandlers(handlers, map[string]worker.Handler{
		"automation.execute": func(context.Context, worker.Dependencies, worker.Job) error { return nil },
	}); err == nil {
		t.Fatal("duplicate worker kind was accepted")
	}
	mailHandlers, err := BuildWorkerHandlers(config.Config{
		IdentityKeyID:     "mailbox-test-key",
		IdentityKeyBase64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mailHandlers["mailbox.outgoing.deliver"] == nil {
		t.Fatal("durable mailbox delivery handler is not registered")
	}
}

type wiringAPIKeyRepository struct {
	key        integrations.APIKey
	credential integrations.APIKeyCredential
}

func (repository *wiringAPIKeyRepository) CreateAPIKey(
	_ context.Context, key integrations.APIKey, hash [32]byte,
) (integrations.APIKey, error) {
	repository.key = key
	repository.credential = integrations.APIKeyCredential{
		ID: key.ID, WorkspaceID: key.WorkspaceID, Prefix: key.Prefix, SecretHash: hash,
		Scopes: key.Scopes, CreatedBy: key.CreatedBy, ExpiresAt: key.ExpiresAt,
	}
	return key, nil
}

func (repository *wiringAPIKeyRepository) ListAPIKeys(context.Context, ids.UUID, int) ([]integrations.APIKey, error) {
	return []integrations.APIKey{repository.key}, nil
}

func (repository *wiringAPIKeyRepository) LookupAPIKey(
	_ context.Context, workspaceID ids.UUID, prefix string,
) (integrations.APIKeyCredential, bool, error) {
	return repository.credential, workspaceID == repository.credential.WorkspaceID && prefix == repository.credential.Prefix, nil
}

func (*wiringAPIKeyRepository) TouchAPIKey(context.Context, ids.UUID, ids.UUID) error { return nil }

func (*wiringAPIKeyRepository) RevokeAPIKey(context.Context, ids.UUID, ids.UUID) (bool, error) {
	return true, nil
}
