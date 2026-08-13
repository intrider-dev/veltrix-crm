package mailbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const DeliveryJobKind = "mailbox.outgoing.deliver"

type deliveryJobPayload struct {
	OutgoingID string `json:"outgoingId"`
	ActorID    string `json:"actorUserId"`
}

type deliveryPlan struct {
	workspaceID ids.UUID
	actorID     ids.UUID
	outgoingID  ids.UUID
	account     dbgen.MailboxAccount
	outgoing    dbgen.MailboxOutgoingMessage
	password    []byte
}

func (plan *deliveryPlan) close() {
	if plan == nil {
		return
	}
	clear(plan.password)
	plan.password = nil
}

type deliveryStore interface {
	prepare(context.Context, worker.Job, deliveryJobPayload) (*deliveryPlan, bool, error)
	markSent(context.Context, worker.Job, *deliveryPlan) error
	markFailed(context.Context, worker.Job, *deliveryPlan, string, bool) error
}

type deliverySender func(context.Context, *deliveryPlan) error

type postgresDeliveryStore struct {
	tenancy *tenancy.Service
	service *Service
}

func NewDeliveryJobHandler(tenancyService *tenancy.Service, service *Service) worker.Handler {
	store := &postgresDeliveryStore{tenancy: tenancyService, service: service}
	return newDeliveryJobHandler(store, service.sendDelivery)
}

func newDeliveryJobHandler(store deliveryStore, send deliverySender) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		payload, err := decodeDeliveryJob(job)
		if err != nil {
			return deliveryJobError{code: "mail_delivery_payload_invalid", err: err}
		}
		plan, skip, err := store.prepare(ctx, job, payload)
		if err != nil {
			return deliveryJobError{code: "mail_delivery_prepare_failed", err: err}
		}
		if skip {
			return nil
		}
		defer plan.close()
		if err := send(ctx, plan); err != nil {
			code, retryable, ambiguous := deliveryFailureDisposition(err)
			terminal := ambiguous || !retryable || job.Attempts >= job.MaxAttempts
			if markErr := store.markFailed(ctx, job, plan, code, terminal); markErr != nil {
				return deliveryJobError{code: "mail_delivery_state_failed", err: markErr}
			}
			if terminal {
				return nil
			}
			return deliveryJobError{code: code, err: err}
		}
		if err := store.markSent(ctx, job, plan); err != nil {
			// The durable state remains `sending`. A reclaimed job treats that
			// state as an uncertain prior submission and never sends again.
			return deliveryJobError{code: "mail_delivery_commit_failed", err: err}
		}
		return nil
	}
}

func decodeDeliveryJob(job worker.Job) (deliveryJobPayload, error) {
	if job.SchemaVersion != 1 || job.Kind != DeliveryJobKind || len(job.Payload) == 0 {
		return deliveryJobPayload{}, errors.New("mail delivery job envelope is invalid")
	}
	var payload deliveryJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return deliveryJobPayload{}, errors.New("mail delivery job payload is invalid")
	}
	if _, err := ids.Parse(payload.OutgoingID); err != nil {
		return deliveryJobPayload{}, errors.New("mail delivery outgoing ID is invalid")
	}
	if _, err := ids.Parse(payload.ActorID); err != nil {
		return deliveryJobPayload{}, errors.New("mail delivery actor ID is invalid")
	}
	return payload, nil
}

func (store *postgresDeliveryStore) prepare(
	ctx context.Context, job worker.Job, payload deliveryJobPayload,
) (*deliveryPlan, bool, error) {
	if store == nil || store.tenancy == nil || store.service == nil {
		return nil, false, errors.New("mail delivery store is unavailable")
	}
	actorID, _ := ids.Parse(payload.ActorID)
	outgoingID, _ := ids.Parse(payload.OutgoingID)
	principal := identity.Principal{UserID: actorID}
	var plan *deliveryPlan
	skip := false
	err := store.tenancy.WithWorkspace(ctx, principal, job.WorkspaceID, deliveryRequestID(job), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			outgoing, err := workspace.Queries.GetMailboxOutgoing(ctx, dbgen.GetMailboxOutgoingParams{
				WorkspaceID: job.WorkspaceID.PG(), UserID: actorID.PG(), ID: outgoingID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("get outgoing mailbox message: %w", err)
			}
			switch outgoing.State {
			case "sent", "dead":
				skip = true
				return nil
			case "sending":
				code := "mail_delivery_uncertain"
				if err := store.service.markDeliveryFailed(ctx, workspace, job.WorkspaceID, actorID, outgoingID, code, true); err != nil {
					return err
				}
				skip = true
				return nil
			case "queued", "failed":
			default:
				return errors.New("outgoing mailbox state is invalid")
			}
			var recipients RecipientSet
			if err := json.Unmarshal(outgoing.Recipients, &recipients); err != nil {
				return ErrMalformedMessage
			}
			account, password, err := store.service.secretAccount(
				ctx, workspace, job.WorkspaceID, actorID, mustUUID(outgoing.AccountID),
			)
			if err != nil {
				return err
			}
			claimed, err := workspace.Queries.ClaimMailboxOutgoingDelivery(ctx, dbgen.ClaimMailboxOutgoingDeliveryParams{
				WorkspaceID: job.WorkspaceID.PG(), UserID: actorID.PG(), ID: outgoingID.PG(),
			})
			if err != nil {
				clear(password)
				return fmt.Errorf("claim outgoing mailbox delivery: %w", err)
			}
			plan = &deliveryPlan{
				workspaceID: job.WorkspaceID, actorID: actorID, outgoingID: outgoingID,
				account: account, outgoing: claimed, password: password,
			}
			return nil
		})
	if err != nil {
		plan.close()
		return nil, false, err
	}
	return plan, skip, nil
}

