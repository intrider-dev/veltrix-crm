package calls

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Call struct {
	ID               ids.UUID   `json:"id"`
	ConversationID   ids.UUID   `json:"conversationId"`
	Kind             string     `json:"kind"`
	State            string     `json:"state"`
	ParticipantState string     `json:"participantState"`
	CreatedBy        ids.UUID   `json:"createdBy"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	EndedAt          *time.Time `json:"endedAt,omitempty"`
	Version          int64      `json:"version"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type Service struct{ provider Provider }

const (
	ringingCallLifetime = 2 * time.Minute
	activeCallLifetime  = 4 * time.Hour
)

func NewService(provider Provider) *Service {
	if provider == nil {
		provider = DisabledProvider{}
	}
	return &Service{provider: provider}
}

func (service *Service) Enabled() bool { return service.provider.Enabled() }

func (service *Service) Create(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	conversationID ids.UUID, kind string,
) (Call, error) {
	if !service.Enabled() {
		return Call{}, errx.ErrUnavailable
	}
	if kind != "audio" && kind != "video" {
		return Call{}, validationError("/kind", "validation.enum")
	}
	member, err := workspace.Queries.ConversationMemberExists(ctx, dbgen.ConversationMemberExistsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ConversationID: conversationID.PG(), UserID: metadata.ActorID.PG(),
	})
	if err != nil {
		return Call{}, fmt.Errorf("check call conversation membership: %w", err)
	}
	if !member {
		return Call{}, errx.ErrNotFound
	}
	now := time.Now().UTC()
	expired, err := workspace.Queries.ExpireStaleConversationCalls(ctx,
		dbgen.ExpireStaleConversationCallsParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ConversationID: conversationID.PG(),
			RingingCutoff: pgtype.Timestamptz{Time: now.Add(-ringingCallLifetime), Valid: true},
			ActiveCutoff:  pgtype.Timestamptz{Time: now.Add(-activeCallLifetime), Valid: true},
		})
	if err != nil {
		return Call{}, fmt.Errorf("expire stale conversation calls: %w", err)
	}
	for _, rawExpiredID := range expired {
		expiredID, valid := ids.FromPG(rawExpiredID)
		if !valid {
			return Call{}, errors.New("invalid expired call identifier")
		}
		recipients, recipientErr := service.participants(ctx, workspace, metadata.WorkspaceID, expiredID)
		if recipientErr != nil {
			return Call{}, recipientErr
		}
		if participantErr := workspace.Queries.EndCallParticipants(ctx, dbgen.EndCallParticipantsParams{
			WorkspaceID: metadata.WorkspaceID.PG(), CallID: expiredID.PG(),
		}); participantErr != nil {
			return Call{}, fmt.Errorf("end expired call participants: %w", participantErr)
		}
		if eventErr := events.RecordTargeted(ctx, workspace.Queries, metadata, events.Mutation{
			Action: "call.expired", EventType: "call.ended", AggregateType: "call", AggregateID: expiredID,
			Summary: map[string]any{"reason": "timeout"}, Payload: map[string]any{
				"callId": expiredID.String(), "conversationId": conversationID.String(), "reason": "timeout",
			},
		}, recipients); eventErr != nil {
			return Call{}, eventErr
		}
	}
	callID, err := ids.NewV7()
	if err != nil {
		return Call{}, err
	}
	row, err := workspace.Queries.CreateCall(ctx, dbgen.CreateCallParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: callID.PG(), ConversationID: conversationID.PG(),
		RoomName: "veltrix-call-" + callID.String(), CallKind: kind, CreatedBy: metadata.ActorID.PG(),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Call{}, errx.ErrConflict
	}
	if err != nil {
		return Call{}, fmt.Errorf("create call: %w", err)
	}
	added, err := workspace.Queries.AddConversationCallParticipants(ctx, dbgen.AddConversationCallParticipantsParams{
		CallID: callID.PG(), WorkspaceID: metadata.WorkspaceID.PG(), ConversationID: conversationID.PG(),
	})
	if err != nil {
		return Call{}, fmt.Errorf("create call participants: %w", err)
	}
	if added < 2 {
		return Call{}, errx.ErrConflict
	}
	recipients, err := service.participants(ctx, workspace, metadata.WorkspaceID, callID)
	if err != nil {
		return Call{}, err
	}
	payload := map[string]any{
		"callId": callID.String(), "conversationId": conversationID.String(),
		"kind": kind, "createdBy": metadata.ActorID.String(),
	}
	if err := events.RecordTargeted(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "call.created", EventType: "call.invited", AggregateType: "call", AggregateID: callID,
		Summary: map[string]any{"kind": kind, "participantCount": len(recipients)}, Payload: payload,
	}, recipients); err != nil {
		return Call{}, err
	}
	return mapCall(row.ID, row.ConversationID, row.CallKind, row.State, "invited", row.CreatedBy,
		row.StartedAt, row.EndedAt, row.Version, row.CreatedAt), nil
}

