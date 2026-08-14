-- +goose NO TRANSACTION
-- +goose Up

-- request_records 保存不可变的请求元数据，不保存 Header、请求体或响应体。
-- ReplacingMergeTree 负责最终合并相同请求 ID。
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
-- 单机 ClickHouse 在本地保留最近 1000 个插入 Block 的去重标识，只覆盖短期原批重试。
SETTINGS non_replicated_deduplication_window = 1000;

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
    duration_average AggregateFunction(avg, Nullable(UInt64)),
    duration_p95 AggregateFunction(quantileTDigest(0.95), Nullable(UInt64))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(started_at)
ORDER BY (started_at, gateway_id, route_id, upstream_id);

-- 增量物化视图只处理新插入的请求批次，为控制台实时趋势减少明细扫描量。
-- 入库端对完全相同的重试批次启用 dependent materialized view 去重。
CREATE MATERIALIZED VIEW IF NOT EXISTS request_metrics_1m_mv TO request_metrics_1m
AS SELECT
    toStartOfMinute(started_at) AS started_at,
    gateway_id,
    route_id,
    upstream_id,
    count() AS request_count,
    countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
    countIf(status_code >= 500) AS server_error_count,
    avgState(duration_ns) AS duration_average,
    quantileTDigestState(0.95)(duration_ns) AS duration_p95
FROM request_records
GROUP BY started_at, gateway_id, route_id, upstream_id;

-- +goose Down

DROP TABLE IF EXISTS request_metrics_1m_mv;
DROP TABLE IF EXISTS request_metrics_1m;
DROP TABLE IF EXISTS request_records;
