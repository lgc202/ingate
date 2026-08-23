-- +goose NO TRANSACTION
-- +goose Up

-- model_calls 保存网关向模型 Service 发起的实际调用
-- 只有已经选中并尝试模型 Service 的请求才进入该表，本地拒绝不属于模型调用
-- 单次调用明细由 retention.model_calls 控制保留时间。
CREATE TABLE IF NOT EXISTS model_calls
(
    request_record_id String,
    started_at DateTime64(9, 'UTC'),
    gateway_id String,
    route_id String,
    upstream_id String,
    caller_id String,
    access_key_id String,
    status_class UInt8,
    client_model String,
    upstream_model String,
    upstream_protocol LowCardinality(String),
    response_model String,
    finish_reason LowCardinality(String),
    input_tokens Nullable(UInt64),
    output_tokens Nullable(UInt64),
    total_tokens Nullable(UInt64)
)
ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(started_at)
ORDER BY (started_at, request_record_id)
-- 有限去重窗口覆盖消费失败和提交 offset 失败造成的在线重投。
-- 超出窗口的离线历史回放必须重建对应时间范围的 model_usage_1m。
SETTINGS non_replicated_deduplication_window = 100000;

-- model_usage_1m 保存控制台用量查询需要的加法指标。
-- 维度保留到调用方密钥，后续可以按 Caller 或单个 Access Key 汇总累计用量，
-- 无需重新扫描保留周期更短的模型调用明细。
-- 该表是长期累计用量账本，不跟随模型调用明细过期。
CREATE TABLE IF NOT EXISTS model_usage_1m
(
    started_at DateTime('UTC'),
    gateway_id String,
    route_id String,
    upstream_id String,
    caller_id String,
    access_key_id String,
    client_model String,
    upstream_model String,
    call_count UInt64,
    normal_response_count UInt64,
    token_reported_call_count UInt64,
    input_token_count UInt64,
    output_token_count UInt64,
    total_token_count UInt64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(started_at)
ORDER BY (
    started_at,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id,
    client_model,
    upstream_model
)
SETTINGS non_replicated_deduplication_window = 100000;

-- status_class = 1 对应 Analytics 领域中的正常响应。
-- Token 总量只有在厂商返回 total_tokens，或同时返回输入和输出 Token 时才视为完整上报。
-- 同一模型调用重投时，源表在物化视图执行前按稳定事件 ID 去重，避免累计用量翻倍。
CREATE MATERIALIZED VIEW IF NOT EXISTS model_usage_1m_mv TO model_usage_1m
AS SELECT
    toStartOfMinute(started_at) AS started_at,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id,
    client_model,
    upstream_model,
    count() AS call_count,
    countIf(status_class = 1) AS normal_response_count,
    countIf(isNotNull(total_tokens) OR (isNotNull(input_tokens) AND isNotNull(output_tokens))) AS token_reported_call_count,
    sum(ifNull(input_tokens, 0)) AS input_token_count,
    sum(ifNull(output_tokens, 0)) AS output_token_count,
    sum(coalesce(total_tokens, input_tokens + output_tokens, 0)) AS total_token_count
FROM model_calls
GROUP BY
    started_at,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id,
    client_model,
    upstream_model;

-- +goose Down

DROP TABLE IF EXISTS model_usage_1m_mv;
DROP TABLE IF EXISTS model_usage_1m;
DROP TABLE IF EXISTS model_calls;
