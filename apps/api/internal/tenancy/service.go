package tenancy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

type Permission string

const (
	PermissionRecordsRead      Permission = "records.read"
	PermissionRecordsCreate    Permission = "records.create"
	PermissionRecordsUpdate    Permission = "records.update"
	PermissionRecordsDelete    Permission = "records.delete"
	PermissionDataExport       Permission = "data.export"
	PermissionReportsRead      Permission = "reports.read"
	PermissionAuditRead        Permission = "audit.read"
	PermissionSettingsWrite    Permission = "settings.write"
	PermissionMembersRead      Permission = "members.read"
	PermissionMembersWrite     Permission = "members.write"
	PermissionRolesWrite       Permission = "roles.write"
	PermissionLeadsRead        Permission = "leads.read"
	PermissionLeadsCreate      Permission = "leads.create"
	PermissionLeadsUpdate      Permission = "leads.update"
	PermissionLeadsDelete      Permission = "leads.delete"
	PermissionDealsRead        Permission = "deals.read"
	PermissionDealsCreate      Permission = "deals.create"
	PermissionDealsUpdate      Permission = "deals.update"
	PermissionDealsDelete      Permission = "deals.delete"
	PermissionLeadStagesManage Permission = "lead_stages.manage"
	PermissionDealStagesManage Permission = "deal_stages.manage"
)

type WorkspaceTx struct {
	Tx          pgx.Tx
	Queries     *dbgen.Queries
	Membership  dbgen.TenancyMembership
	Permissions map[Permission]struct{}
}

func (workspace *WorkspaceTx) Allows(permission Permission) bool {
	_, allowed := workspace.Permissions[permission]
	return allowed
}

type Service struct {
	pool             *pgxpool.Pool
	supportedLocales map[string]struct{}
	supportedList    []string
	defaultLocale    string
}

type ServiceOptions struct {
	SupportedLocales []string
	DefaultLocale    string
}

func NewService(pool *pgxpool.Pool) *Service {
	return NewServiceWithOptions(pool, ServiceOptions{
		SupportedLocales: []string{"en", "ru"}, DefaultLocale: "en",
	})
}

func NewServiceWithOptions(pool *pgxpool.Pool, options ServiceOptions) *Service {
	if len(options.SupportedLocales) == 0 {
		options.SupportedLocales = []string{"en", "ru"}
	}
	locales := make(map[string]struct{}, len(options.SupportedLocales))
	localeList := make([]string, 0, len(options.SupportedLocales))
	firstLocale := ""
	for _, locale := range options.SupportedLocales {
		normalized := normalizeLocale(locale)
		if normalized != "" {
			if _, exists := locales[normalized]; exists {
				continue
			}
			locales[normalized] = struct{}{}
			localeList = append(localeList, normalized)
			if firstLocale == "" {
				firstLocale = normalized
			}
		}
	}
	if len(localeList) == 0 {
		localeList = []string{"en", "ru"}
		locales = map[string]struct{}{"en": {}, "ru": {}}
		firstLocale = "en"
	}
	defaultLocale := normalizeLocale(options.DefaultLocale)
	if _, ok := locales[defaultLocale]; !ok {
		defaultLocale = firstLocale
	}
	return &Service{
		pool: pool, supportedLocales: locales, supportedList: localeList, defaultLocale: defaultLocale,
	}
}

func (service *Service) SupportsLocale(locale string) bool {
	_, ok := service.supportedLocales[normalizeLocale(locale)]
	return ok
}

func (service *Service) DefaultLocale() string { return service.defaultLocale }

func (service *Service) SupportedLocales() []string {
	return append([]string(nil), service.supportedList...)
}

func (service *Service) ListWorkspaces(ctx context.Context, principal identity.Principal) ([]dbgen.ListUserWorkspacesRow, error) {
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetActorContext(ctx, principal.UserID.String()); err != nil {
		return nil, fmt.Errorf("set actor context: %w", err)
	}
	rows, err := queries.ListUserWorkspaces(ctx, principal.UserID.PG())
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rows, nil
}

func (service *Service) WithWorkspace(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
	permission Permission,
	fn func(*WorkspaceTx) error,
) error {
	return service.withWorkspace(ctx, principal, workspaceID, requestID, []Permission{permission}, fn)
}

