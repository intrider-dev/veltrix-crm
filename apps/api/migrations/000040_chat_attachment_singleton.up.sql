SET ROLE veltrix_owner;

-- Attachment upload retries are serialized with a transaction-scoped
-- advisory lock and return the existing active attachment. A schema-level
-- singleton index cannot be introduced safely on installations that may
-- already contain more than one historic attachment per message.
SELECT 1;

RESET ROLE;
