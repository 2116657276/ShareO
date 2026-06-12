-- ShareO Triggers
-- ============================================================

DELIMITER //

DROP TRIGGER IF EXISTS trg_after_like_insert //
CREATE TRIGGER trg_after_like_insert
AFTER INSERT ON likes
FOR EACH ROW
BEGIN
    UPDATE posts SET like_count = like_count + 1 WHERE id = NEW.post_id;
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'like_post', CONCAT('User ', NEW.user_id, ' liked post ', NEW.post_id));
END //

DROP TRIGGER IF EXISTS trg_after_like_delete //
CREATE TRIGGER trg_after_like_delete
AFTER DELETE ON likes
FOR EACH ROW
BEGIN
    UPDATE posts SET like_count = GREATEST(like_count - 1, 0) WHERE id = OLD.post_id;
END //

DROP TRIGGER IF EXISTS trg_after_comment_insert //
CREATE TRIGGER trg_after_comment_insert
AFTER INSERT ON comments
FOR EACH ROW
BEGIN
    UPDATE posts SET comment_count = comment_count + 1 WHERE id = NEW.post_id;
END //

DROP TRIGGER IF EXISTS trg_after_post_insert //
CREATE TRIGGER trg_after_post_insert
AFTER INSERT ON posts
FOR EACH ROW
BEGIN
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'create_post', CONCAT('User ', NEW.user_id, ' created post ', NEW.id));
END //

DELIMITER ;
