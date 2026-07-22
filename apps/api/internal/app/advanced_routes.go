package app

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/automation"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/integrations"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type AdvancedHandlers struct {
	logger       *slog.Logger
	tenancy      *tenancy.Service
	automations  *automation.Manager
	integrations *integrations.Manager
}

func NewAdvancedHandlers(
	logger *slog.Logger,
	tenancyService *tenancy.Service,
	automationManager *automation.Manager,
	integrationManager *integrations.Manager,
) *AdvancedHandlers {
	return &AdvancedHandlers{
		logger: logger, tenancy: tenancyService,
		automations: automationManager, integrations: integrationManager,
	}
}

// RegisterAdvancedWorkspaceRoutes mounts routes below the existing protected
// /api/v1/workspaces/{workspaceId} router. Keeping registration explicit makes
// it impossible to accidentally expose management endpoints without session,
// CSRF, membership, and RBAC middleware.
func RegisterAdvancedWorkspaceRoutes(router chi.Router, handlers *AdvancedHandlers) {
	router.Get("/automations", handlers.listAutomationRules)
	router.Post("/automations", handlers.createAutomationRule)
	router.Put("/automations/{automationId}", handlers.updateAutomationRule)
	router.Patch("/automations/{automationId}/enabled", handlers.setAutomationEnabled)
	router.Post("/automations/preview", handlers.previewAutomationRule)
	router.Get("/api-keys", handlers.listAPIKeys)
	router.Post("/api-keys", handlers.createAPIKey)
	router.Delete("/api-keys/{apiKeyId}", handlers.revokeAPIKey)
	router.Get("/webhooks", handlers.listWebhooks)
	router.Post("/webhooks", handlers.createWebhook)
	router.Patch("/webhooks/{webhookId}/enabled", handlers.setWebhookEnabled)
	router.Post("/webhooks/{webhookId}/rotate-secret", handlers.rotateWebhookSecret)
	router.Get("/webhook-deliveries", handlers.listWebhookDeliveries)
	router.Post("/webhook-deliveries/{deliveryId}/retry", handlers.retryWebhookDelivery)
}