func (store *postgresDeliveryStore) markSent(
	ctx context.Context, job worker.Job, plan *deliveryPlan,
) error {
	principal := identity.Principal{UserID: plan.actorID}
	return store.tenancy.WithWorkspace(ctx, principal, plan.workspaceID, deliveryRequestID(job), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			rows, err := workspace.Queries.MarkMailboxOutgoingSent(ctx, dbgen.MarkMailboxOutgoingSentParams{
				WorkspaceID: plan.workspaceID.PG(), UserID: plan.actorID.PG(), ID: plan.outgoingID.PG(),
			})
			if err != nil {
				return fmt.Errorf("mark mailbox message sent: %w", err)
			}
			if rows != 1 {
				return errors.New("outgoing mailbox delivery state changed")
			}
			return nil
		})
}

func (store *postgresDeliveryStore) markFailed(
	ctx context.Context, job worker.Job, plan *deliveryPlan, code string, terminal bool,
) error {
	principal := identity.Principal{UserID: plan.actorID}
	return store.tenancy.WithWorkspace(ctx, principal, plan.workspaceID, deliveryRequestID(job), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			return store.service.markDeliveryFailed(
				ctx, workspace, plan.workspaceID, plan.actorID, plan.outgoingID, code, terminal,
			)
		})
}

func (service *Service) markDeliveryFailed(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, actorID, outgoingID ids.UUID,
	code string,
	terminal bool,
) error {
	if code == "" || len(code) > 120 {
		code = "mail_delivery_failed"
	}
	rows, err := workspace.Queries.MarkMailboxOutgoingFailed(ctx, dbgen.MarkMailboxOutgoingFailedParams{
		Terminal: terminal, ErrorCode: &code, WorkspaceID: workspaceID.PG(), UserID: actorID.PG(), ID: outgoingID.PG(),
	})
	if err != nil {
		return fmt.Errorf("record mailbox delivery failure: %w", err)
	}
	if rows != 1 {
		return errors.New("outgoing mailbox delivery state changed")
	}
	return nil
}

func (service *Service) sendDelivery(ctx context.Context, plan *deliveryPlan) error {
	release, err := service.acquireConnection(ctx)
	if err != nil {
		return err
	}
	defer release()
	var recipients RecipientSet
	if err := json.Unmarshal(plan.outgoing.Recipients, &recipients); err != nil {
		return ErrMalformedMessage
	}
	return service.smtp.Send(
		ctx, smtpConfig(plan.account), string(plan.password), plan.account.Email,
		plan.outgoing.InternetMessageID,
		SendInput{Recipients: recipients, Subject: plan.outgoing.Subject, PlainText: plan.outgoing.PlainText},
	)
}

func deliveryRequestID(job worker.Job) string {
	return "mail-delivery-" + job.ID.String()
}

type deliveryJobError struct {
	code string
	err  error
}

func (failure deliveryJobError) Error() string       { return failure.err.Error() }
func (failure deliveryJobError) Unwrap() error       { return failure.err }
func (failure deliveryJobError) FailureCode() string { return failure.code }

func deliveryFailureDisposition(err error) (code string, retryable, ambiguous bool) {
	switch {
	case errors.Is(err, ErrSMTPSubmissionUncertain), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "mail_delivery_uncertain", false, true
	case errors.Is(err, ErrEndpointUnavailable):
		return "mail_endpoint_unavailable", true, false
	case errors.Is(err, ErrEndpointRejected):
		return "mail_endpoint_rejected", false, false
	case errors.Is(err, ErrSMTPAuthentication):
		return "mail_authentication_failed", false, false
	case errors.Is(err, ErrSMTPEnvelopeRejected):
		return "mail_recipient_rejected", false, false
	case errors.Is(err, ErrMessageTooLarge):
		return "mail_message_too_large", false, false
	case errors.Is(err, ErrMalformedMessage):
		return "mail_message_invalid", false, false
	default:
		return "mail_delivery_failed", false, true
	}
}
