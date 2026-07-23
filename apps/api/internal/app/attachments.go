package app

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/files"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const multipartEnvelopeAllowance = 1 << 20

type attachmentResponse struct {
	ID          string `json:"id"`
	EntityType  string `json:"entityType"`
	EntityID    string `json:"entityId"`
	DisplayName string `json:"displayName"`
	MediaType   string `json:"mediaType"`
	SizeBytes   int64  `json:"sizeBytes"`
	ScanState   string `json:"scanState"`
	CreatedAt   string `json:"createdAt"`
}

func (application *Application) uploadAttachment(writer http.ResponseWriter, request *http.Request) {
	workspaceID, entityID, ok := application.attachmentTarget(writer, request)
	if !ok {
		return
	}
	entityType := strings.TrimSpace(request.URL.Query().Get("entityType"))
	request.Body = http.MaxBytesReader(writer, request.Body, application.cfg.MaxUploadBytes+multipartEnvelopeAllowance)
	multipartReader, err := request.MultipartReader()
	if err != nil {
		writeError(application, writer, request, filesValidation("/headers/Content-Type", "validation.content_type.multipart_required"))
		return
	}
	part, err := multipartReader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		writeError(application, writer, request, filesValidation("/file", "validation.required"))
		return
	}
	defer part.Close()

	principal, _ := httpx.Principal(request.Context())
	var result files.UploadResult
	withWorkspace := func(callback func(*tenancy.WorkspaceTx) error) error {
		if entityType == "chat_message" {
			return application.tenancy.WithWorkspaceAny(request.Context(), principal, workspaceID,
				httpx.RequestID(request.Context()), chatAccessPermissions(), callback)
		}
		return application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
			httpx.RequestID(request.Context()), attachmentPermission(entityType, attachmentWrite), callback)
	}
	err = withWorkspace(
		func(workspace *tenancy.WorkspaceTx) error {
			var uploadErr error
			result, uploadErr = application.attachments.Upload(request.Context(), workspace, metadata(request, workspaceID, principal), files.UploadInput{
				EntityType: entityType, EntityID: entityID,
				DisplayName: part.FileName(), DeclaredMediaType: part.Header.Get("Content-Type"), Contents: part,
			})
			if uploadErr != nil {
				return uploadErr
			}
			extra, nextErr := multipartReader.NextPart()
			if nextErr == nil {
				_ = extra.Close()
				return filesValidation("/file", "attachment.single_file.required")
			}
			if nextErr != io.EOF {
				return filesValidation("/file", "validation.multipart.invalid")
			}
			return nil
		},
	)
	if err != nil {
		if result.StorageKey != "" {
			if cleanupErr := application.attachments.RemoveBlob(context.WithoutCancel(request.Context()), result.StorageKey); cleanupErr != nil {
				application.logger.Error("attachment rollback cleanup failed", "request_id", httpx.RequestID(request.Context()), "error", cleanupErr)
			}
		}
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	httpx.WriteJSON(writer, http.StatusCreated, attachmentFromRow(result.Attachment))
}

func (application *Application) listAttachments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, entityID, ok := application.attachmentTarget(writer, request)
	if !ok {
		return
	}
	entityType := strings.TrimSpace(request.URL.Query().Get("entityType"))
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var rows []dbgen.FilesAttachment
	withWorkspace := func(callback func(*tenancy.WorkspaceTx) error) error {
		if entityType == "chat_message" {
			return application.tenancy.WithWorkspaceAny(request.Context(), principal, workspaceID,
				httpx.RequestID(request.Context()), chatAccessPermissions(), callback)
		}
		return application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
			httpx.RequestID(request.Context()), attachmentPermission(entityType, attachmentRead), callback)
	}
	err = withWorkspace(
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			rows, loadErr = application.attachments.List(request.Context(), workspace, workspaceID, entityType, entityID, limit)
			return loadErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	items := make([]attachmentResponse, len(rows))
	for index, row := range rows {
		items[index] = attachmentFromRow(row)
	}
	httpx.WriteJSON(writer, http.StatusOK, items)
}

