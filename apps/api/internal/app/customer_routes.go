package app

import "github.com/go-chi/chi/v5"

func (application *Application) registerAdvancedCustomerRoutes(workspace chi.Router) {
	workspace.Patch("/companies/{companyId}", application.updateCompany)
	workspace.Delete("/companies/{companyId}", application.deleteCompany)
	workspace.Post("/companies/{companyId}/restore", application.restoreCompany)
	workspace.Get("/companies/trash", application.listCompanyTrash)
	workspace.Get("/contacts/trash", application.listContactTrash)
	workspace.Post("/contacts/{contactId}/restore", application.restoreContact)

	workspace.Get("/tags", application.listTags)
	workspace.Post("/tags", application.createTag)
	workspace.Patch("/tags/{tagId}", application.updateTag)
	workspace.Delete("/tags/{tagId}", application.deleteTag)
	workspace.Put("/contacts/{contactId}/tags", application.replaceContactTags)

	workspace.Get("/custom-fields", application.listCustomFieldDefinitions)
	workspace.Get("/reference-users", application.listReferenceUsers)
	workspace.Post("/custom-fields", application.createCustomFieldDefinition)
	workspace.Patch("/custom-fields/{definitionId}", application.updateCustomFieldDefinition)
	workspace.Delete("/custom-fields/{definitionId}", application.deleteCustomFieldDefinition)
	workspace.Put("/contacts/{contactId}/custom-fields", application.setContactCustomFields)
	workspace.Put("/companies/{companyId}/custom-fields", application.setCompanyCustomFields)

	workspace.Get("/saved-views", application.listSavedViews)
	workspace.Post("/saved-views", application.createSavedView)
	workspace.Patch("/saved-views/{viewId}", application.updateSavedView)
	workspace.Delete("/saved-views/{viewId}", application.deleteSavedView)

	workspace.Post("/contacts/bulk/assign", application.bulkAssignContacts)
	workspace.Post("/contacts/bulk/tags", application.bulkTagContacts)
	workspace.Post("/contacts/bulk/delete", application.bulkDeleteContacts)
	workspace.Get("/contacts/{contactId}/duplicates", application.contactDuplicateCandidates)
	workspace.Post("/contacts/{contactId}/merge", application.mergeContacts)
	workspace.Get("/companies/{companyId}/duplicates", application.companyDuplicateCandidates)
	workspace.Post("/companies/{companyId}/merge", application.mergeCompanies)

	workspace.Get("/contacts/export", application.exportContactsCSV)
	workspace.Post("/contacts/imports/preview", application.previewContactImport)
	workspace.Post("/contacts/imports/{importId}/queue", application.queueContactImport)
	workspace.Get("/contacts/imports/{importId}", application.getContactImport)
	workspace.Get("/contacts/imports/{importId}/errors", application.downloadContactImportErrors)
}

func (application *Application) registerAttachmentRoutes(workspace chi.Router) {
	workspace.Get("/attachments", application.listAttachments)
	workspace.Post("/attachments", application.uploadAttachment)
	workspace.Get("/attachments/{attachmentId}", application.downloadAttachment)
	workspace.Delete("/attachments/{attachmentId}", application.deleteAttachment)
}
