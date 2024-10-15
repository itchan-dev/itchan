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
    id          int PRIMARY KEY REFERENCES messages ON DELETE CASCADE,
    title       text NOT NULL,
    board       varchar(10) REFERENCES boards ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS thread_reply_counter(
    id            int PRIMARY KEY REFERENCES threads ON DELETE CASCADE,
    n             int NOT NULL default 1,
    last_reply_ts timestamp NOT NULL default now()
);

CREATE TABLE IF NOT EXISTS messages(
    id          serial PRIMARY KEY, 
    author_id   int NOT NULL,
    text        text NOT NULL,
    created     timestamp default current_timestamp,
    attachments text[],
    thread_id   int, -- null if this is OP message
    n           int NOT NULL default 0
);


-- CREATE MATERIALIZED VIEW $1 AS
-- SELECT
--     t.title as thread_title,
--     t.board as board,
--     trc.n as n_replies,
--     m.id as msg_id,
--     m.author_id as author_id,
--     m.text as text,
--     m.created as created,
--     m.attachments as attachments,
--     m.thread_id as thread_id,
--     CASE WHEN m.thread_id IS NULL THEN TRUE ELSE FALSE end as op,
--     m.n as reply_number,
--     trc.last_reply_ts as last_reply_ts,
--     row_number() over(partition by t.id order by last_reply_ts) as thread_order  --for pagination
-- FROM messages as m
-- LEFT JOIN threads as t
--     ON COALESCE(m.thread_id, m.id) = t.id -- thread_id is null for op message
-- LEFT JOIN thread_reply_counter as trc
--     ON t.id = trc.id
-- WHERE t.board = $2
--     AND (((trc.n - m.n) <= $3 AND m.thread_id is null) -- leave only n last replies without op message
--     OR m.thread_id is not null) -- and op message