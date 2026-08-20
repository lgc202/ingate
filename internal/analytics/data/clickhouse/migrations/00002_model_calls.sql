-- +goose NO TRANSACTION
-- +goose Up

-- model_calls 保存网关向模型 Service 发起的实际调用
-- 通用请求事实仍由 request_records 保存，两者通过请求记录 ID 和开始时间关联
CREATE TABLE IF NOT EXISTS model_calls
(
    request_record_id String,
    started_at DateTime64(9, 'UTC'),
    gateway_id String,
    route_id String,
    upstream_id String,
    caller_id String,
    access_key_id String,
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
ORDER BY (started_at, request_record_id);

-- +goose Down

DROP TABLE IF EXISTS model_calls;