func (service *Service) Get(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, callID, actorID ids.UUID,
) (Call, error) {
	row, err := workspace.Queries.GetCallForParticipant(ctx, dbgen.GetCallForParticipantParams{
		ActorUserID: actorID.PG(), WorkspaceID: workspaceID.PG(), CallID: callID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, errx.ErrNotFound
	}
	if err != nil {
		return Call{}, fmt.Errorf("get call: %w", err)
	}
	return mapCall(row.ID, row.ConversationID, row.CallKind, row.State, row.ParticipantState,
		row.CreatedBy, row.StartedAt, row.EndedAt, row.Version, row.CreatedAt), nil
}

func (service *Service) Join(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, callID, actorID ids.UUID, displayName string,
) (Call, JoinGrant, error) {
	if !service.Enabled() {
		return Call{}, JoinGrant{}, errx.ErrUnavailable
	}
	call, err := service.Get(ctx, workspace, workspaceID, callID, actorID)
	if err != nil {
		return Call{}, JoinGrant{}, err
	}
	if call.State == "ended" || call.ParticipantState == "declined" {
		return Call{}, JoinGrant{}, errx.ErrConflict
	}
	grant, err := service.provider.JoinToken("veltrix-call-"+call.ID.String(), actorID.String(), displayName)
	if err != nil {
		return Call{}, JoinGrant{}, fmt.Errorf("create call token: %w", err)
	}
	if _, err := workspace.Queries.JoinCallParticipant(ctx, dbgen.JoinCallParticipantParams{
		WorkspaceID: workspaceID.PG(), CallID: callID.PG(), ActorUserID: actorID.PG(),
	}); errors.Is(err, pgx.ErrNoRows) {
		return Call{}, JoinGrant{}, errx.ErrConflict
	} else if err != nil {
		return Call{}, JoinGrant{}, fmt.Errorf("join call: %w", err)
	}
	if err := workspace.Queries.ActivateCall(ctx, dbgen.ActivateCallParams{WorkspaceID: workspaceID.PG(), ID: callID.PG()}); err != nil {
		return Call{}, JoinGrant{}, fmt.Errorf("activate call: %w", err)
	}
	call, err = service.Get(ctx, workspace, workspaceID, callID, actorID)
	return call, grant, err
}

func (service *Service) Decline(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, callID, actorID ids.UUID,
) error {
	rows, err := workspace.Queries.DeclineCallParticipant(ctx, dbgen.DeclineCallParticipantParams{
		WorkspaceID: workspaceID.PG(), CallID: callID.PG(), UserID: actorID.PG(),
	})
	if err != nil {
		return fmt.Errorf("decline call: %w", err)
	}
	if rows != 1 {
		return errx.ErrConflict
	}
	return nil
}

func (service *Service) Leave(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, callID, actorID ids.UUID,
) error {
	rows, err := workspace.Queries.LeaveCallParticipant(ctx, dbgen.LeaveCallParticipantParams{
		WorkspaceID: workspaceID.PG(), CallID: callID.PG(), UserID: actorID.PG(),
	})
	if err != nil {
		return fmt.Errorf("leave call: %w", err)
	}
	if rows != 1 {
		return errx.ErrConflict
	}
	return nil
}

func (service *Service) End(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, callID ids.UUID,
) (Call, error) {
	row, err := workspace.Queries.EndCall(ctx, dbgen.EndCallParams{
		WorkspaceID: metadata.WorkspaceID.PG(), CallID: callID.PG(), ActorUserID: metadata.ActorID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Call{}, errx.ErrNotFound
	}
	if err != nil {
		return Call{}, fmt.Errorf("end call: %w", err)
	}
	if err := workspace.Queries.EndCallParticipants(ctx, dbgen.EndCallParticipantsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), CallID: callID.PG(),
	}); err != nil {
		return Call{}, fmt.Errorf("end call participants: %w", err)
	}
	recipients, err := service.participants(ctx, workspace, metadata.WorkspaceID, callID)
	if err != nil {
		return Call{}, err
	}
	if err := events.RecordTargeted(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "call.ended", EventType: "call.ended", AggregateType: "call", AggregateID: callID,
		Summary: map[string]any{"kind": row.CallKind},
		Payload: map[string]any{"callId": callID.String(), "conversationId": mustID(row.ConversationID).String()},
	}, recipients); err != nil {
		return Call{}, err
	}
	return mapCall(row.ID, row.ConversationID, row.CallKind, row.State, "left", row.CreatedBy,
		row.StartedAt, row.EndedAt, row.Version, row.CreatedAt), nil
}

func (service *Service) participants(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID, callID ids.UUID,
) ([]ids.UUID, error) {
	rows, err := workspace.Queries.ListCallParticipantUserIDs(ctx, dbgen.ListCallParticipantUserIDsParams{
		WorkspaceID: workspaceID.PG(), CallID: callID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("list call participants: %w", err)
	}
	result := make([]ids.UUID, 0, len(rows))
	for _, row := range rows {
		if value, valid := ids.FromPG(row); valid {
			result = append(result, value)
		}
	}
	return result, nil
}

func mapCall(
	id, conversationID pgtype.UUID, kind, state, participantState string, createdBy pgtype.UUID,
	startedAt, endedAt pgtype.Timestamptz, version int64, createdAt pgtype.Timestamptz,
) Call {
	return Call{
		ID: mustID(id), ConversationID: mustID(conversationID), Kind: kind, State: state,
		ParticipantState: participantState, CreatedBy: mustID(createdBy),
		StartedAt: optionalTime(startedAt), EndedAt: optionalTime(endedAt),
		Version: version, CreatedAt: createdAt.Time.UTC(),
	}
}

func mustID(value pgtype.UUID) ids.UUID { result, _ := ids.FromPG(value); return result }
func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
func validationError(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
