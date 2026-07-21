-- Forward-only hardening for the Phase 1 chat schema.
-- Refuses to add constraints when orphan rows exist; the error text is a cleanup SELECT.

DROP PROCEDURE IF EXISTS shareo_harden_chat;
DELIMITER //
CREATE PROCEDURE shareo_harden_chat()
BEGIN
    DECLARE orphan_count BIGINT DEFAULT 0;

    SELECT COUNT(*) INTO orphan_count
      FROM conversation_members cm LEFT JOIN conversations c ON c.id = cm.conversation_id
     WHERE c.id IS NULL;
    IF orphan_count > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'SELECT cm.* FROM conversation_members cm LEFT JOIN conversations c ON c.id=cm.conversation_id WHERE c.id IS NULL';
    END IF;

    SELECT COUNT(*) INTO orphan_count
      FROM conversation_members cm LEFT JOIN users u ON u.id = cm.user_id
     WHERE u.id IS NULL;
    IF orphan_count > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'SELECT cm.* FROM conversation_members cm LEFT JOIN users u ON u.id=cm.user_id WHERE u.id IS NULL';
    END IF;

    SELECT COUNT(*) INTO orphan_count
      FROM messages m LEFT JOIN conversations c ON c.id = m.conversation_id
     WHERE c.id IS NULL;
    IF orphan_count > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'SELECT m.* FROM messages m LEFT JOIN conversations c ON c.id=m.conversation_id WHERE c.id IS NULL';
    END IF;

    SELECT COUNT(*) INTO orphan_count
      FROM messages m LEFT JOIN users u ON u.id = m.sender_id
     WHERE u.id IS NULL;
    IF orphan_count > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'SELECT m.* FROM messages m LEFT JOIN users u ON u.id=m.sender_id WHERE u.id IS NULL';
    END IF;

    SELECT COUNT(*) INTO orphan_count
      FROM conversations c LEFT JOIN users u ON u.id = c.owner_id
     WHERE c.type = 'group' AND (c.owner_id = 0 OR u.id IS NULL);
    IF orphan_count > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'SELECT c.* FROM conversations c LEFT JOIN users u ON u.id=c.owner_id WHERE c.type=''group'' AND u.id IS NULL';
    END IF;

    UPDATE conversations SET owner_id = NULL WHERE type = 'dm' AND owner_id = 0;
    ALTER TABLE conversations MODIFY owner_id BIGINT NULL DEFAULT NULL;

    IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
                   WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_chat_member_conversation') THEN
        ALTER TABLE conversation_members
          ADD CONSTRAINT fk_chat_member_conversation FOREIGN KEY (conversation_id)
          REFERENCES conversations(id) ON DELETE CASCADE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
                   WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_chat_conversation_owner') THEN
        ALTER TABLE conversations
          ADD CONSTRAINT fk_chat_conversation_owner FOREIGN KEY (owner_id)
          REFERENCES users(id) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
                   WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_chat_member_user') THEN
        ALTER TABLE conversation_members
          ADD CONSTRAINT fk_chat_member_user FOREIGN KEY (user_id)
          REFERENCES users(id) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
                   WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_chat_message_conversation') THEN
        ALTER TABLE messages
          ADD CONSTRAINT fk_chat_message_conversation FOREIGN KEY (conversation_id)
          REFERENCES conversations(id) ON DELETE CASCADE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.TABLE_CONSTRAINTS
                   WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = 'fk_chat_message_sender') THEN
        ALTER TABLE messages
          ADD CONSTRAINT fk_chat_message_sender FOREIGN KEY (sender_id)
          REFERENCES users(id) ON DELETE RESTRICT;
    END IF;
END//
DELIMITER ;

CALL shareo_harden_chat();
DROP PROCEDURE shareo_harden_chat;
