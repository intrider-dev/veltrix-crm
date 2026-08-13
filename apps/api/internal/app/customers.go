package app

import (
	"net/http"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/customers"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) listContacts(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var page customers.ContactPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			page, loadErr = application.customers.ListContacts(
				request.Context(), workspace, workspaceID,
				request.URL.Query().Get("q"), request.URL.Query().Get("status"), request.URL.Query().Get("cursor"), limit,
			)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	items := make([]apigen.Contact, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, contactFromList(row))
	}
	result := apigen.ContactPage{Items: items}
	if page.NextCursor != "" {
		result.NextCursor = &page.NextCursor
	}
	writer.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createContact(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateContact](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate, "contacts.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			row, createErr := application.customers.CreateContact(request.Context(), workspace, metadata, contactInput(body))
			if createErr != nil {
				return nil, 0, createErr
			}
			return contactFromCreate(row), row.Version, nil
		})
}

func (application *Application) getContact(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	contactID, err := parsePathID(request, "contactId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result apigen.ContactDetails
	var version int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			row, loadErr := application.customers.GetContact(request.Context(), workspace, workspaceID, contactID)
			if loadErr == nil {
				result = contactDetails(row)
				version = row.Version
			}
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) updateContact(writer http.ResponseWriter, request *http.Request) {
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
	body, _, err := httpx.DecodeJSON[apigen.UpdateContact](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result apigen.Contact
	var newVersion int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			row, updateErr := application.customers.UpdateContact(
				request.Context(), workspace, metadata(request, workspaceID, principal), contactID, version, contactInput(body),
			)
			if updateErr == nil {
				result = contactFromUpdate(row)
				newVersion = row.Version
			}
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, newVersion)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) deleteContact(writer http.ResponseWriter, request *http.Request) {
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
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.customers.DeleteContact(request.Context(), workspace, metadata(request, workspaceID, principal), contactID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) listCompanies(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var page customers.CompanyPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			page, loadErr = application.customers.ListCompaniesPage(
				request.Context(), workspace, workspaceID,
				request.URL.Query().Get("q"), request.URL.Query().Get("status"), request.URL.Query().Get("cursor"), limit,
			)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	items := make([]apigen.Company, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, companyFromPage(row))
	}
	result := apigen.CompanyPage{Items: items}
	if page.NextCursor != "" {
		result.NextCursor = &page.NextCursor
	}
	writer.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) createCompany(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[apigen.CreateCompany](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate, "companies.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			row, createErr := application.customers.CreateCompany(request.Context(), workspace, metadata, customers.CompanyInput{
				Name: body.Name, Domain: body.Domain, Industry: body.Industry, OwnerID: internalIDPointer(body.OwnerId),
			})
			if createErr != nil {
				return nil, 0, createErr
			}
			return companyFromCreate(row), row.Version, nil
		})
}

func (application *Application) getCompany(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	companyID, err := parsePathID(request, "companyId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result apigen.Company
	var version int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			row, loadErr := application.customers.GetCompany(request.Context(), workspace, workspaceID, companyID)
			if loadErr == nil {
				result = companyFromGet(row)
				version = row.Version
			}
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func contactInput(body apigen.CreateContact) customers.ContactInput {
	var email *string
	if body.Email != nil {
		value := string(*body.Email)
		email = &value
	}
	status := ""
	if body.Status != nil {
		status = *body.Status
	}
	customFields := map[string]any(nil)
	if body.CustomFields != nil {
		customFields = *body.CustomFields
	}
	return customers.ContactInput{
		FirstName: body.FirstName, LastName: body.LastName, Email: email, Phone: body.Phone,
		JobTitle: body.JobTitle, CompanyID: internalIDPointer(body.CompanyId), OwnerID: internalIDPointer(body.OwnerId),
		Status: status, Source: body.Source, CustomFields: customFields,
	}
}
