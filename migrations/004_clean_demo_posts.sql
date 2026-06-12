-- Clean up seed-data placeholder-image posts
-- Step 1: Find placeholder post IDs into a temp table
CREATE TEMPORARY TABLE tmp_demo_posts AS
SELECT id FROM posts WHERE cover_image LIKE '/static/img/demo/%';

-- Step 2: Delete all related data using the temp table
DELETE l FROM likes l INNER JOIN tmp_demo_posts t ON l.post_id = t.id;
DELETE c FROM comments c INNER JOIN tmp_demo_posts t ON c.post_id = t.id;
DELETE f FROM favorites f INNER JOIN tmp_demo_posts t ON f.post_id = t.id;
DELETE tp FROM topic_posts tp INNER JOIN tmp_demo_posts t ON tp.post_id = t.id;
DELETE pi FROM post_images pi INNER JOIN tmp_demo_posts t ON pi.post_id = t.id;

-- Step 3: Delete seed user logs (users 2-51 are seed users)
DELETE FROM system_logs WHERE user_id BETWEEN 2 AND 51;

-- Step 4: Delete the placeholder posts themselves
DELETE p FROM posts p INNER JOIN tmp_demo_posts t ON p.id = t.id;

-- Step 5: Drop temp table
DROP TEMPORARY TABLE tmp_demo_posts;

-- Show cleanup result
SELECT CONCAT('Remaining posts: ', COUNT(*)) AS result FROM posts WHERE is_deleted = 0;
SELECT CONCAT('Remaining likes: ', COUNT(*)) AS result FROM likes;
SELECT CONCAT('Remaining comments: ', COUNT(*)) AS result FROM comments WHERE is_deleted = 0;
