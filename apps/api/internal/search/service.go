package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) Global(ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, query string) ([]dbgen.GlobalSearchRow, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 || len(query) > 120 {
		return nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/q", Code: "validation.length"}}}
	}
	rows, err := workspace.Queries.GlobalSearch(ctx, dbgen.GlobalSearchParams{SearchQuery: query, WorkspaceID: workspaceID.PG()})
	if err != nil {
		return nil, fmt.Errorf("global search: %w", err)
	}
	return rows, nil
}
