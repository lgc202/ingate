-- +goose NO TRANSACTION
-- +goose Up

-- request_records 保存不可变的请求元数据，不保存 Header、请求体或响应体。
-- 在线重投首先由 insert_deduplication_token 拦截，ReplacingMergeTree 只作为明细最终收敛的第二道保障。
CREATE TABLE IF NOT EXISTS request_records
(
    id String,
    request_id String,
    started_at DateTime64(9, 'UTC'),
    duration_ns Nullable(UInt64),
    client_ip String,
    method LowCardinality(String),
    host String,
    path String,
    status_code UInt16,
    status_class UInt8,
    request_bytes UInt64,
    response_bytes UInt64,
    gateway_id String,
    route_id String,
    upstream_id String,
    caller_id String,
    access_key_id String,
    envoy_node_id LowCardinality(String),
    protocol LowCardinality(String),
    response_code_details String,
    upstream_attempts UInt16,
    upstream_address String,
    time_to_first_byte_ns Nullable(UInt64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(started_at)
ORDER BY (started_at, id)
-- 去重日志只覆盖最近的在线重投；离线历史回放需要重建对应聚合时间范围。
SETTINGS non_replicated_deduplication_window = 100000;

-- request_metrics_1m 保存物化视图产生的分钟级聚合状态。
-- 更大的查询时间粒度在查询时由这些分钟状态继续合并，无需重复保存多套事实。
CREATE TABLE IF NOT EXISTS request_metrics_1m
(
    started_at DateTime('UTC'),
    gateway_id String,
    route_id String,
    upstream_id String,
    request_count SimpleAggregateFunction(sum, UInt64),
    client_error_count SimpleAggregateFunction(sum, UInt64),
    server_error_count SimpleAggregateFunction(sum, UInt64),
    no_response_count SimpleAggregateFunction(sum, UInt64),
    duration_average AggregateFunction(avg, Nullable(UInt64)),
    duration_p50 AggregateFunction(quantileTDigest(0.5), Nullable(UInt64)),
    duration_p95 AggregateFunction(quantileTDigest(0.95), Nullable(UInt64)),
    duration_p99 AggregateFunction(quantileTDigest(0.99), Nullable(UInt64))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(started_at)
ORDER BY (started_at, gateway_id, route_id, upstream_id)
SETTINGS non_replicated_deduplication_window = 100000;

-- 增量物化视图只处理新插入的请求批次，为控制台实时趋势减少明细扫描量。
-- Analytics 为每条 Kafka 事件设置稳定去重标识，ClickHouse 会在物化视图执行前过滤重投，
-- 因此请求明细和分钟聚合使用同一个幂等边界。
CREATE MATERIALIZED VIEW IF NOT EXISTS request_metrics_1m_mv TO request_metrics_1m
AS SELECT
    toStartOfMinute(started_at) AS started_at,
    gateway_id,
    route_id,
    upstream_id,
    count() AS request_count,
    countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
    countIf(status_code >= 500) AS server_error_count,
    countIf(status_code < 100) AS no_response_count,
    avgState(duration_ns) AS duration_average,
    quantileTDigestState(0.5)(duration_ns) AS duration_p50,
    quantileTDigestState(0.95)(duration_ns) AS duration_p95,
    quantileTDigestState(0.99)(duration_ns) AS duration_p99
FROM request_records
GROUP BY started_at, gateway_id, route_id, upstream_id;

-- +goose Down

DROP TABLE IF EXISTS request_metrics_1m_mv;
DROP TABLE IF EXISTS request_metrics_1m;
DROP TABLE IF EXISTS request_records;
