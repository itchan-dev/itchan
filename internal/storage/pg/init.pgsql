CREATE TABLE IF NOT EXISTS users(
    id          serial PRIMARY KEY, 
    email       varchar(254) NOT NULL UNIQUE,
    pass_hash   varchar(80) NOT NULL,
    created     timestamp default current_timestamp
);

CREATE TABLE IF NOT EXISTS boards(
    short_name  varchar(10) PRIMARY KEY, 
    name        varchar(254) NOT NULL,
    created     timestamp default current_timestamp
);

CREATE TABLE IF NOT EXISTS threads(
    id          int PRIMARY KEY,
    title       text NOT NULL,
    created     timestamp default current_timestamp,
    board       varchar(10) REFERENCES boards
);

-- if thread_id is null -> message is started new thread
CREATE TABLE IF NOT EXISTS messages(
    id          serial PRIMARY KEY, 
    author_id   int NOT NULL REFERENCES users,
    text        text NOT NULL,
    created     timestamp default current_timestamp,
    attachments text[],
    thread_id   int REFERENCES threads
);
