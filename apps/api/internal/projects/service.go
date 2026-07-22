package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/pagination"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const defaultPageSize = 25

type Input struct {
	Name             string
	Description      string
	Status           string
	Visibility       string
	PlannedStartDate *time.Time
	TargetEndDate    *time.Time
	OwnerUserID      *ids.UUID
}

type Capabilities struct {
	CanView    bool `json:"canView"`
	CanComment bool `json:"canComment"`
	CanEdit    bool `json:"canEdit"`
	CanManage  bool `json:"canManage"`
}

type Record struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      string       `json:"description"`
	Status           string       `json:"status"`
	Visibility       string       `json:"visibility"`
	PlannedStartDate *string      `json:"plannedStartDate,omitempty"`
	TargetEndDate    *string      `json:"targetEndDate,omitempty"`
	OwnerUserID      *string      `json:"ownerUserId,omitempty"`
	Capabilities     Capabilities `json:"capabilities"`
	Version          int64        `json:"version"`
	CreatedAt        time.Time    `json:"createdAt"`
	UpdatedAt        time.Time    `json:"updatedAt"`
}

type Page struct {
	Items      []Record `json:"items"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

type AssignmentInput struct {
	Kind        string
	SubjectType string
	SubjectID   ids.UUID
}

type Assignment struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	SubjectType string    `json:"subjectType"`
	SubjectID   string    `json:"subjectId"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AssignmentSet struct {
	Items   []Assignment `json:"items"`
	Version int64        `json:"version"`
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (service *Service) List(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	status, cursor string,
	limit int,
) (Page, error) {
	status = strings.TrimSpace(status)
	if status != "" && !validStatus(status) {
		return Page{}, validation("/query/status", "validation.enum")
	}
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > 100 {
		limit = 100
	}
	fingerprint := "status=" + status
	cursorTime, cursorID, err := pagination.Decode(cursor, fingerprint)
	if err != nil {
		return Page{}, validation("/query/cursor", "validation.cursor.invalid")
	}
	rows, err := workspace.Queries.ListVisibleProjects(ctx, dbgen.ListVisibleProjectsParams{
		ActorRole: workspace.Membership.Role, ActorUserID: workspace.Membership.UserID,
		MembershipID: workspace.Membership.ID, WorkspaceID: workspaceID.PG(), StatusFilter: status,
		CursorUpdatedAt: pgtype.Timestamptz{Time: cursorTime, Valid: true}, CursorID: cursorID.PG(),
		PageLimit: int32(limit + 1),
	})
	if err != nil {
		return Page{}, fmt.Errorf("list projects: %w", err)
	}
	page := Page{Items: make([]Record, 0, min(len(rows), limit))}
	for index, row := range rows {
		if index == limit {
			break
		}
		page.Items = append(page.Items, record(row.ID, row.Name, row.Description, row.Status,
			row.Visibility, row.PlannedStartDate, row.TargetEndDate, row.OwnerUserID,
			boolValue(row.CanEdit), boolValue(row.CanManage), row.Version, row.CreatedAt.Time, row.UpdatedAt.Time))
	}
	if len(rows) > limit {
		last := rows[limit-1]
		lastID, _ := ids.FromPG(last.ID)
		page.NextCursor, err = pagination.Encode(last.UpdatedAt.Time, lastID, fingerprint)
		if err != nil {
			return Page{}, fmt.Errorf("encode project cursor: %w", err)
		}
	}
	return page, nil
}

func (service *Service) Get(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, projectID ids.UUID,
) (Record, error) {
	row, err := workspace.Queries.GetVisibleProject(ctx, dbgen.GetVisibleProjectParams{
		ActorRole: workspace.Membership.Role, ActorUserID: workspace.Membership.UserID,
		MembershipID: workspace.Membership.ID, WorkspaceID: workspaceID.PG(), ProjectID: projectID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, errx.ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("get project: %w", err)
	}
	return record(row.ID, row.Name, row.Description, row.Status, row.Visibility,
		row.PlannedStartDate, row.TargetEndDate, row.OwnerUserID, boolValue(row.CanEdit),
		boolValue(row.CanManage), row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) Exists(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, projectID ids.UUID,
) (bool, error) {
	_, err := service.Get(ctx, workspace, workspaceID, projectID)
	if errors.Is(err, errx.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (service *Service) Create(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input Input,
) (Record, error) {
	validated, err := validate(input)
	if err != nil {
		return Record{}, err
	}
	projectID, err := ids.NewV7()
	if err != nil {
		return Record{}, err
	}
	if validated.OwnerUserID == nil {
		actor := metadata.ActorID
		validated.OwnerUserID = &actor
	}
	row, err := workspace.Queries.CreateProject(ctx, dbgen.CreateProjectParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: projectID.PG(), Name: validated.Name,
		Description: validated.Description, Status: validated.Status, Visibility: validated.Visibility,
		PlannedStartDate: optionalDate(validated.PlannedStartDate), TargetEndDate: optionalDate(validated.TargetEndDate),
		OwnerUserID: optionalUUID(validated.OwnerUserID), CreatedBy: metadata.ActorID.PG(),
	})
	if err != nil {
		return Record{}, mapConstraint(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "project.created", EventType: "projects.project.created", AggregateType: "project", AggregateID: projectID,
		Summary: map[string]any{"fields": []string{"name", "status", "visibility", "plannedStartDate", "targetEndDate", "ownerUserId"}},
		Payload: map[string]any{"projectId": projectID.String(), "version": row.Version},
	}); err != nil {
		return Record{}, err
	}
	return record(row.ID, row.Name, row.Description, row.Status, row.Visibility,
		row.PlannedStartDate, row.TargetEndDate, row.OwnerUserID, true, true,
		row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) Update(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	projectID ids.UUID,
	version int64,
	input Input,
) (Record, error) {
	existing, err := service.Get(ctx, workspace, metadata.WorkspaceID, projectID)
	if err != nil {
		return Record{}, err
	}
	if !existing.Capabilities.CanEdit {
		return Record{}, errx.ErrForbidden
	}
	if existing.Version != version {
		return Record{}, errx.ErrVersionConflict
	}
	validated, err := validate(input)
	if err != nil {
		return Record{}, err
	}
	row, err := workspace.Queries.UpdateProject(ctx, dbgen.UpdateProjectParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: projectID.PG(), Name: validated.Name,
		Description: validated.Description, Status: validated.Status, Visibility: validated.Visibility,
		PlannedStartDate: optionalDate(validated.PlannedStartDate), TargetEndDate: optionalDate(validated.TargetEndDate),
		OwnerUserID: optionalUUID(validated.OwnerUserID), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, errx.ErrVersionConflict
	}
	if err != nil {
		return Record{}, mapConstraint(err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "project.updated", EventType: "projects.project.updated", AggregateType: "project", AggregateID: projectID,
		Summary: map[string]any{"fields": []string{"name", "description", "status", "visibility", "plannedStartDate", "targetEndDate", "ownerUserId"}},
		Payload: map[string]any{"projectId": projectID.String(), "version": row.Version},
	}); err != nil {
		return Record{}, err
	}
	return record(row.ID, row.Name, row.Description, row.Status, row.Visibility,
		row.PlannedStartDate, row.TargetEndDate, row.OwnerUserID, existing.Capabilities.CanEdit,
		existing.Capabilities.CanManage, row.Version, row.CreatedAt.Time, row.UpdatedAt.Time), nil
}

func (service *Service) Delete(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	projectID ids.UUID,
	version int64,
) error {
	existing, err := service.Get(ctx, workspace, metadata.WorkspaceID, projectID)
	if err != nil {
		return err
	}
	if !existing.Capabilities.CanManage {
		return errx.ErrForbidden
	}
	if existing.Version != version {
		return errx.ErrVersionConflict
	}
	changed, err := workspace.Queries.SoftDeleteProject(ctx, dbgen.SoftDeleteProjectParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: projectID.PG(), Version: version,
	})
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if changed == 0 {
		return errx.ErrVersionConflict
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "project.deleted", EventType: "projects.project.deleted", AggregateType: "project", AggregateID: projectID,
		Summary: map[string]any{"deleted": true}, Payload: map[string]any{"projectId": projectID.String(), "version": version + 1},
	})
}

func (service *Service) ListAssignments(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, projectID ids.UUID,
) (AssignmentSet, error) {
	project, err := service.Get(ctx, workspace, workspaceID, projectID)
	if err != nil {
		return AssignmentSet{}, err
	}
	rows, err := workspace.Queries.ListProjectAssignments(ctx, dbgen.ListProjectAssignmentsParams{
		WorkspaceID: workspaceID.PG(), ProjectID: projectID.PG(),
	})
	if err != nil {
		return AssignmentSet{}, fmt.Errorf("list project assignments: %w", err)
	}
	result := make([]Assignment, 0, len(rows))
	for _, row := range rows {
		assignmentID, _ := ids.FromPG(row.ID)
		subjectType := "user"
		subjectID, _ := ids.FromPG(row.UserID)
		displayName := stringValue(row.UserName)
		if !row.UserID.Valid {
			subjectType = "department"
			subjectID, _ = ids.FromPG(row.DepartmentID)
			displayName = stringValue(row.DepartmentName)
		}
		result = append(result, Assignment{ID: assignmentID.String(), Kind: row.AssignmentKind,
			SubjectType: subjectType, SubjectID: subjectID.String(), DisplayName: displayName,
			CreatedAt: row.CreatedAt.Time.UTC()})
	}
	return AssignmentSet{Items: result, Version: project.Version}, nil
}

func (service *Service) ReplaceAssignments(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	projectID ids.UUID,
	version int64,
	items []AssignmentInput,
) (AssignmentSet, error) {
	project, err := service.Get(ctx, workspace, metadata.WorkspaceID, projectID)
	if err != nil {
		return AssignmentSet{}, err
	}
	if !project.Capabilities.CanManage {
		return AssignmentSet{}, errx.ErrForbidden
	}
	if project.Version != version {
		return AssignmentSet{}, errx.ErrVersionConflict
	}
	if len(items) > 100 {
		return AssignmentSet{}, validation("/assignments", "validation.items.range")
	}
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		if item.Kind != "responsible" && item.Kind != "watcher" {
			return AssignmentSet{}, validation(fmt.Sprintf("/assignments/%d/kind", index), "validation.enum")
		}
		if item.SubjectType != "user" && item.SubjectType != "department" {
			return AssignmentSet{}, validation(fmt.Sprintf("/assignments/%d/subjectType", index), "validation.enum")
		}
		key := item.Kind + ":" + item.SubjectType + ":" + item.SubjectID.String()
		if _, duplicate := seen[key]; duplicate {
			return AssignmentSet{}, validation(fmt.Sprintf("/assignments/%d", index), "validation.duplicate")
		}
		seen[key] = struct{}{}
	}
	if err := workspace.Queries.DeleteProjectAssignments(ctx, dbgen.DeleteProjectAssignmentsParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ProjectID: projectID.PG(),
	}); err != nil {
		return AssignmentSet{}, fmt.Errorf("replace project assignments: %w", err)
	}
	for _, item := range items {
		assignmentID, idErr := ids.NewV7()
		if idErr != nil {
			return AssignmentSet{}, idErr
		}
		userID, departmentID := pgtype.UUID{}, pgtype.UUID{}
		if item.SubjectType == "user" {
			userID = item.SubjectID.PG()
		} else {
			departmentID = item.SubjectID.PG()
		}
		if _, createErr := workspace.Queries.CreateProjectAssignment(ctx, dbgen.CreateProjectAssignmentParams{
			WorkspaceID: metadata.WorkspaceID.PG(), ID: assignmentID.PG(), ProjectID: projectID.PG(),
			AssignmentKind: item.Kind, UserID: userID, DepartmentID: departmentID, CreatedBy: metadata.ActorID.PG(),
		}); createErr != nil {
			return AssignmentSet{}, mapConstraint(createErr)
		}
	}
	newVersion, err := workspace.Queries.BumpProjectAssignmentsVersion(ctx, dbgen.BumpProjectAssignmentsVersionParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: projectID.PG(), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return AssignmentSet{}, errx.ErrVersionConflict
	}
	if err != nil {
		return AssignmentSet{}, fmt.Errorf("bump project assignment version: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "project.assignments_replaced", EventType: "projects.assignments.replaced", AggregateType: "project", AggregateID: projectID,
		Summary: map[string]any{"assignmentCount": len(items)}, Payload: map[string]any{"projectId": projectID.String(), "version": newVersion},
	}); err != nil {
		return AssignmentSet{}, err
	}
	result, err := service.ListAssignments(ctx, workspace, metadata.WorkspaceID, projectID)
	result.Version = newVersion
	return result, err
}

func validate(input Input) (Input, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.TrimSpace(input.Status)
	input.Visibility = strings.TrimSpace(input.Visibility)
	fields := make([]errx.FieldError, 0, 4)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 200 {
		fields = append(fields, errx.FieldError{Pointer: "/name", Code: "validation.length"})
	}
	if utf8.RuneCountInString(input.Description) > 20000 {
		fields = append(fields, errx.FieldError{Pointer: "/description", Code: "validation.length"})
	}
	if !validStatus(input.Status) {
		fields = append(fields, errx.FieldError{Pointer: "/status", Code: "validation.enum"})
	}
	if input.Visibility != "workspace" && input.Visibility != "restricted" {
		fields = append(fields, errx.FieldError{Pointer: "/visibility", Code: "validation.enum"})
	}
	if input.PlannedStartDate != nil && input.TargetEndDate != nil && input.PlannedStartDate.After(*input.TargetEndDate) {
		fields = append(fields, errx.FieldError{Pointer: "/plannedStartDate", Code: "validation.date.range"})
	}
	if len(fields) > 0 {
		return Input{}, &errx.ValidationError{Fields: fields}
	}
	return input, nil
}

func validStatus(status string) bool {
	switch status {
	case "planned", "active", "on_hold", "completed", "archived":
		return true
	default:
		return false
	}
}

func record(
	id pgtype.UUID,
	name, description, status, visibility string,
	plannedStartDate, targetEndDate pgtype.Date,
	ownerUserID pgtype.UUID,
	canEdit, canManage bool,
	version int64,
	createdAt, updatedAt time.Time,
) Record {
	projectID, _ := ids.FromPG(id)
	return Record{ID: projectID.String(), Name: name, Description: description, Status: status,
		Visibility: visibility, PlannedStartDate: optionalDateString(plannedStartDate),
		TargetEndDate: optionalDateString(targetEndDate), OwnerUserID: optionalIDString(ownerUserID),
		Capabilities: Capabilities{CanView: true, CanComment: true, CanEdit: canEdit, CanManage: canManage},
		Version:      version, CreatedAt: createdAt.UTC(), UpdatedAt: updatedAt.UTC()}
}

func boolValue(value *bool) bool { return value != nil && *value }

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalUUID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func optionalIDString(value pgtype.UUID) *string {
	id, ok := ids.FromPG(value)
	if !ok {
		return nil
	}
	result := id.String()
	return &result
}

func optionalDate(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: value.UTC(), Valid: true}
}

func optionalDateString(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC().Format("2006-01-02")
	return &result
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}

func mapConstraint(err error) error {
	var pgError interface{ SQLState() string }
	if errors.As(err, &pgError) {
		switch pgError.SQLState() {
		case "23503":
			return validation("/subjectId", "validation.reference.invalid")
		case "23505":
			return errx.ErrConflict
		case "23514":
			return validation("/record", "validation.constraint")
		}
	}
	return fmt.Errorf("project constraint: %w", err)
}
