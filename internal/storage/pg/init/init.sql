CREATE TABLE IF NOT EXISTS users(
    id          serial PRIMARY KEY, 
    email       varchar(254) NOT NULL UNIQUE,
    pass_hash   varchar(80) NOT NULL,
    is_admin    boolean default false,
    created     timestamp default current_timestamp
);

CREATE TABLE IF NOT EXISTS boards(
    short_name  varchar(10) PRIMARY KEY, 
    name        varchar(254) NOT NULL,
    created     timestamp default current_timestamp
);

-- metainfo in first thread message
-- thread id is first msg id
CREATE TABLE IF NOT EXISTS threads(
    id          int PRIMARY KEY,
    title       text NOT NULL,
    board       varchar(10) REFERENCES boards ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages(
    id          serial PRIMARY KEY, 
    author_id   int NOT NULL REFERENCES users,
    text        text NOT NULL,
    created     timestamp default current_timestamp,
    attachments text[],
    thread_id   int REFERENCES threads ON DELETE CASCADE,
    op          boolean NOT NULL default false
);