// WithWorkspaceAny opens a tenant transaction when the active member has at
// least one of the supplied capabilities. It is used by shared resources such
// as a conversation that can be reached from records.read, leads.read, or
// deals.read without weakening the entity-specific checks inside that resource.
func (service *Service) WithWorkspaceAny(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
	permissions []Permission,
	fn func(*WorkspaceTx) error,
) error {
	if len(permissions) == 0 {
		return errx.ErrForbidden
	}
	return service.withWorkspace(ctx, principal, workspaceID, requestID, permissions, fn)
}

func (service *Service) withWorkspace(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
	permissions []Permission,
	fn func(*WorkspaceTx) error,
) error {
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetWorkspaceContext(ctx, dbgen.SetWorkspaceContextParams{
		ActorID: principal.UserID.String(), WorkspaceID: workspaceID.String(), RequestID: requestID,
	}); err != nil {
		return fmt.Errorf("set workspace context: %w", err)
	}
	authorization, err := queries.AuthorizeWorkspace(ctx, dbgen.AuthorizeWorkspaceParams{
		WorkspaceID: workspaceID.PG(),
		ActorID:     principal.UserID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errx.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("authorize workspace: %w", err)
	}
	membership := dbgen.TenancyMembership{
		WorkspaceID:      authorization.WorkspaceID,
		ID:               authorization.ID,
		UserID:           authorization.UserID,
		Role:             authorization.Role,
		Status:           authorization.Status,
		LocaleOverride:   authorization.LocaleOverride,
		TimezoneOverride: authorization.TimezoneOverride,
		CreatedAt:        authorization.CreatedAt,
		UpdatedAt:        authorization.UpdatedAt,
		RoleID:           authorization.RoleID,
	}
	effective := make(map[Permission]struct{}, len(authorization.Permissions))
	for _, grant := range authorization.Permissions {
		effective[Permission(grant)] = struct{}{}
	}
	allowed := false
	for _, permission := range permissions {
		if _, exists := effective[permission]; exists {
			allowed = true
			break
		}
	}
	if !allowed {
		return errx.ErrForbidden
	}
	if err := fn(&WorkspaceTx{Tx: tx, Queries: queries, Membership: membership, Permissions: effective}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	return nil
}

func RoleAllows(role string, permission Permission) bool {
	switch permission {
	case PermissionLeadsRead, PermissionDealsRead:
		return RoleAllows(role, PermissionRecordsRead)
	case PermissionLeadsCreate, PermissionDealsCreate:
		return RoleAllows(role, PermissionRecordsCreate)
	case PermissionLeadsUpdate, PermissionDealsUpdate:
		return RoleAllows(role, PermissionRecordsUpdate)
	case PermissionLeadsDelete, PermissionDealsDelete:
		return RoleAllows(role, PermissionRecordsDelete)
	case PermissionLeadStagesManage, PermissionDealStagesManage:
		return role == "owner" || role == "admin"
	}
	switch role {
	case "owner":
		return knownPermission(permission)
	case "admin":
		return permission != PermissionRolesWrite && knownPermission(permission)
	case "manager":
		switch permission {
		case PermissionRecordsRead, PermissionRecordsCreate, PermissionRecordsUpdate,
			PermissionRecordsDelete, PermissionDataExport, PermissionReportsRead,
			PermissionAuditRead, PermissionMembersRead:
			return true
		default:
			return false
		}
	case "sales":
		switch permission {
		case PermissionRecordsRead, PermissionRecordsCreate, PermissionRecordsUpdate:
			return true
		default:
			return false
		}
	case "viewer":
		return permission == PermissionRecordsRead || permission == PermissionReportsRead
	default:
		return false
	}
}

func knownPermission(permission Permission) bool {
	switch permission {
	case PermissionRecordsRead, PermissionRecordsCreate, PermissionRecordsUpdate,
		PermissionRecordsDelete, PermissionDataExport, PermissionReportsRead,
		PermissionAuditRead, PermissionSettingsWrite, PermissionMembersRead,
		PermissionMembersWrite:
		return true
	case PermissionRolesWrite:
		return true
	case PermissionLeadsRead, PermissionLeadsCreate, PermissionLeadsUpdate, PermissionLeadsDelete,
		PermissionDealsRead, PermissionDealsCreate, PermissionDealsUpdate, PermissionDealsDelete,
		PermissionLeadStagesManage, PermissionDealStagesManage:
		return true
	default:
		return false
	}
}
