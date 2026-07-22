package app

import (
	"bufio"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type companyUpdateRequest struct {
	Name         string            `json:"name"`
	Domain       *string           `json:"domain"`
	Industry     *string           `json:"industry"`
	OwnerID      *string           `json:"ownerId"`
	TeamID       *string           `json:"teamId"`
	Status       string            `json:"status"`
	Address      map[string]string `json:"address"`
	CustomFields map[string]any    `json:"customFields"`
}

type tagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type contactTagsRequest struct {
	TagIDs []string `json:"tagIds"`
}

type customFieldsRequest struct {
	Values map[string]any `json:"values"`
}

type versionedRecordRequest struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type bulkAssignRequest struct {
	Records []versionedRecordRequest `json:"records"`
	OwnerID *string                  `json:"ownerId"`
}

type bulkTagsRequest struct {
	Records []versionedRecordRequest `json:"records"`
	TagIDs  []string                 `json:"tagIds"`
	Mode    string                   `json:"mode"`
}

type bulkDeleteRequest struct {
	Records []versionedRecordRequest `json:"records"`
}

type mergeRequest struct {
	SourceID      string `json:"sourceId"`
	SourceVersion int64  `json:"sourceVersion"`
	TargetVersion int64  `json:"targetVersion"`
}

func (application *Application) updateCompany(writer http.ResponseWriter, request *http.Request) {
	workspaceID, companyID, version, body, ok := parseCompanyMutation(application, writer, request)
	if !ok {
		return
	}
	ownerID, err := parseOptionalStringID(body.OwnerID, "/ownerId")
	if writeError(application, writer, request, err) {
		return
	}
	teamID, err := parseOptionalStringID(body.TeamID, "/teamId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result customers.CompanyRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			updated, updateErr := application.customers.UpdateCompany(request.Context(), workspace, metadata(request, workspaceID, principal), companyID, version,
				customers.CompanyUpdateInput{Name: body.Name, Domain: body.Domain, Industry: body.Industry, OwnerID: ownerID,
					TeamID: teamID, Status: body.Status, Address: body.Address, CustomFields: body.CustomFields})
			result = updated
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteCompany(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	companyID, err := parsePathID(request, "companyId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.customers.DeleteCompany(request.Context(), workspace, metadata(request, workspaceID, principal), companyID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) restoreCompany(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	companyID, err := parsePathID(request, "companyId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result customers.CompanyRecord
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			row, restoreErr := application.customers.RestoreCompany(request.Context(), workspace, metadata(request, workspaceID, principal), companyID, version)
			result = row
			return restoreErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listCompanyTrash(writer http.ResponseWriter, request *http.Request) {
	application.listCustomerTrash(writer, request, "company")
}

func (application *Application) listContactTrash(writer http.ResponseWriter, request *http.Request) {
	application.listCustomerTrash(writer, request, "contact")
}

func (application *Application) listCustomerTrash(writer http.ResponseWriter, request *http.Request, entityType string) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var page customers.DeletedPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			if entityType == "contact" {
				page, err = application.customers.ListContactTrash(request.Context(), workspace, workspaceID, request.URL.Query().Get("cursor"), limit)
			} else {
				page, err = application.customers.ListCompanyTrash(request.Context(), workspace, workspaceID, request.URL.Query().Get("cursor"), limit)
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, page)
}

func (application *Application) restoreContact(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	contactID, err := parsePathID(request, "contactId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			newVersion, err = application.customers.RestoreContact(request.Context(), workspace, metadata(request, workspaceID, principal), contactID, version)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{"id": contactID.String(), "version": newVersion})
}

func (application *Application) listTags(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var tags []customers.Tag
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			tags, err = application.customers.ListTags(request.Context(), workspace, workspaceID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, tags)
}

func (application *Application) createTag(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[tagRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsUpdate, "tags.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			tag, createErr := application.customers.CreateTag(request.Context(), workspace, metadata, customers.TagInput{Name: body.Name, Color: body.Color})
			return tag, tag.Version, createErr
		})
}

func (application *Application) updateTag(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	tagID, err := parsePathID(request, "tagId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[tagRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var tag customers.Tag
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			tag, err = application.customers.UpdateTag(request.Context(), workspace, metadata(request, workspaceID, principal), tagID, version,
				customers.TagInput{Name: body.Name, Color: body.Color})
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, tag.Version)
	httpx.WriteJSON(writer, http.StatusOK, tag)
}

func (application *Application) deleteTag(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	tagID, err := parsePathID(request, "tagId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.customers.DeleteTag(request.Context(), workspace, metadata(request, workspaceID, principal), tagID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) replaceContactTags(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	contactID, err := parsePathID(request, "contactId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[contactTagsRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	tagIDs, err := parseIDs(body.TagIDs, "/tagIds")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			newVersion, err = application.customers.ReplaceContactTags(request.Context(), workspace, metadata(request, workspaceID, principal), contactID, version, tagIDs)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{"id": contactID.String(), "tagIds": body.TagIDs, "version": newVersion})
}

func (application *Application) listCustomFieldDefinitions(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var definitions []customers.CustomFieldDefinition
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			definitions, err = application.customers.ListCustomFieldDefinitions(request.Context(), workspace, workspaceID, request.URL.Query().Get("entityType"))
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, definitions)
}

func (application *Application) createCustomFieldDefinition(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[customers.CustomFieldDefinitionInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionSettingsWrite, "custom-fields.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			definition, createErr := application.customers.CreateCustomFieldDefinition(request.Context(), workspace, metadata, body)
			return definition, definition.Version, createErr
		})
}

func (application *Application) updateCustomFieldDefinition(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	definitionID, err := parsePathID(request, "definitionId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[customers.CustomFieldDefinitionInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var definition customers.CustomFieldDefinition
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			definition, err = application.customers.UpdateCustomFieldDefinition(request.Context(), workspace, metadata(request, workspaceID, principal), definitionID, version, body)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, definition.Version)
	httpx.WriteJSON(writer, http.StatusOK, definition)
}

func (application *Application) deleteCustomFieldDefinition(writer http.ResponseWriter, request *http.Request) {
	application.deleteVersionedCustomerResource(writer, request, "definitionId", tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata, id ids.UUID, version int64) error {
			return application.customers.DeleteCustomFieldDefinition(request.Context(), workspace, metadata, id, version)
		})
}

func (application *Application) setContactCustomFields(writer http.ResponseWriter, request *http.Request) {
	application.setEntityCustomFields(writer, request, "contact", "contactId")
}

func (application *Application) setCompanyCustomFields(writer http.ResponseWriter, request *http.Request) {
	application.setEntityCustomFields(writer, request, "company", "companyId")
}

func (application *Application) setEntityCustomFields(writer http.ResponseWriter, request *http.Request, entityType, pathKey string) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	entityID, err := parsePathID(request, pathKey)
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[customFieldsRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			newVersion, err = application.customers.SetEntityCustomFields(request.Context(), workspace, metadata(request, workspaceID, principal), entityType, entityID, version, body.Values)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	httpx.WriteJSON(writer, http.StatusOK, map[string]any{"id": entityID.String(), "customFields": body.Values, "version": newVersion})
}

func (application *Application) listSavedViews(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var views []customers.SavedView
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			views, err = application.customers.ListSavedViews(request.Context(), workspace, workspaceID, principal.UserID, request.URL.Query().Get("entityType"))
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, views)
}

