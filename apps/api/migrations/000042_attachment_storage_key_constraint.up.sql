SET ROLE veltrix_owner;

ALTER TABLE files.attachments
  DROP CONSTRAINT attachments_storage_key_check,
  ADD CONSTRAINT attachments_storage_key_check CHECK (
    char_length(storage_key) BETWEEN 1 AND 500
    AND storage_key !~ '[^a-zA-Z0-9/_-]'
  );

RESET ROLE;
