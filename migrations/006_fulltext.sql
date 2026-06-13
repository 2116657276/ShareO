-- Add FULLTEXT index for Chinese content search (requires ngram parser)
-- Run this migration after the base schema is in place.
-- Note: MySQL 5.7.6+ with InnoDB supports FULLTEXT + ngram.
ALTER TABLE posts ADD FULLTEXT INDEX idx_posts_content_ft (content) WITH PARSER ngram;
