package reporting

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) Dashboard(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID) (dbgen.GetDashboardSummaryRow, error) {
	row, err := workspace.Queries.GetDashboardSummary(ctx, workspaceID.PG())
	if errors.Is(err, pgx.ErrNoRows) {
		if err := workspace.Queries.RefreshDashboardSummary(ctx, workspaceID.PG()); err != nil {
			return dbgen.GetDashboardSummaryRow{}, fmt.Errorf("initialize dashboard summary: %w", err)
		}
		row, err = workspace.Queries.GetDashboardSummary(ctx, workspaceID.PG())
	}
	if err != nil {
		return dbgen.GetDashboardSummaryRow{}, fmt.Errorf("get dashboard summary: %w", err)
	}
	return row, nil
}
