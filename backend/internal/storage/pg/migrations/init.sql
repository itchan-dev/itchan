-- Represents a user account
CREATE TABLE IF NOT EXISTS users (
    id                     serial PRIMARY KEY,
    email_encrypted        bytea NOT NULL,
    email_domain           varchar(254) NOT NULL,
    email_hash             bytea NOT NULL UNIQUE,
    password_hash          text NOT NULL,
    is_admin               boolean default false,
    created_at             timestamp default (now() at time zone 'utc')
);

-- Index on email_hash for fast lookups (unique constraint already creates an index)
-- Index on email_domain for board permission queries
CREATE INDEX IF NOT EXISTS idx_users_email_domain ON users(email_domain);

-- Stores blacklisted users
CREATE TABLE IF NOT EXISTS user_blacklist (
    user_id        int NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blacklisted_at timestamp NOT NULL DEFAULT (now() at time zone 'utc'),
    reason         text,
    blacklisted_by int REFERENCES users(id),
    PRIMARY KEY (user_id)
);

-- Index for efficient cache queries (fetch users blacklisted within JWT TTL)
CREATE INDEX IF NOT EXISTS idx_user_blacklist_time
    ON user_blacklist (blacklisted_at DESC);

-- Used for account confirmation or password resets
CREATE TABLE IF NOT EXISTS confirmation_data (
    email_hash             bytea PRIMARY KEY,
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
CREATE INDEX IF NOT EXISTS idx_board_permissions_email ON board_permissions (allowed_email_domain);

-- Represents a thread on a board
-- metainfo in first thread message
CREATE TABLE IF NOT EXISTS threads (
    id               bigint NOT NULL,
    title            text NOT NULL,
    board            varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
	message_count    int NOT NULL default 0,
	last_bumped_at   timestamp NOT NULL default (now() at time zone 'utc'),
	last_modified_at timestamp NOT NULL default (now() at time zone 'utc'),
    created_at       timestamp NOT NULL default (now() at time zone 'utc'),
    is_pinned        boolean NOT NULL default false,
    PRIMARY KEY (board, id)
) PARTITION BY LIST (board);
CREATE INDEX IF NOT EXISTS threads_last_bumped_at_index ON threads (board, last_bumped_at DESC);

-- Represents a single message within a thread
-- id is per-thread sequential (1, 2, 3...) - id=1 is always OP
CREATE TABLE IF NOT EXISTS messages (
    id          int NOT NULL,
    board       varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    thread_id   bigint NOT NULL,
    author_id   int NOT NULL REFERENCES users(id),
    text        text NOT NULL,
    show_email_domain boolean NOT NULL DEFAULT false,
    created_at  timestamp NOT NULL default (now() at time zone 'utc'),
    updated_at  timestamp NOT NULL default (now() at time zone 'utc'),

    PRIMARY KEY (board, thread_id, id),
    FOREIGN KEY (board, thread_id) REFERENCES threads(board, id) ON DELETE CASCADE
) PARTITION BY LIST (board);
-- Get all messages by a user (for moderation, user history, etc.)
CREATE INDEX IF NOT EXISTS idx_messages_author ON messages (author_id);

CREATE TABLE IF NOT EXISTS files (
    id                 bigserial PRIMARY KEY,
    file_path          text NOT NULL UNIQUE,
    filename           varchar(255) NOT NULL,
    original_filename  text NOT NULL,
    file_size_bytes    bigint NOT NULL,
    mime_type          varchar(255) NOT NULL,
    original_mime_type varchar(255) NOT NULL,
    image_width        int,
    image_height       int,
    thumbnail_path     text
    -- file_hash_sha256  bytea UNIQUE
);
COMMENT ON COLUMN files.filename IS 'Sanitized filename stored on disk (may differ from upload if sanitized, e.g., photo.gif -> photo.jpg)';
COMMENT ON COLUMN files.original_filename IS 'Filename as uploaded by user (before any sanitization)';
-- Partial index - only indexes non-NULL thumbnails
CREATE INDEX IF NOT EXISTS idx_files_thumbnail_path 
ON files (thumbnail_path) 
WHERE thumbnail_path IS NOT NULL;

-- Sequence for attachments (global, used across all board partitions)
CREATE SEQUENCE IF NOT EXISTS attachments_id_seq;

CREATE TABLE IF NOT EXISTS attachments (
    id                bigint NOT NULL DEFAULT nextval('attachments_id_seq'),
    board             varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    thread_id         bigint NOT NULL,
    message_id        int NOT NULL,
    file_id           bigint NOT NULL REFERENCES files(id) ON DELETE CASCADE,

    PRIMARY KEY (board, id),
    FOREIGN KEY (board, thread_id, message_id) REFERENCES messages(board, thread_id, id) ON DELETE CASCADE
) PARTITION BY LIST (board);
-- Create an index to quickly find all attachments for a given message
CREATE INDEX IF NOT EXISTS attachments_message_id_index ON attachments (board, thread_id, message_id);

-- Stores the relationship between a message and a message it replies to
CREATE TABLE IF NOT EXISTS message_replies (
    board               varchar(10) NOT NULL REFERENCES boards(short_name) ON DELETE CASCADE,
    sender_thread_id    BIGINT NOT NULL,
    sender_message_id   INT NOT NULL,
    receiver_thread_id  BIGINT NOT NULL,
    receiver_message_id INT NOT NULL,
    created_at          timestamp NOT NULL default (now() at time zone 'utc'),

    PRIMARY KEY (board, sender_thread_id, sender_message_id, receiver_thread_id, receiver_message_id),

    FOREIGN KEY (board, sender_thread_id, sender_message_id)
        REFERENCES messages(board, thread_id, id) ON DELETE CASCADE,

    FOREIGN KEY (board, receiver_thread_id, receiver_message_id)
        REFERENCES messages(board, thread_id, id) ON DELETE CASCADE
) PARTITION BY LIST (board);

-- Index to find all replies to a specific message
CREATE INDEX IF NOT EXISTS idx_message_replies_receiver_msg
    ON message_replies (board, receiver_thread_id, receiver_message_id);

-- Stores invite codes (similar to confirmation_data)
CREATE TABLE IF NOT EXISTS invite_codes (
    code_hash          varchar(80) PRIMARY KEY,      -- bcrypt hash of the invite code
    created_by         int NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at         timestamp NOT NULL DEFAULT (now() at time zone 'utc'),
    expires_at         timestamp NOT NULL,           -- Configurable expiration (default: 30 days)
    used_by            int REFERENCES users(id) ON DELETE SET NULL,
    used_at            timestamp                     -- NULL if unused

    -- Ensure consistency: both used_by and used_at must be NULL or both must be set
    CONSTRAINT used_consistency CHECK (
        (used_by IS NULL AND used_at IS NULL) OR
        (used_by IS NOT NULL AND used_at IS NOT NULL)
    )
);

-- Index to quickly find all invites created by a user
CREATE INDEX IF NOT EXISTS idx_invite_codes_created_by
    ON invite_codes (created_by, created_at DESC);

-- Index for cleanup of expired invites
CREATE INDEX IF NOT EXISTS idx_invite_codes_expires
    ON invite_codes (expires_at);

-- Index to check if a user has been invited (for analytics/tracking)
CREATE INDEX IF NOT EXISTS idx_invite_codes_used_by
    ON invite_codes (used_by) WHERE used_by IS NOT NULL;
