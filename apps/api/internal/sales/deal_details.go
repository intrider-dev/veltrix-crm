package sales

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (service *Service) ListDealLineItems(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
) ([]LineItemRecord, error) {
	if _, err := service.GetDeal(ctx, workspace, workspaceID, dealID); err != nil {
		return nil, err
	}
	rows, err := workspace.Queries.ListDealLineItems(ctx, dbgen.ListDealLineItemsParams{
		WorkspaceID: workspaceID.PG(), DealID: dealID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("list deal line items: %w", err)
	}
	result := make([]LineItemRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, makeLineItemRecord(row.ID, row.DealID, row.Name, row.Quantity,
			row.UnitPriceMinor, row.Currency, row.Position, row.Version))
	}
	return result, nil
}

func (service *Service) CreateDealLineItem(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	dealVersion int64,
	input LineItemInput,
) (LineItemRecord, int64, error) {
	validated, err := validateLineItem(input)
	if err != nil {
		return LineItemRecord{}, 0, err
	}
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, dealVersion, false); err != nil {
		return LineItemRecord{}, 0, err
	}
	existing, err := workspace.Queries.ListDealLineItems(ctx, dbgen.ListDealLineItemsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(),
	})
	if err != nil {
		return LineItemRecord{}, 0, fmt.Errorf("list deal line items: %w", err)
	}
	if len(existing) >= MaxDealLineItems {
		return LineItemRecord{}, 0, validation("/lineItems", "validation.max_items")
	}
	quantity, err := numericFromString(validated.Quantity)
	if err != nil {
		return LineItemRecord{}, 0, validation("/quantity", "validation.number.invalid")
	}
	itemID, err := ids.NewV7()
	if err != nil {
		return LineItemRecord{}, 0, err
	}
	row, err := workspace.Queries.CreateDealLineItem(ctx, dbgen.CreateDealLineItemParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: itemID.PG(), DealID: dealID.PG(),
		Name: validated.Name, Quantity: quantity, UnitPriceMinor: validated.UnitPriceMinor,
		Currency: validated.Currency, Position: int32(validated.Position),
	})
	if err != nil {
		return LineItemRecord{}, 0, mapConstraintError(err)
	}
	newDealVersion, err := service.bumpDealForDetails(ctx, workspace, metadata, dealID, dealVersion,
		"deal.line_item.created", "sales.deal.line_item.created", itemID, row.Version)
	if err != nil {
		return LineItemRecord{}, 0, err
	}
	return makeLineItemRecord(row.ID, row.DealID, row.Name, row.Quantity, row.UnitPriceMinor,
		row.Currency, row.Position, row.Version), newDealVersion, nil
}

func (service *Service) UpdateDealLineItem(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID, itemID ids.UUID,
	dealVersion, itemVersion int64,
	input LineItemInput,
) (LineItemRecord, int64, error) {
	validated, err := validateLineItem(input)
	if err != nil {
		return LineItemRecord{}, 0, err
	}
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, dealVersion, false); err != nil {
		return LineItemRecord{}, 0, err
	}
	quantity, err := numericFromString(validated.Quantity)
	if err != nil {
		return LineItemRecord{}, 0, validation("/quantity", "validation.number.invalid")
	}
	row, err := workspace.Queries.UpdateDealLineItem(ctx, dbgen.UpdateDealLineItemParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(), ID: itemID.PG(),
		Name: validated.Name, Quantity: quantity, UnitPriceMinor: validated.UnitPriceMinor,
		Currency: validated.Currency, Position: int32(validated.Position), Version: itemVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LineItemRecord{}, 0, service.classifyLineItemMutation(ctx, workspace, metadata.WorkspaceID, dealID, itemID, itemVersion)
	}
	if err != nil {
		return LineItemRecord{}, 0, mapConstraintError(err)
	}
	newDealVersion, err := service.bumpDealForDetails(ctx, workspace, metadata, dealID, dealVersion,
		"deal.line_item.updated", "sales.deal.line_item.updated", itemID, row.Version)
	if err != nil {
		return LineItemRecord{}, 0, err
	}
	return makeLineItemRecord(row.ID, row.DealID, row.Name, row.Quantity, row.UnitPriceMinor,
		row.Currency, row.Position, row.Version), newDealVersion, nil
}

