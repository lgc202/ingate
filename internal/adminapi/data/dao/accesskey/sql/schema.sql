CREATE TABLE access_keys (
    id CHAR(36) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    name VARCHAR(128) NOT NULL,
    secret_hash BINARY(32) NOT NULL,
    secret_prefix VARCHAR(16) NOT NULL,
    secret_suffix CHAR(4) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    allowed_models JSON NOT NULL,
    expires_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY access_keys_name_uq (name),
    UNIQUE KEY access_keys_secret_hash_uq (secret_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