func (application *Application) createSavedView(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[customers.SavedViewInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsUpdate, "saved-views.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			view, createErr := application.customers.CreateSavedView(request.Context(), workspace, metadata, body)
			return view, view.Version, createErr
		})
}

func (application *Application) updateSavedView(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	viewID, err := parsePathID(request, "viewId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[customers.SavedViewInput](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var view customers.SavedView
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			view, err = application.customers.UpdateSavedView(request.Context(), workspace, metadata(request, workspaceID, principal), viewID, version, body)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, view.Version)
	httpx.WriteJSON(writer, http.StatusOK, view)
}

func (application *Application) deleteSavedView(writer http.ResponseWriter, request *http.Request) {
	application.deleteVersionedCustomerResource(writer, request, "viewId", tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata, id ids.UUID, version int64) error {
			return application.customers.DeleteSavedView(request.Context(), workspace, metadata, id, version)
		})
}

func (application *Application) bulkAssignContacts(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[bulkAssignRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	ownerID, err := parseOptionalStringID(body.OwnerID, "/ownerId")
	if writeError(application, writer, request, err) {
		return
	}
	application.runContactBulk(writer, request, body.Records, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata, records []customers.VersionedID) (customers.BulkResult, error) {
			return application.customers.BulkAssignContacts(request.Context(), workspace, metadata, records, ownerID)
		})
}

func (application *Application) bulkTagContacts(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[bulkTagsRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	tagIDs, err := parseIDs(body.TagIDs, "/tagIds")
	if writeError(application, writer, request, err) {
		return
	}
	application.runContactBulk(writer, request, body.Records, tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata, records []customers.VersionedID) (customers.BulkResult, error) {
			return application.customers.BulkTagContacts(request.Context(), workspace, metadata, records, tagIDs, body.Mode)
		})
}