func (service *Service) DeleteDealLineItem(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID, itemID ids.UUID,
	dealVersion, itemVersion int64,
) (int64, error) {
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, dealVersion, false); err != nil {
		return 0, err
	}
	_, err := workspace.Queries.DeleteDealLineItem(ctx, dbgen.DeleteDealLineItemParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(), ID: itemID.PG(), Version: itemVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, service.classifyLineItemMutation(ctx, workspace, metadata.WorkspaceID, dealID, itemID, itemVersion)
	}
	if err != nil {
		return 0, fmt.Errorf("delete deal line item: %w", err)
	}
	return service.bumpDealForDetails(ctx, workspace, metadata, dealID, dealVersion,
		"deal.line_item.deleted", "sales.deal.line_item.deleted", itemID, itemVersion+1)
}

func (service *Service) ListDealParticipants(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID ids.UUID,
) ([]DealParticipantRecord, error) {
	if _, err := service.GetDeal(ctx, workspace, workspaceID, dealID); err != nil {
		return nil, err
	}
	rows, err := workspace.Queries.ListDealParticipants(ctx, dbgen.ListDealParticipantsParams{
		WorkspaceID: workspaceID.PG(), DealID: dealID.PG(),
	})
	if err != nil {
		return nil, fmt.Errorf("list deal participants: %w", err)
	}
	return participantRecords(rows), nil
}

func (service *Service) UpsertDealParticipant(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	dealVersion int64,
	input DealParticipantInput,
) (DealParticipantRecord, int64, error) {
	validated, err := validateParticipant(input)
	if err != nil {
		return DealParticipantRecord{}, 0, err
	}
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, dealVersion, false); err != nil {
		return DealParticipantRecord{}, 0, err
	}
	existing, err := workspace.Queries.ListDealParticipants(ctx, dbgen.ListDealParticipantsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(),
	})
	if err != nil {
		return DealParticipantRecord{}, 0, fmt.Errorf("list deal participants: %w", err)
	}
	found := false
	for _, participant := range existing {
		participantID, _ := ids.FromPG(participant.ContactID)
		found = found || participantID == validated.ContactID
	}
	if !found && len(existing) >= MaxDealParticipants {
		return DealParticipantRecord{}, 0, validation("/participants", "validation.max_items")
	}
	row, err := workspace.Queries.UpsertDealParticipant(ctx, dbgen.UpsertDealParticipantParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(), ContactID: validated.ContactID.PG(), Role: validated.Role,
	})
	if err != nil {
		return DealParticipantRecord{}, 0, mapConstraintError(err)
	}
	newDealVersion, err := service.bumpDealForDetails(ctx, workspace, metadata, dealID, dealVersion,
		"deal.participant.updated", "sales.deal.participant.updated", validated.ContactID, row.Version)
	if err != nil {
		return DealParticipantRecord{}, 0, err
	}
	rows, err := workspace.Queries.ListDealParticipants(ctx, dbgen.ListDealParticipantsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(),
	})
	if err != nil {
		return DealParticipantRecord{}, 0, fmt.Errorf("reload deal participants: %w", err)
	}
	for _, participant := range participantRecords(rows) {
		if participant.ContactID == validated.ContactID.String() {
			return participant, newDealVersion, nil
		}
	}
	return DealParticipantRecord{}, 0, fmt.Errorf("reload deal participant: %w", errx.ErrNotFound)
}

