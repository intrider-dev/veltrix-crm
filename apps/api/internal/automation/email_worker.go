package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

const (
	maxAutomationEmailJobPayload = 32 << 10
	maxAutomationEmailParams     = 64
	maxRenderedAutomationEmail   = 64 << 10
)

type emailJobPayload struct {
	TargetType     EntityType     `json:"targetType"`
	TargetID       string         `json:"targetId"`
	RecipientField string         `json:"recipientField"`
	TemplateKey    string         `json:"templateKey"`
	TemplateParams map[string]any `json:"templateParams"`
}

type ResolvedAutomationEmail struct {
	Recipient string
	Subject   string
	Body      string
}

type AutomationEmailResolver interface {
	Resolve(context.Context, worker.Job, emailJobPayload) (ResolvedAutomationEmail, error)
}

func NewEmailWorkerHandler(resolver AutomationEmailResolver, sender notifications.EmailSender) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if resolver == nil || sender == nil {
			return executionFailure{code: "automation_email_not_configured", err: errors.New("automation email is not configured")}
		}
		payload, err := decodeEmailJobPayload(job)
		if err != nil {
			return err
		}
		message, err := resolver.Resolve(ctx, job, payload)
		if err != nil {
			return executionFailure{code: "automation_email_resolve_failed", err: err}
		}
		// The stable job UUID is used as Message-ID on every retry. SMTP is an
		// at-least-once boundary; a stable identifier lets downstream transports
		// deduplicate the small crash window after acceptance but before job ACK.
		if err := sender.Send(ctx, notifications.EmailMessage{
			ID: job.ID.String(), Recipient: message.Recipient,
			Subject: message.Subject, TextBody: message.Body,
		}); err != nil {
			return executionFailure{code: "automation_email_send_failed", err: err}
		}
		return nil
	}
}

