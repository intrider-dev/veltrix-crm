package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

const maxAttachmentPageSize = 100

type EntityAuthorizer interface {
	Exists(context.Context, *tenancy.WorkspaceTx, ids.UUID, string, ids.UUID) (bool, error)
}

type Service struct {
	store      BlobStore
	scanner    Scanner
	authorizer EntityAuthorizer
	maxBytes   int64
	backend    string
}

type UploadInput struct {
	EntityType        string
	EntityID          ids.UUID
	DisplayName       string
	DeclaredMediaType string
	Contents          io.Reader
}

type UploadResult struct {
	Attachment dbgen.FilesAttachment
	StorageKey string
}

func NewService(store BlobStore, scanner Scanner, authorizer EntityAuthorizer, maxBytes int64) (*Service, error) {
	if store == nil || authorizer == nil {
		return nil, errors.New("attachment store and entity authorizer are required")
	}
	if scanner == nil {
		scanner = UnavailableScanner{}
	}
	if maxBytes < 1 {
		return nil, errors.New("positive attachment size limit is required")
	}
	backend := "local"
	if _, isS3 := store.(*S3Store); isS3 {
		backend = "s3"
	}
	return &Service{store: store, scanner: scanner, authorizer: authorizer, maxBytes: maxBytes, backend: backend}, nil
}

// Upload streams an object into the configured blob store and records only its
// generated key. Callers must invoke RemoveBlob if their outer transaction
// subsequently fails to commit; that keeps storage and metadata convergent.
func (service *Service) Upload(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	input UploadInput,
) (UploadResult, error) {
	input.EntityType = strings.TrimSpace(input.EntityType)
	if !supportedEntityType(input.EntityType) || input.EntityType == "import" {
		return UploadResult{}, validation("/entityType", "validation.enum")
	}
	if input.Contents == nil {
		return UploadResult{}, validation("/file", "validation.required")
	}
	displayName, err := SanitizedDisplayName(input.DisplayName)
	if err != nil {
		return UploadResult{}, validation("/file", "attachment.filename.invalid")
	}
	exists, err := service.authorizer.Exists(ctx, workspace, metadata.WorkspaceID, input.EntityType, input.EntityID)
	if err != nil {
		return UploadResult{}, err
	}
	if !exists {
		return UploadResult{}, validation("/entityId", "validation.reference.invalid")
	}
	attachmentID, err := ids.NewV7()
	if err != nil {
		return UploadResult{}, err
	}
	storageKey := metadata.WorkspaceID.String() + "/" + attachmentID.String() + "/blob"
	blob, err := service.store.Put(ctx, storageKey, input.DeclaredMediaType, input.Contents, service.maxBytes)
	if err != nil {
		return UploadResult{}, mapStorageError(err)
	}
	keepBlob := false
	defer func() {
		if !keepBlob {
			_ = service.store.Delete(context.WithoutCancel(ctx), storageKey)
		}
	}()

	scanState, scanErr := service.scanner.Scan(ctx, service.store, storageKey, blob)
	if scanErr != nil {
		scanState = ScanUnavailable
	}
	if scanState != ScanClean && scanState != ScanRejected && scanState != ScanUnavailable {
		scanState = ScanUnavailable
	}
	row, err := workspace.Queries.CreateAttachment(ctx, dbgen.CreateAttachmentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: attachmentID.PG(), EntityType: input.EntityType,
		EntityID: input.EntityID.PG(), StorageBackend: service.backend, StorageKey: storageKey,
		DisplayName: displayName, MediaType: blob.MediaType, SizeBytes: blob.SizeBytes,
		ChecksumSha256: blob.ChecksumSHA256[:], ScanState: string(scanState), UploadedBy: metadata.ActorID.PG(),
	})
	if err != nil {
		return UploadResult{}, fmt.Errorf("record attachment: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "attachment.created", EventType: "files.attachment.created", AggregateType: "attachment", AggregateID: attachmentID,
		Summary: map[string]any{"entityType": input.EntityType, "entityId": input.EntityID.String(), "mediaType": blob.MediaType, "sizeBytes": blob.SizeBytes},
		Payload: map[string]any{"attachmentId": attachmentID.String(), "entityType": input.EntityType, "entityId": input.EntityID.String()},
	}); err != nil {
		return UploadResult{}, err
	}
	keepBlob = true
	return UploadResult{Attachment: row, StorageKey: storageKey}, nil
}

