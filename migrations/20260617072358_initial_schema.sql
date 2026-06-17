-- +goose Up
CREATE TABLE IF NOT EXISTS app_user
(
    id              UUID PRIMARY KEY,
    login           TEXT        NOT NULL UNIQUE,
    password_hash   TEXT        NOT NULL,
    master_key_salt BYTEA       NOT NULL,
    registered_at   TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_token
(
    id          UUID PRIMARY KEY,
    app_user_id UUID        NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    token_hash  TEXT        NOT NULL UNIQUE,
    issued_at   TIMESTAMPTZ NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ NULL
);
CREATE INDEX idx_refresh_token_app_user_id ON refresh_token(app_user_id);

CREATE TABLE IF NOT EXISTS record
(
    id                UUID PRIMARY KEY,
    app_user_id       UUID        NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    type              VARCHAR(16) NOT NULL CHECK (type IN ('credential', 'text', 'card', 'binary')),
    title             TEXT        NOT NULL,
    description       TEXT        NOT NULL,
    encrypted_dek     BYTEA       NOT NULL,
    encrypted_payload BYTEA       NOT NULL,
    version           BIGINT      NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    deleted_at        TIMESTAMPTZ NULL
);
CREATE INDEX idx_record_app_user_id_deleted_at ON record(app_user_id, deleted_at);

CREATE TABLE IF NOT EXISTS record_file (
    id             UUID PRIMARY KEY,
    record_id      UUID        NOT NULL UNIQUE REFERENCES record(id) ON DELETE RESTRICT,
    object_key     TEXT        NOT NULL,
    encrypted_size BIGINT      NOT NULL,
    upload_mode    VARCHAR(16) NOT NULL CHECK (upload_mode IN ('single_part', 'multipart')),
    upload_status  VARCHAR(16) NOT NULL CHECK (upload_status IN ('pending', 'uploaded', 'failed')),
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS record_file;
DROP TABLE IF EXISTS record;
DROP TABLE IF EXISTS refresh_token;
DROP TABLE IF EXISTS app_user;
