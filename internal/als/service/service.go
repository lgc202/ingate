// Package service 实现 Envoy ALS 协议入口和请求记录转换
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	accesslogdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
)

const envoyRouteNamePrefix = "ingate-route"

// Service 将 Envoy HTTP access log 转换为 Ingate 请求记录
type Service struct {
	accesslogservice.UnimplementedAccessLogServiceServer
	recorder *biz.Recorder
	logger   *slog.Logger
}

// NewService 创建 ALS gRPC 服务
func NewService(recorder *biz.Recorder, logger *slog.Logger) *Service {
	return &Service{recorder: recorder, logger: logger}
}

// StreamAccessLogs 持续接收 Envoy 批量发送的 HTTP access log
func (s *Service) StreamAccessLogs(stream accesslogservice.AccessLogService_StreamAccessLogsServer) error {
	var nodeID string
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(new(accesslogservice.StreamAccessLogsResponse))
		}
		if err != nil {
			return err
		}
		if identifier := message.GetIdentifier(); identifier != nil {
			// Envoy 只保证在流首批消息中携带标识，后续批次沿用当前流记录的节点 ID
			nodeID = identifier.GetNode().GetId()
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
			record, err := requestRecord(nodeID, entry)
			if err != nil {
				// 单条坏记录不应拖累同批有效记录，更不能让 Envoy 因 gRPC 失败反复重连
				discarded++
				continue
			}
			records = append(records, record)
		}
		if discarded > 0 {
			s.recorder.Discard(discarded)
			s.logger.Warn("incomplete HTTP access log entries discarded", "count", discarded, "envoy_node_id", nodeID)
		}
		if len(records) == 0 {
			continue
		}
		if err := s.recorder.Write(stream.Context(), records); err != nil {
			s.logger.Error("request record batch rejected", "error", err, "records", len(records), "envoy_node_id", nodeID)
			return status.Error(codes.Unavailable, "request record storage is unavailable")
		}
	}
}

func requestRecord(nodeID string, entry *accesslogdata.HTTPAccessLogEntry) (*alsv1.RequestRecord, error) {
	common := entry.GetCommonProperties()
	request := entry.GetRequest()
	response := entry.GetResponse()
	if common == nil || request == nil || response == nil || common.GetStartTime() == nil {
		return nil, errors.New("HTTP access log entry is incomplete")
	}
	if logType := common.GetAccessLogType(); logType != accesslogdata.AccessLogType_NotSet && logType != accesslogdata.AccessLogType_DownstreamEnd {
		// 周期日志描述尚未完成的请求，若与结束日志同时入库会重复计量请求量和 Token
		return nil, fmt.Errorf("HTTP access log type %s is not a completed request", logType)
	}
	if err := common.GetStartTime().CheckValid(); err != nil {
		return nil, fmt.Errorf("HTTP access log start time: %w", err)
	}
	gatewayID, routeID := resourceIDs(common.GetRouteName())

	record := &alsv1.RequestRecord{
		RequestId:           request.GetRequestId(),
		StartedAt:           timestamppb.New(common.GetStartTime().AsTime()),
		ClientIp:            socketAddress(common.GetDownstreamRemoteAddress()),
		Method:              request.GetRequestMethod().String(),
		Host:                request.GetAuthority(),
		Path:                requestPath(request.GetPath()),
		StatusCode:          response.GetResponseCode().GetValue(),
		RequestBytes:        request.GetRequestHeadersBytes() + request.GetRequestBodyBytes(),
		ResponseBytes:       response.GetResponseHeadersBytes() + response.GetResponseBodyBytes(),
		GatewayId:           gatewayID,
		RouteId:             routeID,
		UpstreamId:          common.GetUpstreamCluster(),
		EnvoyNodeId:         nodeID,
		Protocol:            httpProtocol(entry.GetProtocolVersion()),
		ResponseCodeDetails: response.GetResponseCodeDetails(),
		UpstreamAttempts:    common.GetUpstreamRequestAttemptCount(),
		UpstreamAddress:     socketEndpoint(common.GetUpstreamRemoteAddress()),
	}
	record.Id = recordID(nodeID, common.GetStreamId(), record.GetRequestId(), record.GetStartedAt())
	if duration := common.GetDuration(); duration != nil {
		record.Duration = durationpb.New(duration.AsDuration())
	}
	if duration := common.GetTimeToFirstDownstreamTxByte(); duration != nil {
		record.TimeToFirstByte = durationpb.New(duration.AsDuration())
	}
	return record, nil
}

func recordID(nodeID, streamID, requestID string, startedAt *timestamppb.Timestamp) string {
	// Envoy 重连后可能重发最后一批，稳定 ID 让 Kafka 消费端可以幂等入库
	identity := strings.Join([]string{nodeID, streamID, requestID}, "\x00")
	if startedAt != nil {
		identity += "\x00" + startedAt.String()
	}
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:])
}

func httpProtocol(version accesslogdata.HTTPAccessLogEntry_HTTPVersion) string {
	switch version {
	case accesslogdata.HTTPAccessLogEntry_HTTP10:
		return "HTTP/1.0"
	case accesslogdata.HTTPAccessLogEntry_HTTP11:
		return "HTTP/1.1"
	case accesslogdata.HTTPAccessLogEntry_HTTP2:
		return "HTTP/2"
	case accesslogdata.HTTPAccessLogEntry_HTTP3:
		return "HTTP/3"
	default:
		return ""
	}
}

func resourceIDs(routeName string) (string, string) {
	// Controller 生成的 Route 名称格式为 ingate-route/<gateway-id>/<route-id>[/<method>]
	parts := strings.Split(routeName, "/")
	if len(parts) < 3 || parts[0] != envoyRouteNamePrefix {
		return "", ""
	}
	return parts[1], parts[2]
}

func requestPath(value string) string {
	// 查询参数常含 Token、签名和业务标识；ALS 默认不采集它们，避免分析链路扩大敏感数据面
	path, _, _ := strings.Cut(value, "?")
	return path
}

func socketAddress(address *corev3.Address) string {
	if address == nil || address.GetSocketAddress() == nil {
		return ""
	}
	return address.GetSocketAddress().GetAddress()
}

func socketEndpoint(address *corev3.Address) string {
	socket := address.GetSocketAddress()
	if socket == nil {
		return ""
	}
	if socket.GetPortValue() == 0 {
		return socket.GetAddress()
	}
	return net.JoinHostPort(socket.GetAddress(), fmt.Sprint(socket.GetPortValue()))
}
