-- Represents a user account
CREATE TABLE IF NOT EXISTS users (
    id                     serial PRIMARY KEY,
    email                  varchar(254) NOT NULL UNIQUE,
    password_hash          text NOT NULL,
    is_admin               boolean default false,
    created_at             timestamp default (now() at time zone 'utc')
);

-- Used for account confirmation or password resets
CREATE TABLE IF NOT EXISTS confirmation_data (
    email                  varchar(254) PRIMARY KEY,
    password_hash          varchar(80) NOT NULL,
    confirmation_code_hash varchar(80) default '',
    expires_at             timestamp default (now() at time zone 'utc'),
    created_at             timestamp default (now() at time zone 'utc')
);

-- Represents a message board
CREATE TABLE IF NOT EXISTS boards (
    short_name        varchar(10) PRIMARY KEY,
    name              varchar(254) NOT NULL,
    created_at        timestamp default (now() at time zone 'utc'),
    last_activity_at  timestamp default (now() at time zone 'utc')
);

CREATE TABLE IF NOT EXISTS board_permissions (
    board_short_name     varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    allowed_email_domain varchar(254) NOT NULL,
    PRIMARY KEY (board_short_name, allowed_email_domain)
);
-- Index to quickly find all boards a user can access
CREATE INDEX IF NOT EXISTS ON board_permissions (allowed_email_domain);

-- Represents a thread on a board
-- metainfo in first thread message
CREATE TABLE IF NOT EXISTS threads (
    id               bigint NOT NULL,
    title            text NOT NULL,
    board            varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
	message_count    int NOT NULL default 0,
	last_bumped_at   timestamp NOT NULL default (now() at time zone 'utc'),
    created_at       timestamp NOT NULL default (now() at time zone 'utc'),
    is_sticky        boolean NOT NULL default false,
    PRIMARY KEY (board, id)
) PARTITION BY LIST (board);
CREATE INDEX IF NOT EXISTS threads_last_bumped_at_index ON threads (board, last_bumped_at DESC);

-- Represents a single message within a thread
CREATE TABLE IF NOT EXISTS messages (
    id          bigint NOT NULL,
    board       varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    thread_id   int NOT NULL,
    author_id   int NOT NULL REFERENCES users(id),
    text        text NOT NULL,
    is_op       boolean NOT NULL default false,
    ordinal     int NOT NULL default 0,
    created_at  timestamp NOT NULL default (now() at time zone 'utc'),
    updated_at  timestamp NOT NULL default (now() at time zone 'utc'),

    PRIMARY KEY (board, id),
    FOREIGN KEY (board, thread_id) REFERENCES threads(board, id) ON DELETE CASCADE,
    UNIQUE (board, thread_id, id)
) PARTITION BY LIST (board);
-- Get all messages for certain thread
CREATE INDEX IF NOT EXISTS messages_thread_id_index ON messages (board, thread_id);
-- Get all messages by a user (for moderation, user history, etc.)
CREATE INDEX IF NOT EXISTS idx_messages_author ON messages (author_id);

CREATE TABLE IF NOT EXISTS files (
    id                bigserial PRIMARY KEY,
    file_path         text NOT NULL UNIQUE,
    original_filename text NOT NULL,
    file_size_bytes   bigint NOT NULL,
    mime_type         varchar(255) NOT NULL,
    image_width       int,
    image_height      int,
    thumbnail_path    text
    -- file_hash_sha256  bytea UNIQUE
);
-- Partial index - only indexes non-NULL thumbnails
CREATE INDEX IF NOT EXISTS idx_files_thumbnail_path 
ON files (thumbnail_path) 
WHERE thumbnail_path IS NOT NULL;

CREATE TABLE IF NOT EXISTS attachments (
    id                bigint NOT NULL,
    board             varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    message_id        int NOT NULL,
    file_id           bigint NOT NULL REFERENCES files(id) ON DELETE CASCADE,

    PRIMARY KEY (board, id),
    FOREIGN KEY (board, message_id) REFERENCES messages(board, id) ON DELETE CASCADE
) PARTITION BY LIST (board);
-- Create an index to quickly find all attachments for a given message
CREATE INDEX IF NOT EXISTS attachments_message_id_index ON attachments (board, message_id);

-- Stores the relationship between a message and a message it replies to
CREATE TABLE IF NOT EXISTS message_replies (
    board               varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    sender_thread_id    INT NOT NULL,
    sender_message_id   INT NOT NULL,
    receiver_thread_id  INT NOT NULL,
    receiver_message_id INT NOT NULL,
    created_at          timestamp NOT NULL default (now() at time zone 'utc'),

    PRIMARY KEY (board, sender_message_id, receiver_message_id),

    -- This foreign key now works because of the UNIQUE constraint in the messages table
    FOREIGN KEY (board, sender_thread_id, sender_message_id)
        REFERENCES messages(board, thread_id, id) ON DELETE CASCADE,

    -- This foreign key also works now
    FOREIGN KEY (board, receiver_thread_id, receiver_message_id)
        REFERENCES messages(board, thread_id, id) ON DELETE CASCADE
) PARTITION BY LIST (board);

-- Index to find all replies within a specific thread
CREATE INDEX IF NOT EXISTS idx_message_replies_receiver_thread
    ON message_replies (board, receiver_thread_id);

-- Index to find all replies to a specific message
CREATE INDEX IF NOT EXISTS idx_message_replies_receiver_msg
    ON message_replies (board, receiver_message_id);
