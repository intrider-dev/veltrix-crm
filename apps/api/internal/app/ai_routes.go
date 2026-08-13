package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/ai"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) registerAIRoutes(router chi.Router) {
	router.Get("/ai/status", application.aiStatus)
	router.Post("/ai/timeline-summary", application.aiTimelineSummary)
	router.Post("/ai/follow-up-draft", application.aiFollowUpDraft)
	router.Post("/ai/next-action", application.aiNextAction)
	router.Post("/ai/duplicate-candidates", application.aiDuplicateCandidates)
}

func (application *Application) aiStatus(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(_ *tenancy.WorkspaceTx) error { return nil },
	)
	if writeError(application, writer, request, err) {
		return
	}
	status := ai.DisabledStatus(configuredAILimits(application.cfg))
	if application.ai != nil {
		status = application.ai.Status()
	}
	writer.Header().Set("Cache-Control", "private, max-age=30")
	httpx.WriteJSON(writer, http.StatusOK, aiStatusResponse(status))
}

func (application *Application) aiTimelineSummary(writer http.ResponseWriter, request *http.Request) {
	handleAIRequest[apigen.AITimelineSummaryRequest, ai.TimelineSummaryRequest](
		application, writer, request, ai.CapabilityTimelineSummary,
		func(ctx context.Context, service *ai.Service, body ai.TimelineSummaryRequest) (ai.Result, error) {
			return service.TimelineSummary(ctx, body)
		},
		aiTimelineRequest,
		func(body ai.TimelineSummaryRequest) bool {
			return body.Consent != nil && body.Consent.ExternalPIITransfer
		},
	)
}

func (application *Application) aiFollowUpDraft(writer http.ResponseWriter, request *http.Request) {
	handleAIRequest[apigen.AIFollowUpDraftRequest, ai.FollowUpDraftRequest](
		application, writer, request, ai.CapabilityFollowUpDraft,
		func(ctx context.Context, service *ai.Service, body ai.FollowUpDraftRequest) (ai.Result, error) {
			return service.FollowUpDraft(ctx, body)
		},
		aiFollowUpRequest,
		func(body ai.FollowUpDraftRequest) bool {
			return body.Consent != nil && body.Consent.ExternalPIITransfer
		},
	)
}

func (application *Application) aiNextAction(writer http.ResponseWriter, request *http.Request) {
	handleAIRequest[apigen.AINextActionRequest, ai.NextActionRequest](
		application, writer, request, ai.CapabilityNextAction,
		func(ctx context.Context, service *ai.Service, body ai.NextActionRequest) (ai.Result, error) {
			return service.NextAction(ctx, body)
		},
		aiNextActionRequest,
		func(body ai.NextActionRequest) bool {
			return body.Consent != nil && body.Consent.ExternalPIITransfer
		},
	)
}

func (application *Application) aiDuplicateCandidates(writer http.ResponseWriter, request *http.Request) {
	handleAIRequest[apigen.AIDuplicateCandidatesRequest, ai.DuplicateCandidatesRequest](
		application, writer, request, ai.CapabilityDuplicateCandidates,
		func(ctx context.Context, service *ai.Service, body ai.DuplicateCandidatesRequest) (ai.Result, error) {
			return service.DuplicateCandidates(ctx, body)
		},
		aiDuplicateRequest,
		func(body ai.DuplicateCandidatesRequest) bool {
			return body.Consent != nil && body.Consent.ExternalPIITransfer
		},
	)
}

func handleAIRequest[Transport any, Domain any](
	application *Application,
	writer http.ResponseWriter,
	request *http.Request,
	capability ai.Capability,
	invoke func(context.Context, *ai.Service, Domain) (ai.Result, error),
	prepare func(Transport, string) Domain,
	externalConsent func(Domain) bool,
) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[Transport](writer, request, application.cfg.AIMaxInputBytes)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	locale := principal.PreferredLocale
	if locale == "" {
		locale = application.cfg.DefaultLocale
	}
	domainRequest := prepare(body, locale)
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(_ *tenancy.WorkspaceTx) error { return nil },
	)
	if writeError(application, writer, request, err) {
		return
	}
	if application.ai == nil {
		httpx.WriteProblem(writer, request, application.logger, errx.ErrUnavailable)
		return
	}
	limitKey := workspaceID.String() + ":" + principal.UserID.String()
	if !application.aiRateLimits.Allow(limitKey, time.Now().UTC()) {
		httpx.WriteProblem(writer, request, application.logger, errx.ErrRateLimited)
		return
	}
	result, err := invoke(request.Context(), application.ai, domainRequest)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, mapAIError(err))
		return
	}
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return ai.RecordAudit(
				request.Context(), workspace, metadata(request, workspaceID, principal), capability,
				application.ai.ProviderInfo(), externalConsent(domainRequest), len(result.Content),
			)
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(writer, http.StatusOK, apigen.AIResult{
		Capability: apigen.AICapability(result.Capability), Content: result.Content,
		Advisory: apigen.AIResultAdvisoryTrue,
	})
}

