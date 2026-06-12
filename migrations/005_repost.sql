-- Repost system: add repost fields to posts table
ALTER TABLE posts ADD COLUMN is_repost TINYINT DEFAULT 0 AFTER favorite_count;
ALTER TABLE posts ADD COLUMN repost_of_id BIGINT DEFAULT NULL AFTER is_repost;
ALTER TABLE posts ADD COLUMN repost_text TEXT AFTER repost_of_id;
ALTER TABLE posts ADD FOREIGN KEY (repost_of_id) REFERENCES posts(id) ON DELETE SET NULL;
