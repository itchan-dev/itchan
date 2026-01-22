-- Create a partition without auto-increment sequence
-- Used for tables where id is set explicitly (messages, attachments, message_replies)
CREATE TABLE %[1]s PARTITION OF %[2]s
FOR VALUES IN (%[3]s);