func aiStatusResponse(status ai.Status) apigen.AIStatus {
	capabilities := make([]apigen.AICapability, 0, len(status.Capabilities))
	for _, capability := range status.Capabilities {
		capabilities = append(capabilities, apigen.AICapability(capability))
	}
	var providerClass *apigen.AIStatusProviderClass
	if status.ProviderClass != nil {
		value := apigen.AIStatusProviderClass(*status.ProviderClass)
		providerClass = &value
	}
	return apigen.AIStatus{
		Enabled: status.Enabled, Provider: status.Provider, ProviderClass: providerClass,
		RequiresExternalPiiConsent: status.RequiresExternalPIIConsent, Capabilities: capabilities,
		Limits: apigen.AILimits{
			MaxInputBytes: int(status.Limits.MaxInputBytes), MaxOutputBytes: int(status.Limits.MaxOutputBytes),
			MaxContextItems:        status.Limits.MaxContextItems,
			MaxDuplicateCandidates: status.Limits.MaxDuplicateCandidates,
		},
	}
}

func aiTimelineRequest(body apigen.AITimelineSummaryRequest, fallbackLocale string) ai.TimelineSummaryRequest {
	entityType := ""
	if body.EntityType != nil {
		entityType = *body.EntityType
	}
	return ai.TimelineSummaryRequest{
		Locale: localeOrFallback(body.Locale, fallbackLocale), EntityType: entityType,
		EntityID: body.EntityId, Items: aiContext(body.Items), Consent: aiConsent(body.Consent),
	}
}

func aiFollowUpRequest(body apigen.AIFollowUpDraftRequest, fallbackLocale string) ai.FollowUpDraftRequest {
	return ai.FollowUpDraftRequest{
		Locale: localeOrFallback(body.Locale, fallbackLocale), Channel: string(body.Channel),
		Tone: optionalString(body.Tone), Recipient: optionalString(body.Recipient), Objective: body.Objective,
		Context: aiContext(body.Context), Consent: aiConsent(body.Consent),
	}
}

func aiNextActionRequest(body apigen.AINextActionRequest, fallbackLocale string) ai.NextActionRequest {
	return ai.NextActionRequest{
		Locale: localeOrFallback(body.Locale, fallbackLocale), EntityType: string(body.EntityType),
		EntityID: body.EntityId, Goal: optionalString(body.Goal), Context: aiContext(body.Context),
		Consent: aiConsent(body.Consent),
	}
}

func aiDuplicateRequest(body apigen.AIDuplicateCandidatesRequest, fallbackLocale string) ai.DuplicateCandidatesRequest {
	candidates := make([]ai.DuplicateCandidate, 0, len(body.Candidates))
	for _, candidate := range body.Candidates {
		candidates = append(candidates, ai.DuplicateCandidate{ID: candidate.Id, Fields: candidate.Fields})
	}
	return ai.DuplicateCandidatesRequest{
		Locale: localeOrFallback(body.Locale, fallbackLocale), EntityType: string(body.EntityType),
		Subject:    ai.DuplicateRecord{ID: body.Subject.Id, Fields: body.Subject.Fields},
		Candidates: candidates, Consent: aiConsent(body.Consent),
	}
}

func aiContext(items []apigen.AIContextItem) []ai.ContextItem {
	result := make([]ai.ContextItem, 0, len(items))
	for _, item := range items {
		result = append(result, ai.ContextItem{
			Kind: item.Kind, OccurredAt: item.OccurredAt, Subject: item.Subject, Detail: item.Detail,
		})
	}
	return result
}

func aiConsent(consent *apigen.AIConsent) *ai.Consent {
	if consent == nil {
		return nil
	}
	return &ai.Consent{ExternalPIITransfer: consent.ExternalPiiTransfer}
}

func localeOrFallback(locale *apigen.Locale, fallback string) string {
	if locale == nil {
		return fallback
	}
	return string(*locale)
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func mapAIError(err error) error {
	switch {
	case errors.Is(err, ai.ErrConsentRequired):
		return errx.ErrSecurityRejected
	case errors.Is(err, ai.ErrConcurrencyLimited):
		return errx.ErrRateLimited
	case errors.Is(err, ai.ErrProviderUnavailable), errors.Is(err, ai.ErrOutputTooLarge):
		return errx.ErrUnavailable
	default:
		return err
	}
}
