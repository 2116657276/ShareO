-- Persistent JWT revocation version. Safe to apply to databases created before
-- session versioning was introduced.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS session_version BIGINT NOT NULL DEFAULT 1;

UPDATE users
SET session_version = 1
WHERE session_version IS NULL OR session_version < 1;
