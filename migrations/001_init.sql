CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(50) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(100) NOT NULL DEFAULT '',
    avatar_url VARCHAR(500) NOT NULL DEFAULT '',
    bio VARCHAR(200) NOT NULL DEFAULT '',
    role ENUM('user', 'admin') NOT NULL DEFAULT 'user',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '1=active, 0=banned',
    is_bot TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_users_username (username),
    INDEX idx_users_status (status),
    INDEX idx_users_bot (is_bot)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO users (username, password_hash, email, bio, role, status, is_bot)
VALUES (
    'shareo_bot',
    '!shareo-bot-login-disabled!',
    'bot@shareo.local',
    '基于已审核帖子回答问题，并附上可访问来源。',
    'user',
    1,
    1
)
ON DUPLICATE KEY UPDATE
    password_hash = VALUES(password_hash),
    status = 1,
    is_bot = 1;

CREATE TABLE IF NOT EXISTS posts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    cover_image VARCHAR(500) NOT NULL DEFAULT '',
    view_count INT NOT NULL DEFAULT 0,
    like_count INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    status ENUM('pending', 'approved', 'rejected') NOT NULL DEFAULT 'pending',
    review_comment VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by BIGINT DEFAULT NULL,
    reviewed_at DATETIME DEFAULT NULL,
    is_deleted TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_posts_reviewer FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_posts_user_created (user_id, created_at DESC),
    INDEX idx_posts_status_created (status, is_deleted, created_at DESC),
    INDEX idx_posts_hot (status, is_deleted, like_count DESC, comment_count DESC, view_count DESC),
    FULLTEXT INDEX idx_posts_content_ft (content) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS post_images (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    post_id BIGINT NOT NULL,
    image_url VARCHAR(500) NOT NULL,
	sort_order INT NOT NULL DEFAULT 0,
	width INT NOT NULL DEFAULT 0,
	height INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_post_images_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    INDEX idx_post_images_post (post_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comments (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    post_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    parent_id BIGINT DEFAULT NULL,
    reply_to_uid BIGINT DEFAULT NULL,
    content TEXT NOT NULL,
    is_deleted TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_comments_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    CONSTRAINT fk_comments_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_comments_parent FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE,
    CONSTRAINT fk_comments_reply_user FOREIGN KEY (reply_to_uid) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_comments_post (post_id, parent_id, created_at),
    INDEX idx_comments_user (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS likes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_likes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_likes_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE KEY uk_likes_user_post (user_id, post_id),
    INDEX idx_likes_post (post_id),
    INDEX idx_likes_user_created (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS favorites (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_favorites_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_favorites_post FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE KEY uk_favorites_user_post (user_id, post_id),
    INDEX idx_favorites_user_created (user_id, created_at DESC),
    INDEX idx_favorites_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS follows (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    follower_id BIGINT NOT NULL,
    followee_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_follows_follower FOREIGN KEY (follower_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_follows_followee FOREIGN KEY (followee_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY uk_follows_pair (follower_id, followee_id),
    INDEX idx_follows_follower (follower_id, created_at DESC),
    INDEX idx_follows_followee (followee_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    type ENUM('like', 'comment', 'follow', 'review') NOT NULL,
    actor_id BIGINT NOT NULL,
    target_id BIGINT NOT NULL DEFAULT 0,
    is_read TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_notifications_actor FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_notifications_user (user_id, is_read, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS system_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT DEFAULT NULL,
	action VARCHAR(255) NOT NULL,
	detail TEXT,
	ip VARCHAR(45) NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_system_logs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
	INDEX idx_logs_user (user_id, created_at DESC),
	INDEX idx_logs_action (action(64)),
	INDEX idx_logs_time (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS conversations (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    dm_key VARCHAR(50) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_conversations_dm_key (dm_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS conversation_members (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_conversation_members_conversation FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    CONSTRAINT fk_conversation_members_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY uk_conversation_members_pair (conversation_id, user_id),
    INDEX idx_conversation_members_user (user_id, conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS messages (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id BIGINT NOT NULL,
    sender_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    meta JSON DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_messages_conversation FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    CONSTRAINT fk_messages_sender FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_messages_conversation (conversation_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS bot_replies (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    source_message_id BIGINT NOT NULL,
    reply_message_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_bot_replies_source FOREIGN KEY (source_message_id) REFERENCES messages(id) ON DELETE CASCADE,
    CONSTRAINT fk_bot_replies_reply FOREIGN KEY (reply_message_id) REFERENCES messages(id) ON DELETE CASCADE,
    UNIQUE KEY uk_bot_replies_source (source_message_id),
    UNIQUE KEY uk_bot_replies_reply (reply_message_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
