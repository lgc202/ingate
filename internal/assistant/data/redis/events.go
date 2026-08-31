// Package redis 保存运维助手可过期、可重放的流式事件。
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	redisgo "github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/conf"
)

const eventKeyPrefix = "ingate:assistant:v1:execution:"

// EventStore 使用 Redis Stream 保存 SSE 断线重连需要的短期事件。
// MySQL 中的消息和执行状态仍是持久事实，Stream 到期不会丢失最终结果。
type EventStore struct {
	client    *redisgo.Client
	retention time.Duration
	maxEvents int64
}

// NewEventStore 创建 Redis 客户端并验证连接。
func NewEventStore(
	ctx context.Context,
	config *conf.Data_Redis,
	stream *conf.Stream,
) (*EventStore, error) {
	client := redisgo.NewClient(&redisgo.Options{
		Addr:         config.GetAddress(),
		Password:     config.GetPassword(),
		DB:           int(config.GetDatabase()),
		DialTimeout:  config.GetDialTimeout().AsDuration(),
		ReadTimeout:  config.GetReadTimeout().AsDuration(),
		WriteTimeout: config.GetWriteTimeout().AsDuration(),
		PoolSize:     int(config.GetPoolSize()),
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect Redis: %w", err)
	}
	return &EventStore{
		client:    client,
		retention: stream.GetRetention().AsDuration(),
		maxEvents: int64(stream.GetMaxEvents()),
	}, nil
}

// Close 释放 EventStore 持有的 Redis 连接池。
func (s *EventStore) Close() error {
	return s.client.Close()
}

// Ping 检查 Redis 是否能够在当前请求期限内响应。
func (s *EventStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Append 追加一条执行流式事件，并刷新整个事件流的保留时间。
func (s *EventStore) Append(
	ctx context.Context,
	executionID string,
	event execution.StreamEvent,
) (string, error) {
	key := eventKey(executionID)
	pipeline := s.client.TxPipeline()
	command := pipeline.XAdd(ctx, &redisgo.XAddArgs{
		Stream: key,
		MaxLen: s.maxEvents,
		Approx: true,
		// go-redis 只接受基础标量或显式序列化类型，领域小类型在 data 边界转为字符串。
		Values: map[string]any{
			"type": string(event.Type),
			"data": event.Data,
		},
	})
	pipeline.Expire(ctx, key, s.retention)
	if _, err := pipeline.Exec(ctx); err != nil {
		return "", fmt.Errorf("append Redis stream event: %w", err)
	}
	return command.Val(), nil
}

// Read 从指定事件 ID 之后读取执行事件，支持 SSE 断线后的短时重放。
func (s *EventStore) Read(
	ctx context.Context,
	executionID string,
	lastID string,
	limit int64,
	block time.Duration,
) ([]execution.StreamEvent, error) {
	if lastID == "" {
		lastID = "0-0"
	}
	streams, err := s.client.XRead(ctx, &redisgo.XReadArgs{
		Streams: []string{eventKey(executionID), lastID},
		Count:   limit,
		Block:   block,
	}).Result()
	if errors.Is(err, redisgo.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Redis stream events: %w", err)
	}
	if len(streams) == 0 {
		return nil, nil
	}
	events := make([]execution.StreamEvent, 0, len(streams[0].Messages))
	for _, message := range streams[0].Messages {
		event, err := streamEventFromRedis(message)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func streamEventFromRedis(message redisgo.XMessage) (execution.StreamEvent, error) {
	typeText, typeOK := message.Values["type"].(string)
	data, dataOK := message.Values["data"].(string)
	if !typeOK || !dataOK {
		return execution.StreamEvent{}, errors.New("stream event fields must be strings")
	}
	eventType := execution.EventType(typeText)
	switch eventType {
	case execution.EventStarted,
		execution.EventReasoningDelta,
		execution.EventContentDelta,
		execution.EventCompleted,
		execution.EventFailed,
		execution.EventCancelled,
		execution.EventInterrupted:
	default:
		return execution.StreamEvent{}, fmt.Errorf("unsupported Redis stream event type %q", typeText)
	}
	return execution.StreamEvent{ID: message.ID, Type: eventType, Data: data}, nil
}

func eventKey(executionID string) string {
	// Hash tag 让同一次执行的所有事件在 Redis Cluster 中固定落到同一分片。
	return eventKeyPrefix + "{" + executionID + "}:events"
}