func (application *Application) downloadAttachment(writer http.ResponseWriter, request *http.Request) {
	workspaceID, attachmentID, ok := application.workspaceAndPathID(writer, request, "attachmentId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var attachment dbgen.FilesAttachment
	err := application.tenancy.WithWorkspaceAny(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), chatAccessPermissions(),
		func(workspace *tenancy.WorkspaceTx) error {
			candidate, candidateErr := workspace.Queries.GetAttachment(request.Context(), dbgen.GetAttachmentParams{
				WorkspaceID: workspaceID.PG(), ID: attachmentID.PG(),
			})
			if candidateErr != nil {
				if errors.Is(candidateErr, pgx.ErrNoRows) {
					return errx.ErrNotFound
				}
				return candidateErr
			}
			if candidate.EntityType != "chat_message" {
				required := attachmentPermission(candidate.EntityType, attachmentRead)
				if !workspace.Allows(required) {
					return errx.ErrForbidden
				}
			}
			var loadErr error
			attachment, loadErr = application.attachments.Get(request.Context(), workspace, workspaceID, attachmentID)
			return loadErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	reader, err := application.attachments.Open(request.Context(), attachment.StorageKey)
	if writeError(application, writer, request, err) {
		return
	}
	defer reader.Close()
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": attachment.DisplayName})
	writer.Header().Set("Content-Type", attachment.MediaType)
	writer.Header().Set("Content-Disposition", disposition)
	writer.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(http.StatusOK)
	if _, err := io.CopyBuffer(writer, reader, make([]byte, 32<<10)); err != nil && request.Context().Err() == nil {
		application.logger.Warn("attachment download interrupted", "request_id", httpx.RequestID(request.Context()), "error", err)
	}
}

func (application *Application) deleteAttachment(writer http.ResponseWriter, request *http.Request) {
	workspaceID, attachmentID, ok := application.workspaceAndPathID(writer, request, "attachmentId")
	if !ok {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var storageKey string
	err := application.tenancy.WithWorkspaceAny(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()),
		[]tenancy.Permission{
			tenancy.PermissionRecordsDelete,
			tenancy.PermissionDealsDelete,
			tenancy.PermissionRecordsRead,
			tenancy.PermissionLeadsRead,
			tenancy.PermissionDealsRead,
		},
		func(workspace *tenancy.WorkspaceTx) error {
			candidate, candidateErr := workspace.Queries.GetAttachment(request.Context(), dbgen.GetAttachmentParams{
				WorkspaceID: workspaceID.PG(), ID: attachmentID.PG(),
			})
			if candidateErr != nil {
				if errors.Is(candidateErr, pgx.ErrNoRows) {
					return errx.ErrNotFound
				}
				return candidateErr
			}
			if candidate.EntityType == "chat_message" {
				uploaderID, valid := ids.FromPG(candidate.UploadedBy)
				if !valid || uploaderID != principal.UserID {
					return errx.ErrForbidden
				}
			} else if !workspace.Allows(attachmentPermission(candidate.EntityType, attachmentDelete)) {
				return errx.ErrForbidden
			}
			var deleteErr error
			storageKey, deleteErr = application.attachments.MarkDeleted(request.Context(), workspace, metadata(request, workspaceID, principal), attachmentID)
			return deleteErr
		},
	)
	if writeError(application, writer, request, err) {
		return
	}
	if err := application.attachments.RemoveBlob(context.WithoutCancel(request.Context()), storageKey); err != nil {
		application.logger.Error("attachment blob cleanup deferred", "request_id", httpx.RequestID(request.Context()), "error", err)
	}
	writer.WriteHeader(http.StatusNoContent)
}

type attachmentOperation uint8

const (
	attachmentRead attachmentOperation = iota
	attachmentWrite
	attachmentDelete
)

func attachmentPermission(entityType string, operation attachmentOperation) tenancy.Permission {
	entityType = strings.TrimSpace(entityType)
	if entityType == "deal" {
		switch operation {
		case attachmentWrite:
			return tenancy.PermissionDealsUpdate
		case attachmentDelete:
			return tenancy.PermissionDealsDelete
		default:
			return tenancy.PermissionDealsRead
		}
	}
	switch operation {
	case attachmentWrite:
		return tenancy.PermissionRecordsCreate
	case attachmentDelete:
		return tenancy.PermissionRecordsDelete
	default:
		return tenancy.PermissionRecordsRead
	}
}

func (application *Application) attachmentTarget(writer http.ResponseWriter, request *http.Request) (ids.UUID, ids.UUID, bool) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return ids.UUID{}, ids.UUID{}, false
	}
	entityID, err := ids.Parse(strings.TrimSpace(request.URL.Query().Get("entityId")))
	if err != nil {
		writeError(application, writer, request, filesValidation("/query/entityId", "validation.uuid.invalid"))
		return ids.UUID{}, ids.UUID{}, false
	}
	return workspaceID, entityID, true
}

func attachmentFromRow(row dbgen.FilesAttachment) attachmentResponse {
	id, _ := ids.FromPG(row.ID)
	entityID, _ := ids.FromPG(row.EntityID)
	return attachmentResponse{
		ID: id.String(), EntityType: row.EntityType, EntityID: entityID.String(), DisplayName: row.DisplayName,
		MediaType: row.MediaType, SizeBytes: row.SizeBytes, ScanState: row.ScanState, CreatedAt: row.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}

func filesValidation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}
