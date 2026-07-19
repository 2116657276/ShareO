-- ShareO Triggers
-- ============================================================

DELIMITER //

DROP TRIGGER IF EXISTS trg_after_like_insert //
CREATE TRIGGER trg_after_like_insert
AFTER INSERT ON likes
FOR EACH ROW
BEGIN
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'like_post', CONCAT('User ', NEW.user_id, ' liked post ', NEW.post_id));
END //

DROP TRIGGER IF EXISTS trg_after_like_delete //
CREATE TRIGGER trg_after_like_delete
AFTER DELETE ON likes
FOR EACH ROW
BEGIN
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (OLD.user_id, 'unlike_post', CONCAT('User ', OLD.user_id, ' unliked post ', OLD.post_id));
END //

DROP TRIGGER IF EXISTS trg_after_comment_insert //
CREATE TRIGGER trg_after_comment_insert
AFTER INSERT ON comments
FOR EACH ROW
BEGIN
    INSERT INTO system_logs (user_id, action, detail)
    VALUES (NEW.user_id, 'comment_post', CONCAT('User ', NEW.user_id, ' commented on post ', NEW.post_id));
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
