// Package service 实现 Envoy ALS 协议入口和请求记录转换。
package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	accesslogdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/google/wire"
	otelcodes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

// ProviderSet 汇总 Envoy ALS 协议实现。
var ProviderSet = wire.NewSet(NewService)

// Service 将 Envoy HTTP access log 转换为 Ingate 请求记录。
type Service struct {
	accesslogservice.UnimplementedAccessLogServiceServer
	recorder *biz.Recorder
	events   *alsmetrics.EventCollector
	logger   *slog.Logger
	tracer   oteltrace.Tracer
}

// NewService 创建 ALS gRPC 服务。
func NewService(
	recorder *biz.Recorder,
	events *alsmetrics.EventCollector,
	logger *slog.Logger,
	tracer oteltrace.Tracer,
) *Service {
	return &Service{recorder: recorder, events: events, logger: logger, tracer: tracer}
}

// StreamAccessLogs 持续接收 Envoy 批量发送的 HTTP access log。
// ALS 协议没有逐批确认；仅当 Kafka 和磁盘队列都无法接收记录时终止流。
// Envoy 会重新建立失败的流，但协议不保证重发当前批次；
// 单条无效记录只计入丢弃指标，并保留同批有效记录。
func (s *Service) StreamAccessLogs(stream accesslogservice.AccessLogService_StreamAccessLogsServer) error {
	s.events.StreamStarted()
	defer s.events.StreamFinished()

	var nodeID string
	for {
		message, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(new(accesslogservice.StreamAccessLogsResponse))
		}
		if err != nil {
			return err
		}

		startedAt := time.Now()
		recordCount := batchRecordCount(message)
		ctx, span := s.tracer.Start(
			stream.Context(),
			"als.receive_batch",
			// ALS 流可能长期存在。每批独立采样，避免整条流共享一次采样决定并形成超大 Trace。
			oteltrace.WithNewRoot(),
			oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
		)
		nodeID, err = s.acceptBatch(ctx, nodeID, message)
		if err != nil {
			// 具体错误由协议边界返回或记录，Span 只标记结果，避免意外采集请求内容。
			span.SetStatus(otelcodes.Error, "batch rejected")
		}
		span.End()
		s.events.ObserveBatch(recordCount, time.Since(startedAt))
		if err != nil {
			return err
		}
	}
}

func (s *Service) acceptBatch(
	ctx context.Context,
	nodeID string,
	message *accesslogservice.StreamAccessLogsMessage,
) (string, error) {
	nodeID, err := accessLogNodeID(nodeID, message)
	if err != nil {
		return "", err
	}

	if tcpLogs := message.GetTcpLogs(); tcpLogs != nil {
		// Ingate 当前只代理 HTTP 流量，忽略意外的 TCP 记录比主动断开整条 ALS 流更安全。
		s.recorder.Discard(len(tcpLogs.GetLogEntry()))
		return nodeID, nil
	}
	entries := message.GetHttpLogs().GetLogEntry()
	if len(entries) == 0 {
		return nodeID, nil
	}

	records, discardedCount, firstParseErr := parseRequestRecords(nodeID, entries)
	if discardedCount > 0 {
		s.recorder.Discard(discardedCount)
		s.logger.WarnContext(
			ctx,
			"invalid HTTP access log entries discarded",
			"err", firstParseErr,
			"count", discardedCount,
			"envoy_node_id", nodeID,
		)
	}
	if len(records) == 0 {
		return nodeID, nil
	}

	if err := s.recorder.Write(ctx, records); err != nil {
		return "", status.Error(codes.Unavailable, "request record storage is unavailable")
	}
	return nodeID, nil
}

func batchRecordCount(message *accesslogservice.StreamAccessLogsMessage) int {
	if tcpLogs := message.GetTcpLogs(); tcpLogs != nil {
		return len(tcpLogs.GetLogEntry())
	}
	return len(message.GetHttpLogs().GetLogEntry())
}

func accessLogNodeID(
	current string,
	message *accesslogservice.StreamAccessLogsMessage,
) (string, error) {
	identifier := message.GetIdentifier()
	if identifier == nil {
		if current == "" {
			return "", status.Error(codes.InvalidArgument, "envoy node identity is required")
		}
		return current, nil
	}

	// Envoy 只保证在流首批消息中携带标识，后续批次沿用当前流记录的节点 ID。
	nodeID := identifier.GetNode().GetId()
	if identifier.GetLogName() != requestrecord.StreamName ||
		nodeID == "" ||
		(current != "" && nodeID != current) {
		return "", status.Error(codes.InvalidArgument, "envoy access log stream identity is invalid")
	}
	return nodeID, nil
}

func parseRequestRecords(
	nodeID string,
	entries []*accesslogdata.HTTPAccessLogEntry,
) ([]*alsv1.RequestRecord, int, error) {
	records := make([]*alsv1.RequestRecord, 0, len(entries))
	discarded := 0
	var firstErr error
	for _, entry := range entries {
		record, err := parseRequestRecord(nodeID, entry)
		if err != nil {
			// 单条坏记录不应拖累同批有效记录，更不能让 Envoy 因 gRPC 失败反复重连。
			discarded++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		records = append(records, record)
	}
	return records, discarded, firstErr
}
