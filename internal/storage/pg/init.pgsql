CREATE TABLE IF NOT EXISTS users(
    id          serial PRIMARY KEY, 
    email       varchar(254) NOT NULL UNIQUE,
    pass_hash   varchar(80) NOT NULL,
    created     timestamp default current_timestamp
);
