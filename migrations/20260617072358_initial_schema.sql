-- +goose Up
CREATE TABLE IF NOT EXISTS app_user
(
    id                  UUID PRIMARY KEY,
    login               TEXT        NOT NULL UNIQUE,
    password_hash       TEXT        NOT NULL,
    master_key_salt     BYTEA       NOT NULL,
    master_key_verifier BYTEA       NOT NULL,
    security_version    BIGINT      NOT NULL DEFAULT 1,
    registered_at       TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_token
(
    id               UUID PRIMARY KEY,
    app_user_id      UUID        NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    token_hash       TEXT        NOT NULL UNIQUE,
    security_version BIGINT      NOT NULL DEFAULT 1,
    issued_at        TIMESTAMPTZ NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    revoked_at       TIMESTAMPTZ NULL
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
    encrypted_size BIGINT      NULL,
    encrypted_sha256 TEXT      NULL,
    upload_status  VARCHAR(16) NOT NULL CHECK (upload_status IN ('uploading', 'uploaded', 'failed')),
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS record_file_multipart_upload
(
    id                UUID PRIMARY KEY,
    app_user_id       UUID        NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    record_id         UUID        NOT NULL REFERENCES record(id) ON DELETE RESTRICT,
    record_version    BIGINT      NOT NULL,
    file_id           UUID        NOT NULL REFERENCES record_file(id) ON DELETE RESTRICT,
    object_key        TEXT        NOT NULL,
    storage_upload_id TEXT        NOT NULL,
    encrypted_size    BIGINT      NOT NULL,
    part_size         BIGINT      NOT NULL,
    status            VARCHAR(16) NOT NULL CHECK (status IN ('uploading', 'completed', 'aborted')),
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    completed_at      TIMESTAMPTZ NULL
);
CREATE INDEX idx_record_file_multipart_upload_app_user_status
    ON record_file_multipart_upload(app_user_id, status);

CREATE TABLE IF NOT EXISTS record_file_multipart_part
(
    upload_id   UUID        NOT NULL REFERENCES record_file_multipart_upload(id) ON DELETE CASCADE,
    part_number INTEGER     NOT NULL,
    size        BIGINT      NOT NULL,
    etag        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (upload_id, part_number)
);

-- +goose Down
DROP TABLE IF EXISTS record_file_multipart_part;
DROP TABLE IF EXISTS record_file_multipart_upload;
DROP TABLE IF EXISTS record_file;
DROP TABLE IF EXISTS record;
DROP TABLE IF EXISTS refresh_token;
DROP TABLE IF EXISTS app_user;
