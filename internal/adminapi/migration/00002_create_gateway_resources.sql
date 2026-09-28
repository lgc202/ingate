-- +goose Up
CREATE TABLE gateways (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部主键',
    name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '网关稳定名称',
    display_name LONGTEXT NOT NULL COMMENT '供人阅读的网关名称',
    resource_version VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'API Server 资源版本',
    created_at DATETIME(6) NOT NULL COMMENT 'API Server 创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '查询模型更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_gateways_name (name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '网关';

CREATE TABLE gateway_listeners (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部主键',
    gateway_id BIGINT UNSIGNED NOT NULL COMMENT '所属网关主键',
    name VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '监听入口名称',
    protocol VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '监听协议',
    port SMALLINT UNSIGNED NOT NULL COMMENT '监听端口',
    hostname VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '匹配的主机名，空字符串表示任意主机',
    PRIMARY KEY (id),
    UNIQUE KEY uk_gateway_listeners_gateway_name (gateway_id, name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '网关监听入口';

CREATE TABLE routes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部主键',
    name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '路由稳定名称',
    gateway_name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '引用的网关名称',
    listener_name VARCHAR(63) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '引用的监听入口名称',
    path_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '路径匹配方式',
    path_value LONGTEXT NOT NULL COMMENT '路径匹配值',
    upstream_name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '引用的后端资源名称',
    resource_version VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'API Server 资源版本',
    created_at DATETIME(6) NOT NULL COMMENT 'API Server 创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '查询模型更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_routes_name (name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '路由';

CREATE TABLE upstreams (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部主键',
    name VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '后端资源稳定名称',
    resource_version VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'API Server 资源版本',
    created_at DATETIME(6) NOT NULL COMMENT 'API Server 创建时间',
    updated_at DATETIME(6) NOT NULL COMMENT '查询模型更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_upstreams_name (name)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '后端资源';

CREATE TABLE upstream_endpoints (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库内部主键',
    upstream_id BIGINT UNSIGNED NOT NULL COMMENT '所属后端资源主键',
    address VARCHAR(253) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '主机名或 IP 地址',
    port SMALLINT UNSIGNED NOT NULL COMMENT '服务端口',
    weight INT NOT NULL COMMENT '相对流量权重',
    PRIMARY KEY (id),
    UNIQUE KEY uk_upstream_endpoints_address_port (upstream_id, address, port)
) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '后端节点';

-- +goose Down
DROP TABLE upstream_endpoints;
DROP TABLE upstreams;
DROP TABLE routes;
DROP TABLE gateway_listeners;
DROP TABLE gateways;