func (application *Application) bulkDeleteContacts(writer http.ResponseWriter, request *http.Request) {
	body, _, err := httpx.DecodeJSON[bulkDeleteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runContactBulk(writer, request, body.Records, tenancy.PermissionRecordsDelete,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata, records []customers.VersionedID) (customers.BulkResult, error) {
			return application.customers.BulkDeleteContacts(request.Context(), workspace, metadata, records)
		})
}

func (application *Application) contactDuplicateCandidates(writer http.ResponseWriter, request *http.Request) {
	application.customerDuplicates(writer, request, "contact", "contactId")
}

func (application *Application) companyDuplicateCandidates(writer http.ResponseWriter, request *http.Request) {
	application.customerDuplicates(writer, request, "company", "companyId")
}

func (application *Application) customerDuplicates(writer http.ResponseWriter, request *http.Request, entityType, pathKey string) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	entityID, err := parsePathID(request, pathKey)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var candidates []customers.DuplicateCandidate
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			if entityType == "contact" {
				candidates, err = application.customers.ContactDuplicateCandidates(request.Context(), workspace, workspaceID, entityID)
			} else {
				candidates, err = application.customers.CompanyDuplicateCandidates(request.Context(), workspace, workspaceID, entityID)
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, candidates)
}

func (application *Application) mergeContacts(writer http.ResponseWriter, request *http.Request) {
	application.mergeCustomerRecords(writer, request, "contact", "contactId")
}

func (application *Application) mergeCompanies(writer http.ResponseWriter, request *http.Request) {
	application.mergeCustomerRecords(writer, request, "company", "companyId")
}

func (application *Application) mergeCustomerRecords(writer http.ResponseWriter, request *http.Request, entityType, targetPathKey string) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	targetID, err := parsePathID(request, targetPathKey)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[mergeRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	sourceID, err := ids.Parse(body.SourceID)
	if err != nil {
		err = &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/sourceId", Code: "validation.uuid.invalid"}}}
	}
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result customers.MergeResult
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			input := customers.MergeInput{SourceID: sourceID, SourceVersion: body.SourceVersion, TargetID: targetID, TargetVersion: body.TargetVersion}
			if entityType == "contact" {
				result, err = application.customers.MergeContacts(request.Context(), workspace, metadata(request, workspaceID, principal), input)
			} else {
				result, err = application.customers.MergeCompanies(request.Context(), workspace, metadata(request, workspaceID, principal), input)
			}
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.TargetVersion)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) exportContactsCSV(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	ownerID, err := parseOptionalID(request.URL.Query().Get("ownerId"), "/query/ownerId")
	if writeError(application, writer, request, err) {
		return
	}
	tagIDs, err := parseIDs(splitCommaValues(request.URL.Query()["tagId"]), "/query/tagId")
	if writeError(application, writer, request, err) {
		return
	}
	filter := customers.ContactExportFilter{Query: request.URL.Query().Get("q"), Status: request.URL.Query().Get("status"),
		OwnerID: ownerID, TagIDs: tagIDs, Sort: request.URL.Query().Get("sort"), Order: request.URL.Query().Get("order")}
	if writeError(application, writer, request, customers.ValidateContactExportFilter(filter)) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	buffer := bufio.NewWriterSize(writer, 64<<10)
	started := false
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionDataExport,
		func(workspace *tenancy.WorkspaceTx) error {
			writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
			writer.Header().Set("Content-Disposition", `attachment; filename="contacts.csv"`)
			writer.Header().Set("Cache-Control", "private, no-store")
			started = true
			return application.customers.ExportContactsCSV(request.Context(), workspace, workspaceID, filter, buffer)
		})
	if err == nil {
		err = buffer.Flush()
	}
	if err != nil {
		if !started {
			writeError(application, writer, request, err)
			return
		}
		application.logger.Error("contact CSV export failed", "request_id", httpx.RequestID(request.Context()), "error", err)
	}
}

func (application *Application) previewContactImport(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, customers.MaxCSVBytes+(1<<20))
	mediaType, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || parameters["boundary"] == "" {
		writeError(application, writer, request, multipartFileValidation())
		return
	}
	reader, err := request.MultipartReader()
	if writeError(application, writer, request, err) {
		return
	}
	var fileFound bool
	for {
		part, nextErr := reader.NextPart()
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				break
			}
			writeError(application, writer, request, nextErr)
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		if fileFound {
			_ = part.Close()
			writeError(application, writer, request, multipartFileValidation())
			return
		}
		fileFound = true
		principal, _ := httpx.Principal(request.Context())
		var preview customers.ImportPreview
		err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsCreate,
			func(workspace *tenancy.WorkspaceTx) error {
				preview, err = application.customers.StageContactCSV(request.Context(), workspace, metadata(request, workspaceID, principal), part)
				return err
			})
		_ = part.Close()
		if writeError(application, writer, request, err) {
			return
		}
		httpx.WriteJSON(writer, http.StatusCreated, preview)
		return
	}
	if !fileFound {
		writeError(application, writer, request, multipartFileValidation())
	}
}

