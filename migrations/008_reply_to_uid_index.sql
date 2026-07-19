-- ShareO Migration 008: Add INDEX and FOREIGN KEY for comments.reply_to_uid
-- ============================================================
-- Fixes: Full table scan on JOINs; orphan reply_to_uid references

ALTER TABLE comments
    ADD INDEX idx_comments_reply_to_uid (reply_to_uid);

-- Add foreign key constraint (only if no orphan data exists)
-- ALTER TABLE comments
--     ADD CONSTRAINT fk_comments_reply_to_uid
--         FOREIGN KEY (reply_to_uid) REFERENCES users(id)
--         ON DELETE SET NULL;
