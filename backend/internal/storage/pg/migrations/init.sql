CREATE TABLE IF NOT EXISTS users(
    id                     serial PRIMARY KEY,
    email                  varchar(254) NOT NULL UNIQUE,
    pass_hash              varchar(80) NOT NULL,
    is_admin               boolean default false,
    created                timestamp default (now() at time zone 'utc')
);

CREATE TABLE IF NOT EXISTS confirmation_data(
    email                  varchar(254) PRIMARY KEY,
    new_pass_hash          varchar(80) NOT NULL,
    confirmation_code_hash varchar(6) default '',
    confirmation_expires   timestamp default (now() at time zone 'utc'),
    created                timestamp default (now() at time zone 'utc')
);

CREATE TABLE IF NOT EXISTS messages(
    id          serial PRIMARY KEY,
    author_id   int NOT NULL,
    text        text NOT NULL,
    created     timestamp default (now() at time zone 'utc'),
    attachments text[],
    thread_id   int default NULL, -- null if this is OP message
    n           int NOT NULL default 0
);

CREATE TABLE IF NOT EXISTS boards(
    short_name     varchar(10) PRIMARY KEY,
    name           varchar(254) NOT NULL,
    allowed_emails text[],
    created        timestamp default (now() at time zone 'utc')
);

-- metainfo in first thread message
-- thread id is first msg id
CREATE TABLE IF NOT EXISTS threads(
    id            int PRIMARY KEY REFERENCES messages ON DELETE CASCADE,
    title         text NOT NULL,
    board         varchar(10) REFERENCES boards ON DELETE CASCADE,
	reply_count   int NOT NULL default 0,
	last_bump_ts  timestamp NOT NULL 
);
