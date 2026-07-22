package tenancy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

var (
	ErrInvalidInvitation = errors.New("invalid or expired invitation")
	ErrLastOwner         = errors.New("workspace must retain an active owner")
	ErrCannotManageOwner = errors.New("only an owner can manage owner memberships")
)

var (
	workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
	localePattern        = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})*$`)
)

type CreateWorkspaceRequest struct {
	Name            string
	Slug            string
	DefaultLocale   string
	Timezone        string
	DefaultCurrency string
}

type CreatedWorkspace struct {
	Workspace  dbgen.TenancyWorkspace
	Membership dbgen.TenancyMembership
}

type Invitation struct {
	ID          ids.UUID
	WorkspaceID ids.UUID
	Email       string
	Role        string
	Token       string
	ExpiresAt   time.Time
}

func (service *Service) CreateWorkspace(
	ctx context.Context,
	principal identity.Principal,
	requestID string,
	request CreateWorkspaceRequest,
) (CreatedWorkspace, error) {
	name := strings.TrimSpace(request.Name)
	if count := utf8.RuneCountInString(name); count < 1 || count > 160 {
		return CreatedWorkspace{}, validation("/name", "validation.length")
	}
	slug := strings.ToLower(strings.TrimSpace(request.Slug))
	if !workspaceSlugPattern.MatchString(slug) {
		return CreatedWorkspace{}, validation("/slug", "validation.slug")
	}
	locale := strings.ToLower(strings.TrimSpace(request.DefaultLocale))
	if !service.SupportsLocale(locale) {
		return CreatedWorkspace{}, validation("/defaultLocale", "validation.locale.unsupported")
	}
	timezone := strings.TrimSpace(request.Timezone)
	if _, err := time.LoadLocation(timezone); timezone == "" || err != nil {
		return CreatedWorkspace{}, validation("/timezone", "validation.timezone")
	}
	currency := strings.ToUpper(strings.TrimSpace(request.DefaultCurrency))
	if !validCurrency(currency) {
		return CreatedWorkspace{}, validation("/defaultCurrency", "validation.currency")
	}
	workspaceID, err := ids.NewV7()
	if err != nil {
		return CreatedWorkspace{}, err
	}
	membershipID, err := ids.NewV7()
	if err != nil {
		return CreatedWorkspace{}, err
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreatedWorkspace{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetActorContext(ctx, principal.UserID.String()); err != nil {
		return CreatedWorkspace{}, fmt.Errorf("set workspace creator: %w", err)
	}
	// Scope the transaction before INSERT ... RETURNING. PostgreSQL evaluates
	// the SELECT RLS policy for the returned row, while the owner membership is
	// deliberately created only after the workspace exists.
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: workspaceID.String(), RequestID: requestID,
	}); err != nil {
		return CreatedWorkspace{}, fmt.Errorf("scope new workspace: %w", err)
	}
	workspace, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID: workspaceID.PG(), Name: name, Slug: slug, DefaultLocale: locale,
		Timezone: timezone, DefaultCurrency: currency, SupportedLocales: service.SupportedLocales(),
	})
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return CreatedWorkspace{}, validation("/slug", "validation.slug.alreadyUsed")
		}
		return CreatedWorkspace{}, fmt.Errorf("create workspace: %w", err)
	}
	membership, err := queries.CreateMembership(ctx, dbgen.CreateMembershipParams{
		WorkspaceID: workspaceID.PG(), ID: membershipID.PG(),
		UserID: principal.UserID.PG(), RoleKey: "owner",
	})
	if err != nil {
		return CreatedWorkspace{}, fmt.Errorf("create owner membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return CreatedWorkspace{}, fmt.Errorf("commit workspace creation: %w", err)
	}
	return CreatedWorkspace{Workspace: dbgen.TenancyWorkspace{
		ID: workspace.ID, Name: workspace.Name, Slug: workspace.Slug,
		DefaultLocale: workspace.DefaultLocale, Timezone: workspace.Timezone,
		DefaultCurrency: workspace.DefaultCurrency, SupportedLocales: workspace.SupportedLocales,
		Version:   workspace.Version,
		CreatedAt: workspace.CreatedAt, UpdatedAt: workspace.UpdatedAt,
	}, Membership: membership}, nil
}

func (service *Service) InviteMember(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID,
	email,
	role string,
	ttl time.Duration,
) (Invitation, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return Invitation{}, validation("/email", "validation.email")
	}
	if !validInvitedRole(role) {
		return Invitation{}, validation("/role", "validation.role")
	}
	if ttl <= 0 || ttl > 30*24*time.Hour {
		return Invitation{}, validation("/expiresIn", "validation.range")
	}
	invitationID, err := ids.NewV7()
	if err != nil {
		return Invitation{}, err
	}
	randomPart, tokenHash, err := secureToken()
	if err != nil {
		return Invitation{}, err
	}
	expiresAt := time.Now().UTC().Add(ttl)
	result := Invitation{
		ID: invitationID, WorkspaceID: workspaceID, Email: normalizedEmail,
		Role: role, Token: workspaceID.String() + "." + randomPart, ExpiresAt: expiresAt,
	}
	err = service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersWrite,
		func(workspace *WorkspaceTx) error {
			_, createErr := workspace.Queries.CreateInvitation(ctx, dbgen.CreateInvitationParams{
				WorkspaceID: workspaceID.PG(), ID: invitationID.PG(),
				EmailNormalized: normalizedEmail, Role: role, TokenHash: tokenHash[:],
				InvitedBy: workspace.Membership.ID,
				ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			if createErr != nil {
				return fmt.Errorf("create invitation: %w", createErr)
			}
			return nil
		})
	if err != nil {
		return Invitation{}, err
	}
	return result, nil
}

func (service *Service) AcceptInvitation(
	ctx context.Context,
	principal identity.Principal,
	rawToken,
	requestID string,
) (dbgen.TenancyMembership, error) {
	workspaceID, tokenHash, ok := parseInvitationToken(rawToken)
	if !ok {
		return dbgen.TenancyMembership{}, ErrInvalidInvitation
	}
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return dbgen.TenancyMembership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetActorContext(ctx, principal.UserID.String()); err != nil {
		return dbgen.TenancyMembership{}, fmt.Errorf("set invitation actor: %w", err)
	}
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: workspaceID.String(), RequestID: requestID,
	}); err != nil {
		return dbgen.TenancyMembership{}, fmt.Errorf("scope invitation acceptance: %w", err)
	}
	invitation, err := queries.LockInvitationByHash(ctx, dbgen.LockInvitationByHashParams{
		WorkspaceID: workspaceID.PG(), TokenHash: tokenHash[:],
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.TenancyMembership{}, ErrInvalidInvitation
	}
	if err != nil {
		return dbgen.TenancyMembership{}, fmt.Errorf("lock invitation: %w", err)
	}
	principalEmail, err := normalizeEmail(principal.Email)
	if err != nil || principalEmail != invitation.EmailNormalized {
		return dbgen.TenancyMembership{}, ErrInvalidInvitation
	}
	if err := queries.LockWorkspaceMembershipMutation(ctx, workspaceID.String()); err != nil {
		return dbgen.TenancyMembership{}, fmt.Errorf("lock workspace membership set: %w", err)
	}
	membership, err := queries.GetMembershipByUserID(ctx, dbgen.GetMembershipByUserIDParams{
		WorkspaceID: workspaceID.PG(), UserID: principal.UserID.PG(),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		membershipID, idErr := ids.NewV7()
		if idErr != nil {
			return dbgen.TenancyMembership{}, idErr
		}
		membership, err = queries.CreateMembership(ctx, dbgen.CreateMembershipParams{
			WorkspaceID: workspaceID.PG(), ID: membershipID.PG(),
			UserID: principal.UserID.PG(), RoleKey: invitation.Role,
		})
		if err != nil {
			return dbgen.TenancyMembership{}, fmt.Errorf("create invited membership: %w", err)
		}
	case err != nil:
		return dbgen.TenancyMembership{}, fmt.Errorf("load existing membership: %w", err)
	case membership.Status == "disabled" && membership.Role == "owner":
		return dbgen.TenancyMembership{}, ErrInvalidInvitation
	case membership.Status == "disabled":
		membership, err = queries.UpdateMembershipRole(ctx, dbgen.UpdateMembershipRoleParams{
			WorkspaceID: workspaceID.PG(), ID: membership.ID, RoleKey: invitation.Role,
		})
		if err != nil {
			return dbgen.TenancyMembership{}, fmt.Errorf("apply invited membership role: %w", err)
		}
		membership, err = queries.UpdateMembershipStatus(ctx, dbgen.UpdateMembershipStatusParams{
			WorkspaceID: workspaceID.PG(), ID: membership.ID, Status: "active",
		})
		if err != nil {
			return dbgen.TenancyMembership{}, fmt.Errorf("reactivate invited membership: %w", err)
		}
	}
	accepted, err := queries.AcceptInvitation(ctx, dbgen.AcceptInvitationParams{
		WorkspaceID: workspaceID.PG(), ID: invitation.ID, AcceptedByUserID: principal.UserID.PG(),
	})
	if err != nil {
		return dbgen.TenancyMembership{}, fmt.Errorf("accept invitation: %w", err)
	}
	if accepted != 1 {
		return dbgen.TenancyMembership{}, ErrInvalidInvitation
	}
	if err := tx.Commit(ctx); err != nil {
		return dbgen.TenancyMembership{}, fmt.Errorf("commit invitation acceptance: %w", err)
	}
	return membership, nil
}

func (service *Service) ListMembers(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
) ([]dbgen.ListWorkspaceMembersRow, error) {
	var result []dbgen.ListWorkspaceMembersRow
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersRead,
		func(workspace *WorkspaceTx) error {
			rows, err := workspace.Queries.ListWorkspaceMembers(ctx, workspaceID.PG())
			result = rows
			return err
		})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *Service) UpdateMemberRole(
	ctx context.Context,
	principal identity.Principal,
	workspaceID,
	membershipID ids.UUID,
	requestID,
	nextRole string,
) (dbgen.TenancyMembership, error) {
	if !validRole(nextRole) {
		return dbgen.TenancyMembership{}, validation("/role", "validation.role")
	}
	var result dbgen.TenancyMembership
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersWrite,
		func(workspace *WorkspaceTx) error {
			if err := workspace.Queries.LockWorkspaceMembershipMutation(ctx, workspaceID.String()); err != nil {
				return err
			}
			target, err := workspace.Queries.GetMembershipByIDForUpdate(ctx, dbgen.GetMembershipByIDForUpdateParams{
				WorkspaceID: workspaceID.PG(), ID: membershipID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			}
			if err != nil {
				return err
			}
			ownerCount, err := workspace.Queries.CountActiveOwners(ctx, workspaceID.PG())
			if err != nil {
				return err
			}
			if err := ValidateMemberTransition(workspace.Membership.Role, target.Role, target.Status, nextRole, target.Status, ownerCount); err != nil {
				return err
			}
			result, err = workspace.Queries.UpdateMembershipRole(ctx, dbgen.UpdateMembershipRoleParams{
				WorkspaceID: workspaceID.PG(), ID: membershipID.PG(), RoleKey: nextRole,
			})
			return err
		})
	return result, err
}

func (service *Service) SetMemberStatus(
	ctx context.Context,
	principal identity.Principal,
	workspaceID,
	membershipID ids.UUID,
	requestID,
	nextStatus string,
) (dbgen.TenancyMembership, error) {
	if nextStatus != "active" && nextStatus != "disabled" {
		return dbgen.TenancyMembership{}, validation("/status", "validation.status")
	}
	var result dbgen.TenancyMembership
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersWrite,
		func(workspace *WorkspaceTx) error {
			if err := workspace.Queries.LockWorkspaceMembershipMutation(ctx, workspaceID.String()); err != nil {
				return err
			}
			target, err := workspace.Queries.GetMembershipByIDForUpdate(ctx, dbgen.GetMembershipByIDForUpdateParams{
				WorkspaceID: workspaceID.PG(), ID: membershipID.PG(),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			}
			if err != nil {
				return err
			}
			ownerCount, err := workspace.Queries.CountActiveOwners(ctx, workspaceID.PG())
			if err != nil {
				return err
			}
			if err := ValidateMemberTransition(workspace.Membership.Role, target.Role, target.Status, target.Role, nextStatus, ownerCount); err != nil {
				return err
			}
			result, err = workspace.Queries.UpdateMembershipStatus(ctx, dbgen.UpdateMembershipStatusParams{
				WorkspaceID: workspaceID.PG(), ID: membershipID.PG(), Status: nextStatus,
			})
			return err
		})
	return result, err
}

func ValidateMemberTransition(actorRole, currentRole, currentStatus, nextRole, nextStatus string, activeOwners int64) error {
	if (currentRole == "owner" || nextRole == "owner") && actorRole != "owner" {
		return ErrCannotManageOwner
	}
	removesActiveOwner := currentRole == "owner" && currentStatus == "active" &&
		(nextRole != "owner" || nextStatus != "active")
	if removesActiveOwner && activeOwners <= 1 {
		return ErrLastOwner
	}
	return nil
}

func (service *Service) UpdateWorkspaceDefaults(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID,
	locale,
	timezone,
	currency string,
	expectedVersion int64,
) (dbgen.TenancyWorkspace, error) {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if !service.SupportsLocale(locale) {
		return dbgen.TenancyWorkspace{}, validation("/defaultLocale", "validation.locale.unsupported")
	}
	if _, err := time.LoadLocation(timezone); strings.TrimSpace(timezone) == "" || err != nil {
		return dbgen.TenancyWorkspace{}, validation("/timezone", "validation.timezone")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !validCurrency(currency) {
		return dbgen.TenancyWorkspace{}, validation("/defaultCurrency", "validation.currency")
	}
	var result dbgen.TenancyWorkspace
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionSettingsWrite,
		func(workspace *WorkspaceTx) error {
			row, err := workspace.Queries.UpdateWorkspaceDefaults(ctx, dbgen.UpdateWorkspaceDefaultsParams{
				WorkspaceID: workspaceID.PG(), DefaultLocale: locale,
				Timezone: timezone, DefaultCurrency: currency, ExpectedVersion: expectedVersion,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrVersionConflict
			}
			result = dbgen.TenancyWorkspace{
				ID: row.ID, Name: row.Name, Slug: row.Slug, DefaultLocale: row.DefaultLocale,
				Timezone: row.Timezone, DefaultCurrency: row.DefaultCurrency,
				SupportedLocales: row.SupportedLocales, Version: row.Version,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			}
			return err
		})
	return result, err
}

func (service *Service) SetMyWorkspaceLocale(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
	localeOverride *string,
) (dbgen.TenancyMembership, error) {
	if localeOverride != nil {
		normalized := strings.ToLower(strings.TrimSpace(*localeOverride))
		if !service.SupportsLocale(normalized) {
			return dbgen.TenancyMembership{}, validation("/locale", "validation.locale.unsupported")
		}
		localeOverride = &normalized
	}
	var result dbgen.TenancyMembership
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionRecordsRead,
		func(workspace *WorkspaceTx) error {
			row, err := workspace.Queries.UpdateMembershipLocaleOverride(ctx, dbgen.UpdateMembershipLocaleOverrideParams{
				WorkspaceID: workspaceID.PG(), UserID: principal.UserID.PG(), LocaleOverride: localeOverride,
			})
			result = row
			return err
		})
	return result, err
}

func (service *Service) CreateTeam(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID,
	name string,
) (dbgen.TenancyTeam, error) {
	name = strings.TrimSpace(name)
	if count := utf8.RuneCountInString(name); count < 1 || count > 120 {
		return dbgen.TenancyTeam{}, validation("/name", "validation.length")
	}
	teamID, err := ids.NewV7()
	if err != nil {
		return dbgen.TenancyTeam{}, err
	}
	var result dbgen.TenancyTeam
	err = service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersWrite,
		func(workspace *WorkspaceTx) error {
			row, err := workspace.Queries.CreateTeam(ctx, dbgen.CreateTeamParams{
				WorkspaceID: workspaceID.PG(), ID: teamID.PG(), Name: name,
			})
			result = row
			return err
		})
	return result, err
}

func (service *Service) ListTeams(
	ctx context.Context,
	principal identity.Principal,
	workspaceID ids.UUID,
	requestID string,
) ([]dbgen.TenancyTeam, error) {
	var result []dbgen.TenancyTeam
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionRecordsRead,
		func(workspace *WorkspaceTx) error {
			rows, err := workspace.Queries.ListTeams(ctx, workspaceID.PG())
			result = rows
			return err
		})
	return result, err
}

func (service *Service) ListTeamMembers(
	ctx context.Context,
	principal identity.Principal,
	workspaceID,
	teamID ids.UUID,
	requestID string,
) ([]dbgen.ListTeamMembershipsRow, error) {
	var result []dbgen.ListTeamMembershipsRow
	err := service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersRead,
		func(workspace *WorkspaceTx) error {
			if _, err := workspace.Queries.GetTeam(ctx, dbgen.GetTeamParams{
				WorkspaceID: workspaceID.PG(), ID: teamID.PG(),
			}); errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			} else if err != nil {
				return err
			}
			rows, err := workspace.Queries.ListTeamMemberships(ctx, dbgen.ListTeamMembershipsParams{
				WorkspaceID: workspaceID.PG(), TeamID: teamID.PG(),
			})
			result = rows
			return err
		})
	return result, err
}

func (service *Service) SetTeamMember(
	ctx context.Context,
	principal identity.Principal,
	workspaceID,
	teamID,
	membershipID ids.UUID,
	requestID string,
	present bool,
) error {
	return service.WithWorkspace(ctx, principal, workspaceID, requestID, PermissionMembersWrite,
		func(workspace *WorkspaceTx) error {
			if _, err := workspace.Queries.GetTeam(ctx, dbgen.GetTeamParams{
				WorkspaceID: workspaceID.PG(), ID: teamID.PG(),
			}); errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			} else if err != nil {
				return err
			}
			if _, err := workspace.Queries.GetMembershipByIDForUpdate(ctx, dbgen.GetMembershipByIDForUpdateParams{
				WorkspaceID: workspaceID.PG(), ID: membershipID.PG(),
			}); errors.Is(err, pgx.ErrNoRows) {
				return errx.ErrNotFound
			} else if err != nil {
				return err
			}
			params := dbgen.AddTeamMembershipParams{
				WorkspaceID: workspaceID.PG(), TeamID: teamID.PG(), MembershipID: membershipID.PG(),
			}
			if present {
				return workspace.Queries.AddTeamMembership(ctx, params)
			}
			_, err := workspace.Queries.RemoveTeamMembership(ctx, dbgen.RemoveTeamMembershipParams{
				WorkspaceID: params.WorkspaceID, TeamID: params.TeamID, MembershipID: params.MembershipID,
			})
			return err
		})
}

// ResolveLocale applies the documented precedence without silently accepting
// unsupported tags: user preference, workspace default, deployment default.
func ResolveLocale(userPreference, workspaceDefault, deploymentDefault string, supported []string) string {
	allowed := make(map[string]struct{}, len(supported))
	for _, locale := range supported {
		if normalized := normalizeLocale(locale); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	for _, candidate := range []string{userPreference, workspaceDefault, deploymentDefault} {
		normalized := normalizeLocale(candidate)
		if _, ok := allowed[normalized]; ok {
			return normalized
		}
	}
	return ""
}

func normalizeLocale(locale string) string {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	if !localePattern.MatchString(normalized) {
		return ""
	}
	return normalized
}

func validRole(role string) bool {
	switch role {
	case "owner", "admin", "manager", "sales", "viewer":
		return true
	default:
		return false
	}
}

func validInvitedRole(role string) bool {
	return role != "owner" && validRole(role)
}

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed || len(trimmed) > 254 {
		return "", errors.New("invalid email")
	}
	return strings.ToLower(trimmed), nil
}

func secureToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate secure token: %w", err)
	}
	hash := sha256.Sum256(raw[:])
	return base64.RawURLEncoding.EncodeToString(raw[:]), hash, nil
}

func parseInvitationToken(rawToken string) (ids.UUID, [32]byte, bool) {
	workspacePart, randomPart, ok := strings.Cut(rawToken, ".")
	if !ok {
		return ids.UUID{}, [32]byte{}, false
	}
	workspaceID, err := ids.Parse(workspacePart)
	if err != nil {
		return ids.UUID{}, [32]byte{}, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(randomPart)
	if err != nil || len(raw) != 32 {
		return ids.UUID{}, [32]byte{}, false
	}
	return workspaceID, sha256.Sum256(raw), true
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
