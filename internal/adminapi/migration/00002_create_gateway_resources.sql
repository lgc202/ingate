-- +goose Up
CREATE TABLE gateways (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
    name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '网关唯一标识',
    display_name LONGTEXT NOT NULL COMMENT '网关名称',
    version VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '对象版本，用于并发更新校验',
    created_at DATETIME(6) NOT NULL COMMENT '网关创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '记录更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_gateways_name (name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '网关资源';

CREATE TABLE gateway_listeners (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
    gateway_id BIGINT UNSIGNED NOT NULL COMMENT '所属网关 ID',
    name VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '监听入口名称',
    protocol VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '监听协议',
    port SMALLINT UNSIGNED NOT NULL COMMENT '监听端口',
    hostname VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '匹配的主机名，空字符串表示任意主机',
    PRIMARY KEY (id),
    UNIQUE KEY uk_gateway_listeners_gateway_name (gateway_id, name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '网关监听入口';

CREATE TABLE httproutes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
    name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '路由唯一标识',
    display_name LONGTEXT NOT NULL COMMENT '路由名称',
    parents JSON NOT NULL COMMENT '网关引用配置',
    rules JSON NOT NULL COMMENT '路由规则配置',
    version VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '对象版本，用于并发更新校验',
    created_at DATETIME(6) NOT NULL COMMENT '路由创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '记录更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_httproutes_name (name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = 'HTTP 路由';

CREATE TABLE backends (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
    name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '后端唯一标识',
    display_name LONGTEXT NOT NULL COMMENT '后端名称',
    version VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '对象版本，用于并发更新校验',
    created_at DATETIME(6) NOT NULL COMMENT '后端创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '记录更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_backends_name (name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '后端资源';

CREATE TABLE backend_endpoints (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
    backend_id BIGINT UNSIGNED NOT NULL COMMENT '所属后端 ID',
    address VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '主机名或 IP 地址',
    port SMALLINT UNSIGNED NOT NULL COMMENT '服务端口',
    weight INT NOT NULL COMMENT '相对流量权重',
    PRIMARY KEY (id),
    UNIQUE KEY uk_backend_endpoints_address_port (backend_id, address, port)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '后端节点';

-- +goose Down
DROP TABLE backend_endpoints;
DROP TABLE backends;
DROP TABLE httproutes;
DROP TABLE gateway_listeners;
DROP TABLE gateways;
