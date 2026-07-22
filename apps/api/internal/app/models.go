package app

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func apiID(value pgtype.UUID) uuid.UUID { return uuid.UUID(value.Bytes) }

func apiIDPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := apiID(value)
	return &id
}

func internalIDPointer(value *uuid.UUID) *ids.UUID {
	if value == nil {
		return nil
	}
	id := ids.UUID(*value)
	return &id
}

func apiEmail(value *string) *openapi_types.Email {
	if value == nil {
		return nil
	}
	email := openapi_types.Email(*value)
	return &email
}

func apiTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

func apiDate(value pgtype.Date) *openapi_types.Date {
	if !value.Valid {
		return nil
	}
	date := openapi_types.Date{Time: value.Time.UTC()}
	return &date
}

func jsonObject(value []byte) *map[string]interface{} {
	if len(value) == 0 {
		return nil
	}
	object := map[string]interface{}{}
	if err := json.Unmarshal(value, &object); err != nil {
		return nil
	}
	return &object
}

func jsonStringObject(value []byte) *map[string]string {
	if len(value) == 0 {
		return nil
	}
	object := map[string]string{}
	if err := json.Unmarshal(value, &object); err != nil {
		return nil
	}
	return &object
}

func contactFromList(row dbgen.ListContactsRow) apigen.Contact {
	return apigen.Contact{
		Id: apiID(row.ID), FirstName: row.FirstName, LastName: row.LastName,
		DisplayName: row.DisplayName, Email: apiEmail(row.Email), Phone: row.Phone,
		CompanyId: apiIDPointer(row.CompanyID), CompanyName: row.CompanyName,
		OwnerId: apiIDPointer(row.OwnerUserID), Status: row.Status, Source: row.Source,
		CustomFields: jsonObject(row.CustomFields), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func contactFromCreate(row dbgen.CreateContactRow) apigen.Contact {
	return apigen.Contact{
		Id: apiID(row.ID), FirstName: row.FirstName, LastName: row.LastName,
		DisplayName: row.DisplayName, Email: apiEmail(row.Email), Phone: row.Phone,
		CompanyId: apiIDPointer(row.CompanyID), OwnerId: apiIDPointer(row.OwnerUserID),
		Status: row.Status, Source: row.Source, CustomFields: jsonObject(row.CustomFields),
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func contactFromUpdate(row dbgen.UpdateContactRow) apigen.Contact {
	return apigen.Contact{
		Id: apiID(row.ID), FirstName: row.FirstName, LastName: row.LastName,
		DisplayName: row.DisplayName, Email: apiEmail(row.Email), Phone: row.Phone,
		CompanyId: apiIDPointer(row.CompanyID), OwnerId: apiIDPointer(row.OwnerUserID),
		Status: row.Status, Source: row.Source, CustomFields: jsonObject(row.CustomFields),
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func contactDetails(row dbgen.GetContactRow) apigen.ContactDetails {
	return apigen.ContactDetails{
		Id: apiID(row.ID), FirstName: row.FirstName, LastName: row.LastName,
		DisplayName: row.DisplayName, Email: apiEmail(row.Email), Phone: row.Phone,
		JobTitle: row.JobTitle, CompanyId: apiIDPointer(row.CompanyID), CompanyName: row.CompanyName,
		OwnerId: apiIDPointer(row.OwnerUserID), Status: row.Status, Source: row.Source,
		Address: jsonObject(row.Address), CustomFields: jsonObject(row.CustomFields),
		LastContactedAt: apiTime(row.LastContactedAt), NextActivityAt: apiTime(row.NextActivityAt),
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func companyFromList(row dbgen.ListCompaniesRow) apigen.Company {
	return apigen.Company{Id: apiID(row.ID), Name: row.Name, Domain: row.Domain, Industry: row.Industry,
		Status: row.Status, OwnerId: apiIDPointer(row.OwnerUserID), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

func companyFromPage(row dbgen.ListCompaniesPageRow) apigen.Company {
	return apigen.Company{Id: apiID(row.ID), Name: row.Name, Domain: row.Domain, Industry: row.Industry,
		Status: row.Status, OwnerId: apiIDPointer(row.OwnerUserID), TeamId: apiIDPointer(row.TeamID),
		Address: jsonStringObject(row.Address), CustomFields: jsonObject(row.CustomFields), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

func companyFromCreate(row dbgen.CreateCompanyRow) apigen.Company {
	return apigen.Company{Id: apiID(row.ID), Name: row.Name, Domain: row.Domain, Industry: row.Industry,
		Status: row.Status, OwnerId: apiIDPointer(row.OwnerUserID), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

func companyFromGet(row dbgen.GetCompanyRow) apigen.Company {
	return apigen.Company{Id: apiID(row.ID), Name: row.Name, Domain: row.Domain, Industry: row.Industry,
		Status: row.Status, OwnerId: apiIDPointer(row.OwnerUserID), TeamId: apiIDPointer(row.TeamID),
		Address: jsonStringObject(row.Address), CustomFields: jsonObject(row.CustomFields), Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}
}

func dealModel(id, pipelineID, stageID pgtype.UUID, name string, contactID, companyID, ownerID pgtype.UUID,
	amountMinor int64, currency string, plannedStartDate, closeDate pgtype.Date, position int32, status string,
	version int64, updatedAt pgtype.Timestamptz,
) apigen.Deal {
	return apigen.Deal{
		Id: apiID(id), PipelineId: apiID(pipelineID), StageId: apiID(stageID), Name: name,
		ContactId: apiIDPointer(contactID), CompanyId: apiIDPointer(companyID), OwnerId: apiIDPointer(ownerID),
		AmountMinor: amountMinor, Currency: currency, PlannedStartDate: apiDate(plannedStartDate), ExpectedCloseDate: apiDate(closeDate),
		Position: int(position), Status: apigen.DealStatus(status), Version: version, UpdatedAt: updatedAt.Time.UTC(),
	}
}

func dealFromList(row dbgen.ListDealsRow) apigen.Deal {
	return dealModel(row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID, row.OwnerUserID,
		row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position, row.Status, row.Version, row.UpdatedAt)
}

func dealFromCreate(row dbgen.CreateDealRow) apigen.Deal {
	return dealModel(row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID, row.OwnerUserID,
		row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position, row.Status, row.Version, row.UpdatedAt)
}

func dealFromMove(row dbgen.MoveDealRow) apigen.Deal {
	return dealModel(row.ID, row.PipelineID, row.StageID, row.Name, row.ContactID, row.CompanyID, row.OwnerUserID,
		row.AmountMinor, row.Currency, row.PlannedStartDate, row.ExpectedCloseDate, row.Position, row.Status, row.Version, row.UpdatedAt)
}

func activityModel(id pgtype.UUID, activityType, title string, body, relatedType *string,
	relatedID, assigneeID pgtype.UUID, status, priority string, dueAt, occurredAt pgtype.Timestamptz, version int64,
) apigen.Activity {
	var apiRelatedType *apigen.ActivityRelatedType
	if relatedType != nil {
		value := apigen.ActivityRelatedType(*relatedType)
		apiRelatedType = &value
	}
	apiPriority := apigen.ActivityPriority(priority)
	return apigen.Activity{
		Id: apiID(id), Type: apigen.ActivityType(activityType), Title: title, Body: body,
		RelatedType: apiRelatedType, RelatedId: apiIDPointer(relatedID), AssigneeId: apiIDPointer(assigneeID),
		Status: apigen.ActivityStatus(status), Priority: &apiPriority, DueAt: apiTime(dueAt),
		OccurredAt: occurredAt.Time.UTC(), Version: version,
	}
}

func activityFromList(row dbgen.ListActivitiesRow) apigen.Activity {
	return activityModel(row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType, row.RelatedID,
		row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt, row.Version)
}

func activityFromCreate(row dbgen.CreateActivityRow) apigen.Activity {
	return activityModel(row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType, row.RelatedID,
		row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt, row.Version)
}

func activityFromComplete(row dbgen.CompleteActivityRow) apigen.Activity {
	return activityModel(row.ID, row.ActivityType, row.Title, row.Body, row.RelatedType, row.RelatedID,
		row.AssigneeUserID, row.Status, row.Priority, row.DueAt, row.OccurredAt, row.Version)
}
