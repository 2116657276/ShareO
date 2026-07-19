-- ShareO v2 IM: Chat tables
-- Phase 1: Private chat (DM) + Group chat
-- See docs/design/im.md for data model rationale

-- Conversations: DM or group chat
CREATE TABLE IF NOT EXISTS conversations (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    type        VARCHAR(10)  NOT NULL DEFAULT 'dm'       COMMENT '"dm" or "group"',
    title       VARCHAR(100) NOT NULL DEFAULT ''         COMMENT 'group name (empty for dm)',
    owner_id    BIGINT       NOT NULL DEFAULT 0          COMMENT 'group owner (0 for dm)',
    dm_key      VARCHAR(50)  DEFAULT NULL                COMMENT 'dm unique key "smaller_uid:larger_uid", NULL for groups',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_dm_key (dm_key)                     -- NULLs allowed: multiple groups don't conflict
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Conversation members
CREATE TABLE IF NOT EXISTS conversation_members (
    id                   BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id      BIGINT       NOT NULL,
    user_id              BIGINT       NOT NULL,
    role                 VARCHAR(10)  NOT NULL DEFAULT 'member'  COMMENT '"owner" or "member"',
    last_read_message_id BIGINT       NOT NULL DEFAULT 0         COMMENT 'highest message id this user has read',
    joined_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_conv_user (conversation_id, user_id),
    INDEX idx_user_conv (user_id, conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id BIGINT       NOT NULL,
    sender_id       BIGINT       NOT NULL,
    content         TEXT         NOT NULL,
    meta            JSON         DEFAULT NULL                COMMENT 'extension: Bot citations etc.',
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_conv_msg (conversation_id, id)                 -- cursor pagination: WHERE conv_id=? AND id < before_id
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