func (handlers *AdvancedHandlers) listAutomationRules(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var rules []automation.Rule
	err := handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			rules, err = handlers.automations.List(request.Context(), workspace, workspaceID, 100)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	result := make([]automationRuleResponse, 0, len(rules))
	for _, rule := range rules {
		result = append(result, automationResponse(rule))
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (handlers *AdvancedHandlers) createAutomationRule(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	spec, _, err := httpx.DecodeJSON[automation.RuleSpec](writer, request, 128<<10)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var rule automation.Rule
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			rule, err = handlers.automations.Create(request.Context(), workspace, metadata(request, workspaceID, principal), spec)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	setETag(writer, rule.Version)
	httpx.WriteJSON(writer, http.StatusCreated, automationResponse(rule))
}

func (handlers *AdvancedHandlers) updateAutomationRule(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	ruleID, err := parsePathID(request, "automationId")
	if handlers.problem(writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if handlers.problem(writer, request, err) {
		return
	}
	spec, _, err := httpx.DecodeJSON[automation.RuleSpec](writer, request, 128<<10)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var rule automation.Rule
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			rule, err = handlers.automations.Update(request.Context(), workspace, metadata(request, workspaceID, principal), ruleID, version, spec)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	setETag(writer, rule.Version)
	httpx.WriteJSON(writer, http.StatusOK, automationResponse(rule))
}

func (handlers *AdvancedHandlers) setAutomationEnabled(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	ruleID, err := parsePathID(request, "automationId")
	if handlers.problem(writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if handlers.problem(writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[struct {
		Enabled bool `json:"enabled"`
	}](writer, request, 4096)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var rule automation.Rule
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			rule, err = handlers.automations.SetEnabled(request.Context(), workspace, metadata(request, workspaceID, principal), ruleID, version, body.Enabled)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	setETag(writer, rule.Version)
	httpx.WriteJSON(writer, http.StatusOK, automationResponse(rule))
}

func (handlers *AdvancedHandlers) previewAutomationRule(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[struct {
		Rule       automation.RuleSpec    `json:"rule"`
		Trigger    automation.TriggerType `json:"trigger"`
		EntityType automation.EntityType  `json:"entityType"`
		EntityID   string                 `json:"entityId"`
		Fields     map[string]any         `json:"fields"`
		Tags       []string               `json:"tags,omitempty"`
		OwnerID    string                 `json:"ownerId,omitempty"`
		TeamID     string                 `json:"teamId,omitempty"`
	}](writer, request, 128<<10)
	if handlers.problem(writer, request, err) {
		return
	}
	entityID, err := ids.Parse(body.EntityID)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var preview automation.Preview
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(*tenancy.WorkspaceTx) error {
			var err error
			preview, err = automation.PreviewRule(body.Rule, automation.Event{
				WorkspaceID: workspaceID, Trigger: body.Trigger, EntityType: body.EntityType,
				EntityID: entityID, Fields: body.Fields, Tags: body.Tags, OwnerID: body.OwnerID, TeamID: body.TeamID,
			})
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, preview)
}

func (handlers *AdvancedHandlers) listAPIKeys(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var keys []integrations.APIKey
	err := handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			keys, err = handlers.integrations.ListAPIKeys(request.Context(), workspace, workspaceID, 100)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	result := make([]apiKeyResponse, 0, len(keys))
	for _, key := range keys {
		result = append(result, apiKeyView(key))
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (handlers *AdvancedHandlers) createAPIKey(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[struct {
		Name      string               `json:"name"`
		Scopes    []integrations.Scope `json:"scopes"`
		ExpiresAt *time.Time           `json:"expiresAt,omitempty"`
	}](writer, request, 32<<10)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var generated integrations.GeneratedAPIKey
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			generated, err = handlers.integrations.CreateAPIKey(request.Context(), workspace, metadata(request, workspaceID, principal), body.Name, body.Scopes, body.ExpiresAt)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, map[string]any{"apiKey": apiKeyView(generated.APIKey), "token": generated.Token})
}

func (handlers *AdvancedHandlers) revokeAPIKey(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	keyID, err := parsePathID(request, "apiKeyId")
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			return handlers.integrations.RevokeAPIKey(request.Context(), workspace, metadata(request, workspaceID, principal), keyID)
		})
	if handlers.problem(writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (handlers *AdvancedHandlers) listWebhooks(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var subscriptions []integrations.WebhookSubscription
	err := handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			subscriptions, err = handlers.integrations.ListWebhooks(request.Context(), workspace, workspaceID, 100)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	result := make([]webhookResponse, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		result = append(result, webhookView(subscription))
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (handlers *AdvancedHandlers) createWebhook(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[struct {
		URL            string   `json:"url"`
		EventTypes     []string `json:"eventTypes"`
		Enabled        bool     `json:"enabled"`
		TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
		MaxAttempts    int      `json:"maxAttempts,omitempty"`
	}](writer, request, 32<<10)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var generated integrations.GeneratedWebhook
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			generated, err = handlers.integrations.CreateWebhook(request.Context(), workspace, metadata(request, workspaceID, principal), integrations.WebhookCreate{
				URL: body.URL, EventTypes: body.EventTypes, Enabled: body.Enabled,
				Timeout: time.Duration(body.TimeoutSeconds) * time.Second, MaxAttempts: body.MaxAttempts,
			})
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, map[string]any{
		"webhook": webhookView(generated.Subscription), "signingSecret": generated.SigningSecret,
	})
}

func (handlers *AdvancedHandlers) setWebhookEnabled(writer http.ResponseWriter, request *http.Request) {
	workspaceID, subscriptionID, version, ok := handlers.webhookMutationPath(writer, request)
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[struct {
		Enabled bool `json:"enabled"`
	}](writer, request, 4096)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var subscription integrations.WebhookSubscription
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			subscription, err = handlers.integrations.SetWebhookEnabled(request.Context(), workspace, metadata(request, workspaceID, principal), subscriptionID, version, body.Enabled)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	setETag(writer, subscription.Version)
	httpx.WriteJSON(writer, http.StatusOK, webhookView(subscription))
}

func (handlers *AdvancedHandlers) rotateWebhookSecret(writer http.ResponseWriter, request *http.Request) {
	workspaceID, subscriptionID, version, ok := handlers.webhookMutationPath(writer, request)
	if !ok {
		return
	}
	body, _, err := httpx.DecodeJSON[struct {
		OverlapSeconds int `json:"overlapSeconds"`
	}](writer, request, 4096)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var generated integrations.GeneratedWebhook
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			generated, err = handlers.integrations.RotateWebhook(request.Context(), workspace, metadata(request, workspaceID, principal), subscriptionID, version, time.Duration(body.OverlapSeconds)*time.Second)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	setETag(writer, generated.Subscription.Version)
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{
		"webhook": webhookView(generated.Subscription), "signingSecret": generated.SigningSecret,
	})
}

func (handlers *AdvancedHandlers) retryWebhookDelivery(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	deliveryID, err := parsePathID(request, "deliveryId")
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			return handlers.integrations.RetryWebhookDelivery(request.Context(), workspace, metadata(request, workspaceID, principal), deliveryID)
		})
	if handlers.problem(writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusAccepted)
}

func (handlers *AdvancedHandlers) listWebhookDeliveries(writer http.ResponseWriter, request *http.Request) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return
	}
	subscriptionID, err := parseOptionalID(request.URL.Query().Get("subscriptionId"), "/query/subscriptionId")
	if handlers.problem(writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if handlers.problem(writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var page integrations.WebhookDeliveryPage
	err = handlers.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var err error
			page, err = handlers.integrations.ListWebhookDeliveries(
				request.Context(), workspace, workspaceID, subscriptionID,
				request.URL.Query().Get("cursor"), limit,
			)
			return err
		})
	if handlers.problem(writer, request, err) {
		return
	}
	result := webhookDeliveryPageResponse{
		Items: make([]webhookDeliveryResponse, 0, len(page.Items)), NextCursor: page.NextCursor,
	}
	for _, item := range page.Items {
		result.Items = append(result.Items, webhookDeliveryView(item))
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (handlers *AdvancedHandlers) pathWorkspace(writer http.ResponseWriter, request *http.Request) (ids.UUID, bool) {
	id, err := parsePathID(request, "workspaceId")
	return id, !handlers.problem(writer, request, err)
}

func (handlers *AdvancedHandlers) webhookMutationPath(writer http.ResponseWriter, request *http.Request) (ids.UUID, ids.UUID, int64, bool) {
	workspaceID, ok := handlers.pathWorkspace(writer, request)
	if !ok {
		return ids.UUID{}, ids.UUID{}, 0, false
	}
	subscriptionID, err := parsePathID(request, "webhookId")
	if handlers.problem(writer, request, err) {
		return ids.UUID{}, ids.UUID{}, 0, false
	}
	version, err := parseETag(request)
	if handlers.problem(writer, request, err) {
		return ids.UUID{}, ids.UUID{}, 0, false
	}
	return workspaceID, subscriptionID, version, true
}

func (handlers *AdvancedHandlers) problem(writer http.ResponseWriter, request *http.Request, err error) bool {
	if err == nil {
		return false
	}
	httpx.WriteProblem(writer, request, handlers.logger, err)
	return true
}

type automationRuleResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Trigger          automation.TriggerType `json:"trigger"`
	EntityType       automation.EntityType  `json:"entityType"`
	Conditions       automation.Condition   `json:"conditions"`
	Actions          []automation.Action    `json:"actions"`
	Enabled          bool                   `json:"enabled"`
	RateLimitPerHour int                    `json:"rateLimitPerHour"`
	Version          int64                  `json:"version"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
}

func automationResponse(rule automation.Rule) automationRuleResponse {
	return automationRuleResponse{
		ID: rule.ID.String(), Name: rule.Spec.Name, Trigger: rule.Spec.Trigger, EntityType: rule.Spec.EntityType,
		Conditions: rule.Spec.Conditions, Actions: rule.Spec.Actions, Enabled: rule.Spec.Enabled,
		RateLimitPerHour: rule.Spec.RateLimitPerHour, Version: rule.Version,
		CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
	}
}

type apiKeyResponse struct {
	ID         string               `json:"id"`
	Prefix     string               `json:"prefix"`
	Name       string               `json:"name"`
	Scopes     []integrations.Scope `json:"scopes"`
	CreatedAt  time.Time            `json:"createdAt"`
	LastUsedAt *time.Time           `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time           `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time           `json:"revokedAt,omitempty"`
}

func apiKeyView(key integrations.APIKey) apiKeyResponse {
	return apiKeyResponse{ID: key.ID.String(), Prefix: key.Prefix, Name: key.Name, Scopes: key.Scopes,
		CreatedAt: key.CreatedAt, LastUsedAt: key.LastUsedAt, ExpiresAt: key.ExpiresAt, RevokedAt: key.RevokedAt}
}

type webhookResponse struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	EventTypes     []string  `json:"eventTypes"`
	Enabled        bool      `json:"enabled"`
	Version        int64     `json:"version"`
	SecretVersion  int       `json:"secretVersion"`
	TimeoutSeconds int       `json:"timeoutSeconds"`
	MaxAttempts    int       `json:"maxAttempts"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func webhookView(subscription integrations.WebhookSubscription) webhookResponse {
	return webhookResponse{
		ID: subscription.ID.String(), URL: subscription.URL, EventTypes: subscription.EventTypes,
		Enabled: subscription.Enabled, Version: subscription.Version, SecretVersion: subscription.SecretVersion,
		TimeoutSeconds: int(subscription.Timeout / time.Second), MaxAttempts: subscription.MaxAttempts,
		CreatedAt: subscription.CreatedAt, UpdatedAt: subscription.UpdatedAt,
	}
}

type webhookDeliveryPageResponse struct {
	Items      []webhookDeliveryResponse `json:"items"`
	NextCursor string                    `json:"nextCursor,omitempty"`
}

type webhookDeliveryResponse struct {
	ID               string     `json:"id"`
	SubscriptionID   string     `json:"subscriptionId"`
	EventID          string     `json:"eventId"`
	Status           string     `json:"status"`
	Attempts         int        `json:"attempts"`
	NextAttemptAt    *time.Time `json:"nextAttemptAt,omitempty"`
	ResponseStatus   *int32     `json:"responseStatus,omitempty"`
	ResponseSummary  *string    `json:"responseSummary,omitempty"`
	DeliveredAt      *time.Time `json:"deliveredAt,omitempty"`
	RequestTimestamp *int64     `json:"requestTimestamp,omitempty"`
	SignatureVersion int32      `json:"signatureVersion"`
	LastErrorCode    *string    `json:"lastErrorCode,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func webhookDeliveryView(delivery integrations.WebhookDeliveryLog) webhookDeliveryResponse {
	return webhookDeliveryResponse{
		ID: delivery.ID.String(), SubscriptionID: delivery.SubscriptionID.String(), EventID: delivery.EventID.String(),
		Status: webhookDeliveryStatusView(delivery.Status), Attempts: delivery.Attempts, NextAttemptAt: delivery.NextAttemptAt,
		ResponseStatus: delivery.ResponseStatus, ResponseSummary: delivery.ResponseSummary,
		DeliveredAt: delivery.DeliveredAt, RequestTimestamp: delivery.RequestTimestamp,
		SignatureVersion: delivery.SignatureVersion, LastErrorCode: delivery.LastErrorCode,
		CreatedAt: delivery.CreatedAt, UpdatedAt: delivery.UpdatedAt,
	}
}

func webhookDeliveryStatusView(status string) string {
	switch status {
	case "queued":
		return "pending"
	case "failed":
		return "retrying"
	case "succeeded":
		return "delivered"
	default:
		return status
	}
}