func (service *Service) List(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	entityType string,
	entityID ids.UUID,
	limit int,
) ([]dbgen.FilesAttachment, error) {
	entityType = strings.TrimSpace(entityType)
	if !supportedEntityType(entityType) || entityType == "import" {
		return nil, validation("/query/entityType", "validation.enum")
	}
	exists, err := service.authorizer.Exists(ctx, workspace, workspaceID, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errx.ErrNotFound
	}
	if limit < 1 {
		limit = 50
	}
	if limit > maxAttachmentPageSize {
		limit = maxAttachmentPageSize
	}
	rows, err := workspace.Queries.ListEntityAttachments(ctx, dbgen.ListEntityAttachmentsParams{
		WorkspaceID: workspaceID.PG(), EntityType: entityType, EntityID: entityID.PG(), PageLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	return rows, nil
}

func (service *Service) Get(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID, attachmentID ids.UUID,
) (dbgen.FilesAttachment, error) {
	row, err := workspace.Queries.GetAttachment(ctx, dbgen.GetAttachmentParams{WorkspaceID: workspaceID.PG(), ID: attachmentID.PG()})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.FilesAttachment{}, errx.ErrNotFound
	}
	if err != nil {
		return dbgen.FilesAttachment{}, fmt.Errorf("get attachment: %w", err)
	}
	entityID, ok := ids.FromPG(row.EntityID)
	if !ok {
		return dbgen.FilesAttachment{}, errx.ErrNotFound
	}
	allowed, err := service.authorizer.Exists(ctx, workspace, workspaceID, row.EntityType, entityID)
	if err != nil {
		return dbgen.FilesAttachment{}, err
	}
	if !allowed {
		return dbgen.FilesAttachment{}, errx.ErrNotFound
	}
	if row.ScanState == string(ScanRejected) {
		return dbgen.FilesAttachment{}, errx.ErrSecurityRejected
	}
	return row, nil
}

func (service *Service) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	reader, err := service.store.Open(ctx, storageKey)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	return reader, nil
}

func (service *Service) MarkDeleted(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	attachmentID ids.UUID,
) (string, error) {
	existing, err := workspace.Queries.GetAttachment(ctx, dbgen.GetAttachmentParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: attachmentID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errx.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get attachment before delete: %w", err)
	}
	entityID, ok := ids.FromPG(existing.EntityID)
	if !ok {
		return "", errx.ErrNotFound
	}
	allowed, err := service.authorizer.Exists(ctx, workspace, metadata.WorkspaceID, existing.EntityType, entityID)
	if err != nil {
		return "", err
	}
	if !allowed {
		return "", errx.ErrNotFound
	}
	row, err := workspace.Queries.MarkAttachmentDeleted(ctx, dbgen.MarkAttachmentDeletedParams{
		WorkspaceID: metadata.WorkspaceID.PG(), ID: attachmentID.PG(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errx.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("delete attachment metadata: %w", err)
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "attachment.deleted", EventType: "files.attachment.deleted", AggregateType: "attachment", AggregateID: attachmentID,
		Summary: map[string]any{"storageBackend": row.StorageBackend}, Payload: map[string]any{"attachmentId": attachmentID.String(), "storageKey": row.StorageKey},
	}); err != nil {
		return "", err
	}
	return row.StorageKey, nil
}

func (service *Service) RemoveBlob(ctx context.Context, storageKey string) error {
	if storageKey == "" {
		return nil
	}
	return service.store.Delete(ctx, storageKey)
}

func supportedEntityType(value string) bool {
	switch value {
	case "contact", "company", "deal", "activity", "project", "chat_message", "import":
		return true
	default:
		return false
	}
}

func validation(pointer, code string) error {
	return &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: code}}}
}

func mapStorageError(err error) error {
	switch {
	case errors.Is(err, ErrObjectTooLarge):
		return validation("/file", "validation.body.too_large")
	case errors.Is(err, ErrUnsupportedMedia):
		return validation("/file", "attachment.media_type.unsupported")
	case errors.Is(err, ErrInvalidStorageKey), errors.Is(err, ErrStorageKeyCollision):
		return errx.ErrSecurityRejected
	default:
		return fmt.Errorf("store attachment: %w", err)
	}
}
