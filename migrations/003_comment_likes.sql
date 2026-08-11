-- Comment reactions and denormalized heat counters.
-- This migration is safe to source repeatedly during local initialization.
SET @shareo_column_exists = (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'comments' AND COLUMN_NAME = 'like_count'
);
SET @shareo_sql = IF(
    @shareo_column_exists = 0,
    'ALTER TABLE comments ADD COLUMN like_count INT NOT NULL DEFAULT 0 AFTER content',
    'SELECT 1'
);
PREPARE shareo_stmt FROM @shareo_sql;
EXECUTE shareo_stmt;
DEALLOCATE PREPARE shareo_stmt;

SET @shareo_index_exists = (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'comments' AND INDEX_NAME = 'idx_comments_post_hot'
);
SET @shareo_sql = IF(
    @shareo_index_exists = 0,
    'ALTER TABLE comments ADD INDEX idx_comments_post_hot (post_id, parent_id, is_deleted, like_count DESC, created_at DESC, id DESC)',
    'SELECT 1'
);
PREPARE shareo_stmt FROM @shareo_sql;
EXECUTE shareo_stmt;
DEALLOCATE PREPARE shareo_stmt;

CREATE TABLE IF NOT EXISTS comment_likes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    comment_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_comment_likes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_comment_likes_comment FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE,
    UNIQUE KEY uk_comment_likes_user_comment (user_id, comment_id),
    INDEX idx_comment_likes_comment (comment_id),
    INDEX idx_comment_likes_user_created (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
