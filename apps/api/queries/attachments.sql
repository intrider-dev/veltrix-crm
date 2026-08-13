-- name: CreateAttachment :one
INSERT INTO files.attachments (
  workspace_id, id, entity_type, entity_id, storage_backend, storage_key,
  display_name, media_type, size_bytes, checksum_sha256, scan_state, uploaded_by
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(entity_type), sqlc.arg(entity_id),
  sqlc.arg(storage_backend), sqlc.arg(storage_key), sqlc.arg(display_name),
  sqlc.arg(media_type), sqlc.arg(size_bytes), sqlc.arg(checksum_sha256),
  sqlc.arg(scan_state), sqlc.arg(uploaded_by)
)
RETURNING workspace_id, id, entity_type, entity_id, storage_backend, storage_key,
          display_name, media_type, size_bytes, checksum_sha256, scan_state,
          uploaded_by, created_at, deleted_at;

-- name: GetAttachment :one
SELECT workspace_id, id, entity_type, entity_id, storage_backend, storage_key,
       display_name, media_type, size_bytes, checksum_sha256, scan_state,
       uploaded_by, created_at, deleted_at
FROM files.attachments
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: ListEntityAttachments :many
SELECT workspace_id, id, entity_type, entity_id, storage_backend, storage_key,
       display_name, media_type, size_bytes, checksum_sha256, scan_state,
       uploaded_by, created_at, deleted_at
FROM files.attachments
WHERE workspace_id = sqlc.arg(workspace_id)
  AND entity_type = sqlc.arg(entity_type)
  AND entity_id = sqlc.arg(entity_id)
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: MarkAttachmentDeleted :one
UPDATE files.attachments
SET deleted_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING storage_backend, storage_key;

-- name: UpdateAttachmentScanState :execrows
UPDATE files.attachments
SET scan_state = sqlc.arg(scan_state)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NULL;
