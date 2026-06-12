-- ============================================================
-- ShareO Database Migration v1.0
-- 拍摄与作品管理系统
-- ============================================================

CREATE DATABASE IF NOT EXISTS shareo
    DEFAULT CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE shareo;

-- ============================================================
-- 1. 用户表
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(100) DEFAULT '',
    avatar_url VARCHAR(500) DEFAULT '',
    bio VARCHAR(200) DEFAULT '',
    role ENUM('user', 'admin') DEFAULT 'user',
    status TINYINT DEFAULT 1 COMMENT '1=正常 0=封禁',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_username (username),
    INDEX idx_users_role (role),
    INDEX idx_users_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 2. 帖子表（Feed核心内容，含审核状态）
-- ============================================================
CREATE TABLE IF NOT EXISTS posts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    content TEXT COMMENT '正文',
    cover_image VARCHAR(500) DEFAULT '' COMMENT '封面图',
    view_count INT DEFAULT 0,
    like_count INT DEFAULT 0,
    comment_count INT DEFAULT 0,
    favorite_count INT DEFAULT 0,
    share_count INT DEFAULT 0,
    status ENUM('pending', 'approved', 'rejected') DEFAULT 'pending' COMMENT '审核状态',
    review_comment VARCHAR(500) DEFAULT '' COMMENT '审核意见',
    reviewed_by BIGINT DEFAULT NULL,
    reviewed_at TIMESTAMP NULL DEFAULT NULL,
    is_deleted TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_posts_created (created_at DESC),
    INDEX idx_posts_hot (like_count DESC, created_at DESC),
    INDEX idx_posts_user (user_id, created_at DESC),
    INDEX idx_posts_status (status, created_at DESC),
    INDEX idx_posts_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 3. 帖子图片表（一帖多图）
