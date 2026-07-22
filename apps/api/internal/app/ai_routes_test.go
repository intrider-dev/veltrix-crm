package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/ai"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

func TestAIRouteContract(t *testing.T) {
	router := chi.NewRouter()
	(&Application{}).registerAIRoutes(router)
	want := map[string]bool{
		http.MethodGet + " /ai/status":                false,
		http.MethodPost + " /ai/timeline-summary":     false,
		http.MethodPost + " /ai/follow-up-draft":      false,
		http.MethodPost + " /ai/next-action":          false,
		http.MethodPost + " /ai/duplicate-candidates": false,
	}
	if err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if _, ok := want[key]; ok {
			want[key] = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for route, found := range want {
		if !found {
			t.Errorf("route not registered: %s", route)
		}
	}
}

func TestAIErrorMappingUsesStablePublicErrors(t *testing.T) {
	tests := []struct {
		input error
		want  error
	}{
		{ai.ErrConsentRequired, errx.ErrSecurityRejected},
		{ai.ErrConcurrencyLimited, errx.ErrRateLimited},
		{ai.ErrProviderUnavailable, errx.ErrUnavailable},
		{ai.ErrOutputTooLarge, errx.ErrUnavailable},
	}
	for _, test := range tests {
		if got := mapAIError(test.input); !errors.Is(got, test.want) {
			t.Errorf("mapAIError(%v) = %v, want %v", test.input, got, test.want)
		}
	}
}

func TestAIDisabledDoesNotConstructProvider(t *testing.T) {
	service, err := buildAIService(config.Config{AIProvider: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if service != nil {
		t.Fatal("disabled AI constructed a provider")
	}
}

func TestGeneratedAIContractMapsToBoundedDomainRequest(t *testing.T) {
	locale := apigen.Locale("ru")
	entityType := "contact"
	entityID := uuid.New()
	detail := "PII remains request-scoped"
	request := aiTimelineRequest(apigen.AITimelineSummaryRequest{
		Locale: &locale, EntityType: &entityType, EntityId: &entityID,
		Items:   []apigen.AIContextItem{{Kind: "note", Subject: "Follow up", Detail: &detail}},
		Consent: &apigen.AIConsent{ExternalPiiTransfer: true},
	}, "en")
	if request.Locale != "ru" || request.EntityType != "contact" || request.EntityID == nil || *request.EntityID != entityID {
		t.Fatalf("unexpected mapped request: %+v", request)
	}
	if len(request.Items) != 1 || request.Items[0].Detail == nil || *request.Items[0].Detail != detail {
		t.Fatalf("context was not mapped: %+v", request.Items)
	}
	if request.Consent == nil || !request.Consent.ExternalPIITransfer {
		t.Fatal("explicit consent was not mapped")
	}
}

func TestAIStatusMapsToGeneratedResponseWithoutModelDetails(t *testing.T) {
	provider := "openai"
	providerClass := ai.ProviderClassExternal
	response := aiStatusResponse(ai.Status{
		Enabled: true, Provider: &provider, ProviderClass: &providerClass,
		RequiresExternalPIIConsent: true, Capabilities: []ai.Capability{ai.CapabilityTimelineSummary},
		Limits: ai.Limits{MaxInputBytes: 1024, MaxOutputBytes: 256, MaxContextItems: 5, MaxDuplicateCandidates: 3},
	})
	if !response.Enabled || response.Provider == nil || *response.Provider != "openai" ||
		response.ProviderClass == nil || *response.ProviderClass != apigen.External || !response.RequiresExternalPiiConsent {
		t.Fatalf("unexpected status response: %+v", response)
	}
	if len(response.Capabilities) != 1 || response.Capabilities[0] != apigen.TimelineSummary {
		t.Fatalf("unexpected capabilities: %+v", response.Capabilities)
	}
}