func decodeEmailJobPayload(job worker.Job) (emailJobPayload, error) {
	if job.SchemaVersion != 1 || len(job.Payload) == 0 || len(job.Payload) > maxAutomationEmailJobPayload {
		return emailJobPayload{}, executionFailure{code: "automation_email_payload_invalid", err: errors.New("automation email payload is invalid")}
	}
	decoder := json.NewDecoder(bytes.NewReader(job.Payload))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var payload emailJobPayload
	if err := decoder.Decode(&payload); err != nil {
		return emailJobPayload{}, executionFailure{code: "automation_email_payload_invalid", err: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return emailJobPayload{}, executionFailure{code: "automation_email_payload_invalid", err: errors.New("automation email payload has trailing data")}
	}
	if !emailTargetType(payload.TargetType) || !emailRecipientField(payload.RecipientField) ||
		!keyPattern.MatchString(payload.TemplateKey) || len(payload.TemplateKey) > 152 ||
		len(payload.TemplateParams) > maxAutomationEmailParams {
		return emailJobPayload{}, executionFailure{code: "automation_email_payload_invalid", err: errors.New("automation email payload fields are invalid")}
	}
	if _, err := ids.Parse(payload.TargetID); err != nil {
		return emailJobPayload{}, executionFailure{code: "automation_email_payload_invalid", err: errors.New("automation email target ID is invalid")}
	}
	for name, value := range payload.TemplateParams {
		if !fieldNamePattern.MatchString(name) || !validTemplateValue(value) {
			return emailJobPayload{}, executionFailure{code: "automation_email_payload_invalid", err: errors.New("automation email template params are invalid")}
		}
	}
	return payload, nil
}

func emailTargetType(entityType EntityType) bool {
	switch entityType {
	case EntityContact, EntityCompany, EntityLead, EntityDeal, EntityActivity:
		return true
	default:
		return false
	}
}

func emailRecipientField(field string) bool {
	return field == "email" || field == "owner_email"
}

func validTemplateValue(value any) bool {
	switch typed := value.(type) {
	case string:
		return len(typed) <= 4096
	case bool, json.Number:
		return true
	default:
		return false
	}
}

type PostgresAutomationEmailResolver struct {
	pool *pgxpool.Pool
}

func NewPostgresAutomationEmailResolver(pool *pgxpool.Pool) *PostgresAutomationEmailResolver {
	return &PostgresAutomationEmailResolver{pool: pool}
}

func (resolver *PostgresAutomationEmailResolver) Resolve(
	ctx context.Context, job worker.Job, payload emailJobPayload,
) (ResolvedAutomationEmail, error) {
	if resolver == nil || resolver.pool == nil {
		return ResolvedAutomationEmail{}, errors.New("automation email database pool is required")
	}
	targetID, _ := ids.Parse(payload.TargetID)
	tx, err := resolver.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ResolvedAutomationEmail{}, fmt.Errorf("begin automation email resolution: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: job.WorkspaceID.String(), RequestID: "automation-email:" + job.ID.String(),
	}); err != nil {
		return ResolvedAutomationEmail{}, fmt.Errorf("set automation email tenant context: %w", err)
	}
	recipient, err := queries.ResolveAutomationEmailRecipient(ctx, dbgen.ResolveAutomationEmailRecipientParams{
		WorkspaceID: job.WorkspaceID.PG(), TargetType: string(payload.TargetType),
		TargetID: targetID.PG(), RecipientField: payload.RecipientField,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ResolvedAutomationEmail{}, errors.New("automation email recipient is unavailable or inactive")
	}
	if err != nil {
		return ResolvedAutomationEmail{}, fmt.Errorf("resolve automation email recipient: %w", err)
	}
	subject, err := queries.ResolvePublishedContent(ctx, dbgen.ResolvePublishedContentParams{
		WorkspaceID: job.WorkspaceID.PG(), RequestedLocale: recipient.RecipientLocale,
		FallbackLocale: recipient.DefaultLocale, Namespace: "email", ResourceKey: payload.TemplateKey + ".subject",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ResolvedAutomationEmail{}, errors.New("automation email subject template is not published")
	}
	if err != nil {
		return ResolvedAutomationEmail{}, fmt.Errorf("resolve automation email subject: %w", err)
	}
	body, err := queries.ResolvePublishedContent(ctx, dbgen.ResolvePublishedContentParams{
		WorkspaceID: job.WorkspaceID.PG(), RequestedLocale: recipient.RecipientLocale,
		FallbackLocale: recipient.DefaultLocale, Namespace: "email", ResourceKey: payload.TemplateKey + ".body",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ResolvedAutomationEmail{}, errors.New("automation email body template is not published")
	}
	if err != nil {
		return ResolvedAutomationEmail{}, fmt.Errorf("resolve automation email body: %w", err)
	}
	params := make(map[string]any, len(payload.TemplateParams)+1)
	for key, value := range payload.TemplateParams {
		params[key] = value
	}
	params["workspaceName"] = recipient.WorkspaceName
	renderedSubject, err := renderAutomationTemplate(subject.ResolvedText, params)
	if err != nil || strings.ContainsAny(renderedSubject, "\r\n") {
		return ResolvedAutomationEmail{}, errors.New("automation email subject template is invalid")
	}
	renderedBody, err := renderAutomationTemplate(body.ResolvedText, params)
	if err != nil {
		return ResolvedAutomationEmail{}, errors.New("automation email body template is invalid")
	}
	if len(renderedSubject)+len(renderedBody) > maxRenderedAutomationEmail {
		return ResolvedAutomationEmail{}, errors.New("rendered automation email exceeds size limit")
	}
	if err := tx.Commit(ctx); err != nil {
		return ResolvedAutomationEmail{}, fmt.Errorf("commit automation email resolution: %w", err)
	}
	return ResolvedAutomationEmail{
		Recipient: recipient.Recipient, Subject: renderedSubject, Body: renderedBody,
	}, nil
}

func renderAutomationTemplate(template string, params map[string]any) (string, error) {
	placeholders, err := localization.ExtractPlaceholders(template)
	if err != nil {
		return "", err
	}
	rendered := template
	for _, placeholder := range placeholders {
		value, exists := params[placeholder]
		if !exists || !validTemplateValueForRender(value) {
			return "", fmt.Errorf("template parameter %q is missing or invalid", placeholder)
		}
		rendered = strings.ReplaceAll(rendered, "{"+placeholder+"}", fmt.Sprint(value))
	}
	remaining, err := localization.ExtractPlaceholders(rendered)
	if err != nil || len(remaining) > 0 {
		return "", errors.New("template has unresolved placeholders")
	}
	return rendered, nil
}

func validTemplateValueForRender(value any) bool {
	switch typed := value.(type) {
	case string:
		return len(typed) <= 4096
	case bool, json.Number, float64, float32, int, int32, int64, uint, uint32, uint64:
		return true
	default:
		return false
	}
}

var _ AutomationEmailResolver = (*PostgresAutomationEmailResolver)(nil)
