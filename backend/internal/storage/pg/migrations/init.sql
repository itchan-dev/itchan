CREATE TABLE IF NOT EXISTS users (
    id                     serial PRIMARY KEY,
    email                  varchar(254) NOT NULL UNIQUE,
    pass_hash              varchar(80) NOT NULL,
    is_admin               boolean default false,
    created                timestamp default (now() at time zone 'utc')
);

CREATE TABLE IF NOT EXISTS confirmation_data (
    email                  varchar(254) PRIMARY KEY,
    new_pass_hash          varchar(80) NOT NULL,
    confirmation_code_hash varchar(80) default '',
    confirmation_expires   timestamp default (now() at time zone 'utc'),
    created                timestamp default (now() at time zone 'utc')
);

CREATE TABLE IF NOT EXISTS boards (
    short_name     varchar(10) PRIMARY KEY,
    name           varchar(254) NOT NULL,
    allowed_emails text[],
    created        timestamp default (now() at time zone 'utc'),
    last_activity  timestamp default (now() at time zone 'utc')
);

-- metainfo in first thread message
CREATE TABLE threads (
    id            int NOT NULL,
    title         text NOT NULL,
    board         varchar(10) NOT NULL REFERENCES boards,
	num_replies   int NOT NULL default 0,
	last_bump_ts  timestamp NOT NULL default (now() at time zone 'utc'),
    is_sticky     boolean NOT NULL default false,
    PRIMARY KEY (board, id)
) PARTITION BY LIST (board);

CREATE TABLE messages (
    id          int NOT NULL,
    author_id   int NOT NULL,
    text        text NOT NULL,
    created     timestamp NOT NULL default (now() at time zone 'utc'),
    attachments text[],
    thread_id   int NOT NULL,
    ordinal     int NOT NULL default 0,
    modified    timestamp NOT NULL default (now() at time zone 'utc'),
    op          boolean NOT NULL default false,
    board       varchar(10) NOT NULL REFERENCES boards,
    PRIMARY KEY (board, id),
    FOREIGN KEY (board, thread_id) REFERENCES threads ON DELETE CASCADE
) PARTITION BY LIST (board);
CREATE INDEX messages__thread_id__index ON messages (board, thread_id);
