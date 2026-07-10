-- +goose Up
CREATE TABLE IF NOT EXISTS app_user
(
    id                  UUID PRIMARY KEY,
    login               VARCHAR(128) NOT NULL UNIQUE,
    password_hash       VARCHAR(128) NOT NULL,
    master_key_salt     BYTEA        NOT NULL,
    master_key_verifier BYTEA        NOT NULL,
    security_version    BIGINT       NOT NULL DEFAULT 1,
    registered_at       TIMESTAMPTZ  NOT NULL,
    updated_at          TIMESTAMPTZ  NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_token
(
    id               UUID PRIMARY KEY,
    app_user_id      UUID         NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    token_hash       VARCHAR(128) NOT NULL UNIQUE,
    security_version BIGINT       NOT NULL DEFAULT 1,
    issued_at        TIMESTAMPTZ  NOT NULL,
    expires_at       TIMESTAMPTZ  NOT NULL,
    revoked_at       TIMESTAMPTZ  NULL
);
CREATE INDEX idx_refresh_token_app_user_id ON refresh_token(app_user_id);

CREATE TYPE record_type AS ENUM ('credential', 'text', 'card', 'binary');
CREATE TABLE IF NOT EXISTS record
(
    id                UUID PRIMARY KEY,
    app_user_id       UUID          NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    type              record_type   NOT NULL,
    title             VARCHAR(128)  NOT NULL,
    description       VARCHAR(1024) NOT NULL,
    encrypted_dek     BYTEA         NOT NULL,
    encrypted_payload BYTEA         NOT NULL,
    version           BIGINT        NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ   NOT NULL,
    updated_at        TIMESTAMPTZ   NOT NULL,
    deleted_at        TIMESTAMPTZ   NULL
);
CREATE INDEX idx_record_app_user_id_deleted_at ON record(app_user_id, deleted_at);

CREATE TYPE upload_status AS ENUM ('uploading', 'uploaded', 'failed');
CREATE TABLE IF NOT EXISTS record_file (
    id               UUID PRIMARY KEY,
    record_id        UUID          NOT NULL UNIQUE REFERENCES record(id) ON DELETE RESTRICT,
    object_key       VARCHAR(255)  NOT NULL,
    encrypted_size   BIGINT        NULL,
    encrypted_sha256 VARCHAR(64)   NULL,
    upload_status    upload_status NOT NULL,
    created_at       TIMESTAMPTZ   NOT NULL,
    updated_at       TIMESTAMPTZ   NOT NULL
);

CREATE TYPE multipart_upload_status AS ENUM ('uploading', 'completed', 'aborted');
CREATE TABLE IF NOT EXISTS record_file_multipart_upload
(
    id                UUID PRIMARY KEY,
    app_user_id       UUID                    NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
    record_id         UUID                    NOT NULL REFERENCES record(id) ON DELETE RESTRICT,
    record_version    BIGINT                  NOT NULL,
    file_id           UUID                    NOT NULL REFERENCES record_file(id) ON DELETE RESTRICT,
    object_key        VARCHAR(255)            NOT NULL,
    storage_upload_id VARCHAR(255)            NOT NULL,
    encrypted_size    BIGINT                  NOT NULL,
    part_size         BIGINT                  NOT NULL,
    status            multipart_upload_status NOT NULL,
    created_at        TIMESTAMPTZ             NOT NULL,
    updated_at        TIMESTAMPTZ             NOT NULL,
    completed_at      TIMESTAMPTZ             NULL
);
CREATE INDEX idx_record_file_multipart_upload_app_user_status
    ON record_file_multipart_upload(app_user_id, status);

CREATE TABLE IF NOT EXISTS record_file_multipart_part
(
    upload_id   UUID         NOT NULL REFERENCES record_file_multipart_upload(id) ON DELETE CASCADE,
    part_number INTEGER      NOT NULL,
    size        BIGINT       NOT NULL,
    etag        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (upload_id, part_number)
);

-- +goose Down
DROP TABLE IF EXISTS record_file_multipart_part;
DROP TABLE IF EXISTS record_file_multipart_upload;
DROP TABLE IF EXISTS record_file;
DROP TABLE IF EXISTS record;
DROP TABLE IF EXISTS refresh_token;
DROP TABLE IF EXISTS app_user;
DROP TYPE IF EXISTS multipart_upload_status;
DROP TYPE IF EXISTS upload_status;
DROP TYPE IF EXISTS record_type;
