-- Track generated image objects until a user claims them from a post.
CREATE TABLE IF NOT EXISTS upload_artifacts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    batch_id UUID NOT NULL,
    object_key VARCHAR(500) NOT NULL UNIQUE,
    post_id BIGINT REFERENCES posts(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    bound_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_upload_artifacts_user_batch ON upload_artifacts(user_id, batch_id);
CREATE INDEX IF NOT EXISTS idx_upload_artifacts_post ON upload_artifacts(post_id);
CREATE INDEX IF NOT EXISTS idx_upload_artifacts_expiry ON upload_artifacts(expires_at) WHERE post_id IS NULL;
