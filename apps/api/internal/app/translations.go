package app

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type putTranslationRequest struct {
	SourceLocale   string `json:"sourceLocale"`
	SourceText     string `json:"sourceText"`
	Description    string `json:"description"`
	TranslatedText string `json:"translatedText"`
	Status         string `json:"status"`
	Version        int64  `json:"version"`
}

type updateLocaleSettingsRequest struct {
	DefaultLocale    string   `json:"defaultLocale"`
	SupportedLocales []string `json:"supportedLocales"`
}

func (application *Application) listTranslations(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result localization.ContentTranslationPage
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.translations.List(
				request.Context(), workspace, workspaceID,
				request.URL.Query().Get("locale"), request.URL.Query().Get("namespace"),
				request.URL.Query().Get("status"), request.URL.Query().Get("q"),
				request.URL.Query().Get("cursor"), limit,
			)
			return loadErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) translationCoverage(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []localization.TranslationCoverage
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.translations.Coverage(
				request.Context(), workspace, workspaceID, request.URL.Query().Get("locale"),
			)
			return loadErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) putTranslation(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	locale := strings.TrimSpace(chi.URLParam(request, "locale"))
	namespace := strings.TrimSpace(chi.URLParam(request, "namespace"))
	resourceKey := strings.TrimSpace(chi.URLParam(request, "translationKey"))
	body, _, err := httpx.DecodeJSON[putTranslationRequest](writer, request, 32<<10)
	if writeError(application, writer, request, err) {
		return
	}
	if body.Version > 0 {
		headerVersion, parseErr := parseETag(request)
		if writeError(application, writer, request, parseErr) {
			return
		}
		if headerVersion != body.Version {
			httpx.WriteProblem(writer, request, application.logger, errx.ErrVersionConflict)
			return
		}
	} else if match := strings.TrimSpace(request.Header.Get("If-Match")); match != "" {
		httpx.WriteProblem(writer, request, application.logger, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/headers/If-Match", Code: "validation.etag.unexpected",
		}}})
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result localization.ContentTranslation
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = application.translations.Put(
				request.Context(), workspace, metadata(request, workspaceID, principal),
				locale, namespace, resourceKey,
				localization.ContentTranslationInput{
					SourceLocale: body.SourceLocale, SourceText: body.SourceText,
					Description: body.Description, TranslatedText: body.TranslatedText,
					Status: body.Status, Version: body.Version,
				},
			)
			return updateErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.TranslationVersion)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) localizationSettings(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result localization.WorkspaceLocaleSettings
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.translations.Settings(request.Context(), workspace, workspaceID)
			return loadErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	writer.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) updateLocalizationSettings(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[updateLocaleSettingsRequest](writer, request, 16<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result localization.WorkspaceLocaleSettings
	err = application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionSettingsWrite,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = application.translations.UpdateSettings(
				request.Context(), workspace, metadata(request, workspaceID, principal),
				body.DefaultLocale, body.SupportedLocales, version,
			)
			return updateErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}