func (application *Application) queueContactImport(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	sessionID, err := parsePathID(request, "importId")
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[customers.ContactImportMapping](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var status customers.ImportStatus
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsCreate,
		func(workspace *tenancy.WorkspaceTx) error {
			status, err = application.customers.QueueContactImport(request.Context(), workspace, metadata(request, workspaceID, principal), sessionID, body)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusAccepted, status)
}

func (application *Application) getContactImport(writer http.ResponseWriter, request *http.Request) {
	application.contactImportResponse(writer, request, false)
}

func (application *Application) downloadContactImportErrors(writer http.ResponseWriter, request *http.Request) {
	application.contactImportResponse(writer, request, true)
}

func (application *Application) contactImportResponse(writer http.ResponseWriter, request *http.Request, errorsCSV bool) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	sessionID, err := parsePathID(request, "importId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	if errorsCSV {
		buffer := bufio.NewWriterSize(writer, 32<<10)
		started := false
		err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
			func(workspace *tenancy.WorkspaceTx) error {
				if _, statusErr := application.customers.GetImportStatus(request.Context(), workspace, workspaceID, principal.UserID, sessionID); statusErr != nil {
					return statusErr
				}
				writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
				writer.Header().Set("Content-Disposition", `attachment; filename="contact-import-errors.csv"`)
				writer.Header().Set("Cache-Control", "private, no-store")
				started = true
				return application.customers.WriteImportErrorsCSV(request.Context(), workspace, workspaceID, principal.UserID, sessionID, buffer)
			})
		if err == nil {
			err = buffer.Flush()
		}
		if err != nil {
			if !started {
				writeError(application, writer, request, err)
				return
			}
			application.logger.Error("contact import error export failed", "request_id", httpx.RequestID(request.Context()), "error", err)
		}
		return
	}
	var status customers.ImportStatus
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			status, err = application.customers.GetImportStatus(request.Context(), workspace, workspaceID, principal.UserID, sessionID)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, status)
}

func parseCompanyMutation(application *Application, writer http.ResponseWriter, request *http.Request) (ids.UUID, ids.UUID, int64, companyUpdateRequest, bool) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, 0, companyUpdateRequest{}, false
	}
	companyID, err := parsePathID(request, "companyId")
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, 0, companyUpdateRequest{}, false
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, 0, companyUpdateRequest{}, false
	}
	body, _, err := httpx.DecodeJSON[companyUpdateRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, 0, companyUpdateRequest{}, false
	}
	return workspaceID, companyID, version, body, true
}

func (application *Application) deleteVersionedCustomerResource(
	writer http.ResponseWriter,
	request *http.Request,
	pathKey string,
	permission tenancy.Permission,
	mutation func(*tenancy.WorkspaceTx, events.Metadata, ids.UUID, int64) error,
) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	id, err := parsePathID(request, pathKey)
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), permission,
		func(workspace *tenancy.WorkspaceTx) error {
			return mutation(workspace, metadata(request, workspaceID, principal), id, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) runContactBulk(
	writer http.ResponseWriter,
	request *http.Request,
	recordRequests []versionedRecordRequest,
	permission tenancy.Permission,
	mutation func(*tenancy.WorkspaceTx, events.Metadata, []customers.VersionedID) (customers.BulkResult, error),
) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	records := make([]customers.VersionedID, 0, len(recordRequests))
	for _, record := range recordRequests {
		id, parseErr := ids.Parse(record.ID)
		if parseErr != nil {
			writeError(application, writer, request, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/records/id", Code: "validation.uuid.invalid"}}})
			return
		}
		records = append(records, customers.VersionedID{ID: id, Version: record.Version})
	}
	principal, _ := httpx.Principal(request.Context())
	var result customers.BulkResult
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), permission,
		func(workspace *tenancy.WorkspaceTx) error {
			result, err = mutation(workspace, metadata(request, workspaceID, principal), records)
			return err
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func parseOptionalStringID(value *string, pointer string) (*ids.UUID, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	return parseOptionalID(*value, pointer)
}

func parseIDs(values []string, pointer string) ([]ids.UUID, error) {
	result := make([]ids.UUID, 0, len(values))
	for _, value := range values {
		id, err := ids.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: "validation.uuid.invalid"}}}
		}
		result = append(result, id)
	}
	return result, nil
}

func splitCommaValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func multipartFileValidation() error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/file", Code: "validation.content_type.multipart_required"}}}
}
