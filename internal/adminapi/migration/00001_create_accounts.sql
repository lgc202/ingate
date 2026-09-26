-- +goose Up
CREATE TABLE accounts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '账户主键',
    username VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '登录用户名',
    password_hash VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '密码哈希',
    session_version BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '密码修改后递增，用于撤销旧会话',
    created_at DATETIME(6) NOT NULL COMMENT '创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_accounts_username (username)
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_bin
  COMMENT = '管理员账户';

-- +goose Down
DROP TABLE accounts;
