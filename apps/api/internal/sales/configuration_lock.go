package sales

import (
	"context"
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// lockSalesConfiguration serializes small workspace-scoped configuration
// mutations whose next position/default selection is derived from existing
// rows. It is transaction-scoped and does not consume a pooled connection
// after commit or rollback.
func lockSalesConfiguration(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	scope string,
) error {
	if _, err := workspace.Tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		workspaceID.String()+":"+scope,
	); err != nil {
		return fmt.Errorf("lock sales configuration: %w", err)
	}
	return nil
}
