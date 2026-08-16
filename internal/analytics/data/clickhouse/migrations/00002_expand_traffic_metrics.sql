-- +goose NO TRANSACTION
-- +goose Up

-- 分钟聚合表只保留控制台需要长期查询的统计状态。重建时从请求明细回填，
-- 避免升级后历史区间的无响应数和延迟分位值突然为空。
DROP TABLE IF EXISTS request_metrics_1m_mv;
DROP TABLE IF EXISTS request_metrics_1m;

CREATE TABLE request_metrics_1m
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
ORDER BY (started_at, gateway_id, route_id, upstream_id);

INSERT INTO request_metrics_1m
SELECT
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

CREATE MATERIALIZED VIEW request_metrics_1m_mv TO request_metrics_1m
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

CREATE TABLE request_metrics_1m
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

INSERT INTO request_metrics_1m
SELECT
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

CREATE MATERIALIZED VIEW request_metrics_1m_mv TO request_metrics_1m
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