-- ============================================================
CREATE TABLE IF NOT EXISTS post_images (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    post_id BIGINT NOT NULL,
    image_url VARCHAR(500) NOT NULL,
    sort_order INT DEFAULT 0,
    width INT DEFAULT 0,
    height INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    INDEX idx_post_images_post (post_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 4. 评论表（支持多级回复）
-- ============================================================
CREATE TABLE IF NOT EXISTS comments (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    post_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    parent_id BIGINT DEFAULT NULL COMMENT '父评论ID，NULL=一级评论',
    reply_to_uid BIGINT DEFAULT NULL COMMENT '回复的用户ID',
    content TEXT NOT NULL,
    like_count INT DEFAULT 0,
    is_deleted TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE,
    INDEX idx_comments_post (post_id, created_at),
    INDEX idx_comments_user (user_id, created_at DESC),
    INDEX idx_comments_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 5. 点赞表
-- ============================================================
CREATE TABLE IF NOT EXISTS likes (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE KEY uk_user_post (user_id, post_id),
    INDEX idx_likes_post (post_id),
    INDEX idx_likes_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 6. 收藏表
-- ============================================================
CREATE TABLE IF NOT EXISTS favorites (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE KEY uk_user_fav (user_id, post_id),
    INDEX idx_fav_user (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 7. 话题/挑战赛表（趣味玩法）
-- ============================================================
CREATE TABLE IF NOT EXISTS topics (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    cover_image VARCHAR(500) DEFAULT '',
    start_time TIMESTAMP NULL,
    end_time TIMESTAMP NULL,
    post_count INT DEFAULT 0,
    status TINYINT DEFAULT 1 COMMENT '1=进行中 0=已结束',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_topic_name (name),
    INDEX idx_topics_status (status, start_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 8. 话题-帖子关联表
-- ============================================================
CREATE TABLE IF NOT EXISTS topic_posts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    topic_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE KEY uk_topic_post (topic_id, post_id),
    INDEX idx_tp_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 9. 关注表
-- ============================================================
CREATE TABLE IF NOT EXISTS follows (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    follower_id BIGINT NOT NULL COMMENT '关注者',
    followee_id BIGINT NOT NULL COMMENT '被关注者',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (follower_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (followee_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY uk_follow (follower_id, followee_id),
    INDEX idx_follow_follower (follower_id),
    INDEX idx_follow_followee (followee_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 10. 系统日志表
-- ============================================================
CREATE TABLE IF NOT EXISTS system_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT DEFAULT NULL,
    action VARCHAR(255) NOT NULL,
    detail TEXT,
    ip VARCHAR(45) DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_logs_time (created_at DESC),
    INDEX idx_logs_user (user_id, created_at DESC),
    INDEX idx_logs_action (action(64))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 触发器：点赞自动更新热度 + 审计日志
-- ============================================================
DELIMITER //

CREATE TRIGGER trg_after_like_insert
AFTER INSERT ON likes
FOR EACH ROW
BEGIN
    UPDATE posts SET like_count = like_count + 1 WHERE id = NEW.post_id;
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'like_post', CONCAT('User ', NEW.user_id, ' liked post ', NEW.post_id));
END //

CREATE TRIGGER trg_after_like_delete
AFTER DELETE ON likes
FOR EACH ROW
BEGIN
    UPDATE posts SET like_count = GREATEST(like_count - 1, 0) WHERE id = OLD.post_id;
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (OLD.user_id, 'unlike_post', CONCAT('User ', OLD.user_id, ' unliked post ', OLD.post_id));
END //

CREATE TRIGGER trg_after_comment_insert
AFTER INSERT ON comments
FOR EACH ROW
BEGIN
    UPDATE posts SET comment_count = comment_count + 1 WHERE id = NEW.post_id;
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'comment_post', CONCAT('User ', NEW.user_id, ' commented on post ', NEW.post_id));
END //

CREATE TRIGGER trg_after_post_insert
AFTER INSERT ON posts
FOR EACH ROW
BEGIN
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'create_post', CONCAT('User ', NEW.user_id, ' created post ', NEW.id));
END //

DELIMITER ;

-- ============================================================
-- 视图：Feed流安全脱敏视图
-- ============================================================
CREATE OR REPLACE VIEW view_feed_stream AS
SELECT
    p.id AS post_id,
    p.user_id AS author_id,
    u.username AS author_name,
    u.avatar_url AS author_avatar,
    p.content,
    p.cover_image,
    p.like_count,
    p.comment_count,
    p.favorite_count,
    p.share_count,
    p.view_count,
    p.status,
    p.created_at AS publish_time
FROM posts p
JOIN users u ON p.user_id = u.id
WHERE p.is_deleted = 0 AND p.status = 'approved'
ORDER BY p.created_at DESC;

-- ============================================================
-- 存储过程：生成测试数据 + 统计信息
-- ============================================================
DELIMITER //

CREATE PROCEDURE Proc_GenerateTestData(
    IN user_count INT,
    OUT total_users INT,
    OUT avg_likes DECIMAL(10,2)
)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE curr_post_id BIGINT;
    DECLARE curr_user_id BIGINT;

    -- 创建话题
    INSERT IGNORE INTO topics (name, description, cover_image) VALUES
    ('街头摄影挑战', '捕捉城市街头的精彩瞬间，分享你的视角', ''),
    ('人像之美', '记录身边的人，展现人物的独特魅力', ''),
    ('光影游戏', '用光线作画，探索光影的无限可能', ''),
    ('日常碎片', '生活中的小确幸，随手记录美好', ''),
    ('转帖接龙', '基于热门帖子进行二次创作，延续精彩', '');

    -- 批量生成用户
    WHILE i <= user_count DO
        INSERT INTO users (username, password_hash, email, bio, role, status)
        VALUES (
            CONCAT('shareo_user_', i),
            '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', -- bcrypt of "password123"
            CONCAT('user', i, '@shareo.com'),
            CONCAT('我是摄影爱好者 #', i),
            'user',
            1
        );
        SET curr_user_id = LAST_INSERT_ID();

        -- 每个用户生成0-3条帖子
        SET @post_num = FLOOR(RAND() * 4);
        WHILE @post_num > 0 DO
            INSERT INTO posts (user_id, content, cover_image, status, like_count, comment_count, view_count)
            VALUES (
                curr_user_id,
                CONCAT('这是我的第', @post_num, '张作品 #ShareO'),
                CONCAT('/static/img/demo/demo_', (i % 10) + 1, '.jpg'),
                'approved',
                FLOOR(RAND() * 100),
                FLOOR(RAND() * 20),
                FLOOR(RAND() * 500)
            );
            SET curr_post_id = LAST_INSERT_ID();

            -- 为帖子添加图片
            INSERT INTO post_images (post_id, image_url, sort_order)
            VALUES (curr_post_id, CONCAT('/static/img/demo/demo_', (i % 10) + 1, '.jpg'), 0);

            -- 随机关联话题
            IF RAND() > 0.5 THEN
                INSERT IGNORE INTO topic_posts (topic_id, post_id)
                VALUES (FLOOR(1 + RAND() * 5), curr_post_id);
            END IF;

            SET @post_num = @post_num - 1;
        END WHILE;

        SET i = i + 1;
    END WHILE;

    -- 为帖子随机生成点赞（用户1-10给各帖子点赞）
    INSERT IGNORE INTO likes (user_id, post_id)
    SELECT u.id, p.id
    FROM users u
    CROSS JOIN posts p
    WHERE u.id <= 10 AND p.id IS NOT NULL AND RAND() > 0.7
    LIMIT 500;

    SELECT COUNT(*) INTO total_users FROM users WHERE role = 'user';
    SELECT COALESCE(AVG(like_count), 0) INTO avg_likes FROM posts;
END //

DELIMITER ;

-- ============================================================
-- 默认管理员账号 (password: admin123)
-- ============================================================
INSERT IGNORE INTO users (username, password_hash, email, role, status)
VALUES ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin@shareo.com', 'admin', 1);
