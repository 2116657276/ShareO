-- ShareO Seed Data
-- ============================================================

-- Seed topics
INSERT INTO topics (name, description) VALUES
('街头摄影挑战', '捕捉城市街头的精彩瞬间'),
('人像之美', '记录身边的人，展现独特魅力'),
('光影游戏', '用光线作画，探索无限可能'),
('日常碎片', '生活中的小确幸'),
('转帖接龙', '基于热门帖子二次创作');

-- Generate 50 users (n=1..50)
INSERT INTO users (username, password_hash, email, bio, role, status)
SELECT
    CONCAT('shareo_user_', n),
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    CONCAT('user', n, '@shareo.com'),
    CONCAT('摄影爱好者 #', n),
    'user', 1
FROM (
    SELECT (a.n + b.n*10 + 1) AS n
    FROM (SELECT 0 AS n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) a
    CROSS JOIN (SELECT 0 AS n UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4) b
    HAVING n <= 50
) nums;

-- Generate posts (1-4 per user, random)
INSERT INTO posts (user_id, content, cover_image, status, like_count, comment_count, view_count)
SELECT
    u.id,
    CONCAT('精彩作品分享 #', FLOOR(RAND()*100), ' 📸'),
    CONCAT('/static/img/demo/demo_', FLOOR(1 + RAND()*10), '.jpg'),
    'approved',
    FLOOR(RAND()*80),
    FLOOR(RAND()*20),
    FLOOR(RAND()*500)
FROM users u
CROSS JOIN (SELECT 1 AS n UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4) mult
WHERE u.role = 'user' AND RAND() > 0.4;

-- Post images (cover image as first image for each post)
INSERT INTO post_images (post_id, image_url, sort_order)
SELECT id, cover_image, 0 FROM posts;

-- Topic-post associations
INSERT IGNORE INTO topic_posts (topic_id, post_id)
SELECT t.id, p.id FROM topics t CROSS JOIN posts p WHERE RAND() > 0.6;

-- Likes
INSERT IGNORE INTO likes (user_id, post_id)
SELECT u.id, p.id FROM users u CROSS JOIN posts p WHERE RAND() > 0.85;

-- Comments
INSERT INTO comments (post_id, user_id, content)
SELECT p.id, FLOOR(1 + RAND() * 51), CONCAT('拍得真好！👍 #', FLOOR(RAND()*100))
FROM posts p WHERE RAND() > 0.75;

-- Update counters
UPDATE posts p SET like_count = (SELECT COUNT(*) FROM likes WHERE post_id = p.id);
UPDATE posts p SET comment_count = (SELECT COUNT(*) FROM comments WHERE post_id = p.id AND is_deleted = 0);
UPDATE topics t SET post_count = (SELECT COUNT(*) FROM topic_posts WHERE topic_id = t.id);

-- System logs for posts
INSERT INTO system_logs (user_id, action, detail)
SELECT user_id, 'create_post', CONCAT('Created post #', id) FROM posts;

-- Show stats
SELECT 'Total Users' AS metric, COUNT(*) AS value FROM users WHERE role = 'user'
UNION ALL SELECT 'Total Posts', COUNT(*) FROM posts WHERE is_deleted = 0
UNION ALL SELECT 'Total Likes', COUNT(*) FROM likes
UNION ALL SELECT 'Total Comments', COUNT(*) FROM comments WHERE is_deleted = 0
UNION ALL SELECT 'Avg Likes', ROUND(AVG(like_count), 1) FROM posts;
