// Package service 实现 Envoy ALS 协议入口和请求记录转换。
package service

import (
	"errors"
	"io"
	"log/slog"

	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"github.com/google/wire"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
)

// ProviderSet 汇总 Envoy ALS 协议实现。
var ProviderSet = wire.NewSet(NewService)

// Service 将 Envoy HTTP access log 转换为 Ingate 请求记录。
type Service struct {
	accesslogservice.UnimplementedAccessLogServiceServer
	recorder *biz.Recorder
	logger   *slog.Logger
}

// NewService 创建 ALS gRPC 服务。
func NewService(recorder *biz.Recorder, logger *slog.Logger) *Service {
	return &Service{recorder: recorder, logger: logger}
}

// StreamAccessLogs 持续接收 Envoy 批量发送的 HTTP access log。
func (s *Service) StreamAccessLogs(stream accesslogservice.AccessLogService_StreamAccessLogsServer) error {
	var nodeID string
	for {
		message, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(new(accesslogservice.StreamAccessLogsResponse))
		}
		if err != nil {
			return err
		}
		if identifier := message.GetIdentifier(); identifier != nil {
			// Envoy 只保证在流首批消息中携带标识，后续批次沿用当前流记录的节点 ID。
			currentNodeID := identifier.GetNode().GetId()
			if currentNodeID == "" || (nodeID != "" && currentNodeID != nodeID) {
				return status.Error(codes.InvalidArgument, "envoy node identity is invalid")
			}
			nodeID = currentNodeID
		}
		if nodeID == "" {
			return status.Error(codes.InvalidArgument, "envoy node identity is required")
		}
		if tcpLogs := message.GetTcpLogs(); tcpLogs != nil {
			// Ingate 当前只代理 HTTP 流量，忽略意外的 TCP 记录比主动断开整条 ALS 流更安全
			s.recorder.Discard(len(tcpLogs.GetLogEntry()))
			continue
		}
		entries := message.GetHttpLogs().GetLogEntry()
		if len(entries) == 0 {
			continue
		}
		records := make([]*alsv1.RequestRecord, 0, len(entries))
		discarded := 0
		for _, entry := range entries {
			record, err := parseRequestRecord(nodeID, entry)
			if err != nil {
				// 单条坏记录不应拖累同批有效记录，更不能让 Envoy 因 gRPC 失败反复重连
				discarded++
				continue
			}
			records = append(records, record)
		}
		if discarded > 0 {
			s.recorder.Discard(discarded)
			s.logger.Warn("invalid HTTP access log entries discarded", "count", discarded, "envoy_node_id", nodeID)
		}
		if len(records) == 0 {
			continue
		}
		if err := s.recorder.Write(stream.Context(), records); err != nil {
			s.logger.Error("request record batch rejected", "err", err, "records", len(records), "envoy_node_id", nodeID)
			return status.Error(codes.Unavailable, "request record storage is unavailable")
		}
	}
}
