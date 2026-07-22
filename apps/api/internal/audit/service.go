package audit

import (
	"context"
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) List(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, limit int) ([]dbgen.ListAuditEventsRow, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := workspace.Queries.ListAuditEvents(ctx, dbgen.ListAuditEventsParams{WorkspaceID: workspaceID.PG(), Limit: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	return rows, nil
}