func (service *Service) DeleteDealParticipant(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID, contactID ids.UUID,
	dealVersion, participantVersion int64,
) (int64, error) {
	if _, err := service.requireDealVersion(ctx, workspace, metadata.WorkspaceID, dealID, dealVersion, false); err != nil {
		return 0, err
	}
	_, err := workspace.Queries.DeleteDealParticipant(ctx, dbgen.DeleteDealParticipantParams{
		WorkspaceID: metadata.WorkspaceID.PG(), DealID: dealID.PG(), ContactID: contactID.PG(), Version: participantVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, service.classifyParticipantMutation(ctx, workspace, metadata.WorkspaceID, dealID, contactID, participantVersion)
	}
	if err != nil {
		return 0, fmt.Errorf("delete deal participant: %w", err)
	}
	return service.bumpDealForDetails(ctx, workspace, metadata, dealID, dealVersion,
		"deal.participant.deleted", "sales.deal.participant.deleted", contactID, participantVersion+1)
}

func (service *Service) bumpDealForDetails(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	dealID ids.UUID,
	dealVersion int64,
	action, eventType string,
	childID ids.UUID,
	childVersion int64,
) (int64, error) {
	newVersion, err := workspace.Queries.BumpDealVersion(ctx, dbgen.BumpDealVersionParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: dealID.PG(), Version: dealVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, service.classifyDealMutation(ctx, workspace, metadata.WorkspaceID, dealID, dealVersion, false)
	}
	if err != nil {
		return 0, fmt.Errorf("advance deal version: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: action, EventType: eventType, AggregateType: "deal", AggregateID: dealID,
		Summary: map[string]any{"childId": childID.String()},
		Payload: map[string]any{"dealId": dealID.String(), "childId": childID.String(), "childVersion": childVersion, "version": newVersion},
	}); err != nil {
		return 0, err
	}
	return newVersion, nil
}

func (service *Service) classifyLineItemMutation(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID, itemID ids.UUID,
	version int64,
) error {
	var current int64
	err := workspace.Tx.QueryRow(ctx, `SELECT version FROM sales.deal_line_items
		WHERE workspace_id = $1 AND deal_id = $2 AND id = $3`,
		workspaceID.PG(), dealID.PG(), itemID.PG(),
	).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify deal line item: %w", err)
	}
	if current != version {
		return errx.ErrVersionConflict
	}
	return errx.ErrConflict
}

func (service *Service) classifyParticipantMutation(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, dealID, contactID ids.UUID,
	version int64,
) error {
	var current int64
	err := workspace.Tx.QueryRow(ctx, `SELECT version FROM sales.deal_participants
		WHERE workspace_id = $1 AND deal_id = $2 AND contact_id = $3`,
		workspaceID.PG(), dealID.PG(), contactID.PG(),
	).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify deal participant: %w", err)
	}
	if current != version {
		return errx.ErrVersionConflict
	}
	return errx.ErrConflict
}

func numericFromString(value string) (pgtype.Numeric, error) {
	var numeric pgtype.Numeric
	if err := numeric.Scan(value); err != nil {
		return pgtype.Numeric{}, err
	}
	return numeric, nil
}

func numericToString(value pgtype.Numeric) string {
	var raw driver.Value
	raw, err := value.Value()
	if err != nil || raw == nil {
		return "0"
	}
	if result, ok := raw.(string); ok {
		return result
	}
	return fmt.Sprint(raw)
}

func makeLineItemRecord(
	id, dealID pgtype.UUID,
	name string,
	quantity pgtype.Numeric,
	unitPriceMinor int64,
	currency string,
	position int32,
	version int64,
) LineItemRecord {
	item, _ := ids.FromPG(id)
	parent, _ := ids.FromPG(dealID)
	return LineItemRecord{
		ID: item.String(), DealID: parent.String(), Name: name, Quantity: numericToString(quantity),
		UnitPriceMinor: unitPriceMinor, Currency: currency, Position: int(position), Version: version,
	}
}

func participantRecords(rows []dbgen.ListDealParticipantsRow) []DealParticipantRecord {
	result := make([]DealParticipantRecord, 0, len(rows))
	for _, row := range rows {
		contactID, _ := ids.FromPG(row.ContactID)
		result = append(result, DealParticipantRecord{
			ContactID: contactID.String(), DisplayName: row.DisplayName, Email: row.Email, Role: row.Role,
			Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
		})
	}
	return result
}
