package service

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	accesslogdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

const (
	envoyRouteNamePrefix    = "ingate-route"
	maxExactMetadataInteger = 1<<53 - 1
)

func parseRequestRecord(nodeID string, entry *accesslogdata.HTTPAccessLogEntry) (*alsv1.RequestRecord, error) {
	common := entry.GetCommonProperties()
	request := entry.GetRequest()
	response := entry.GetResponse()
	if common == nil || request == nil || response == nil || common.GetStartTime() == nil {
		return nil, errors.New("HTTP access log entry is incomplete")
	}
	logType := common.GetAccessLogType()
	if logType != accesslogdata.AccessLogType_NotSet && logType != accesslogdata.AccessLogType_DownstreamEnd {
		// 周期日志描述尚未完成的请求，若与结束日志同时入库会重复计量请求量和 Token。
		return nil, fmt.Errorf("HTTP access log type %s is not a completed request", logType)
	}
	if err := common.GetStartTime().CheckValid(); err != nil {
		return nil, fmt.Errorf("HTTP access log start time: %w", err)
	}

	gatewayID, routeID, err := resourceIDs(common.GetRouteName())
	if err != nil {
		return nil, err
	}
	requestBytes, err := addByteCounts(request.GetRequestHeadersBytes(), request.GetRequestBodyBytes())
	if err != nil {
		return nil, fmt.Errorf("HTTP access log request size: %w", err)
	}
	responseBytes, err := addByteCounts(response.GetResponseHeadersBytes(), response.GetResponseBodyBytes())
	if err != nil {
		return nil, fmt.Errorf("HTTP access log response size: %w", err)
	}
	aiMetadata := metadataFields(common.GetMetadata(), aiprotocol.MetadataNamespace)
	authzMetadata := metadataFields(common.GetMetadata(), extauthz.MetadataNamespace)
	host := aiMetadata[aiprotocol.ClientHostField].GetStringValue()
	if host == "" {
		host = request.GetAuthority()
	}
	path := aiMetadata[aiprotocol.ClientPathField].GetStringValue()
	if path == "" {
		path = request.GetPath()
	}

	record := &alsv1.RequestRecord{
		RequestId:           request.GetRequestId(),
		StartedAt:           timestamppb.New(common.GetStartTime().AsTime()),
		ClientIp:            socketAddress(common.GetDownstreamRemoteAddress()),
		Method:              request.GetRequestMethod().String(),
		Host:                requestHost(host),
		Path:                requestPath(path),
		StatusCode:          response.GetResponseCode().GetValue(),
		RequestBytes:        requestBytes,
		ResponseBytes:       responseBytes,
		GatewayId:           gatewayID,
		RouteId:             routeID,
		UpstreamId:          common.GetUpstreamCluster(),
		EnvoyNodeId:         nodeID,
		Protocol:            httpProtocol(entry.GetProtocolVersion()),
		ResponseCodeDetails: response.GetResponseCodeDetails(),
		UpstreamAttempts:    common.GetUpstreamRequestAttemptCount(),
		UpstreamAddress:     socketEndpoint(common.GetUpstreamRemoteAddress()),
		AiModelCall:         aiModelCall(aiMetadata),
		CallerId:            authzMetadata[extauthz.CallerIDField].GetStringValue(),
		AccessKeyId:         authzMetadata[extauthz.AccessKeyIDField].GetStringValue(),
	}
	record.Id = requestrecord.NewID(
		nodeID,
		common.GetStreamId(),
		record.GetRequestId(),
		record.GetStartedAt().AsTime(),
	)
	duration, err := normalizedDuration(common.GetDuration())
	if err != nil {
		return nil, fmt.Errorf("HTTP access log duration: %w", err)
	}
	record.Duration = duration
	timeToFirstByte, err := normalizedDuration(common.GetTimeToFirstDownstreamTxByte())
	if err != nil {
		return nil, fmt.Errorf("HTTP access log time to first byte: %w", err)
	}
	if duration != nil && timeToFirstByte != nil && timeToFirstByte.AsDuration() > duration.AsDuration() {
		return nil, errors.New("HTTP access log time to first byte exceeds request duration")
	}
	record.TimeToFirstByte = timeToFirstByte
	if err := requestrecord.Validate(record); err != nil {
		return nil, fmt.Errorf("HTTP access log entry: %w", err)
	}
	if proto.Size(record) > requestrecord.MaxEncodedBytes {
		return nil, errors.New("HTTP access log entry exceeds the request record size limit")
	}
	return record, nil
}

func normalizedDuration(value *durationpb.Duration) (*durationpb.Duration, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, err
	}
	duration := value.AsDuration()
	if duration < 0 {
		return nil, errors.New("duration must not be negative")
	}
	return durationpb.New(duration), nil
}

func metadataFields(metadata *corev3.Metadata, namespace string) map[string]*structpb.Value {
	values := metadata.GetFilterMetadata()[namespace]
	if values == nil {
		return nil
	}
	return values.GetFields()
}

func aiModelCall(fields map[string]*structpb.Value) *alsv1.AIModelCall {
	call := &alsv1.AIModelCall{
		ClientModel:      fields[aiprotocol.ClientModelField].GetStringValue(),
		UpstreamModel:    fields[aiprotocol.UpstreamModelField].GetStringValue(),
		UpstreamProtocol: fields[aiprotocol.UpstreamProtocolField].GetStringValue(),
		ResponseModel:    fields[aiprotocol.ResponseModelField].GetStringValue(),
		FinishReason:     fields[aiprotocol.FinishReasonField].GetStringValue(),
		InputTokens:      metadataTokenCount(fields[aiprotocol.InputTokensField]),
		OutputTokens:     metadataTokenCount(fields[aiprotocol.OutputTokensField]),
		TotalTokens:      metadataTokenCount(fields[aiprotocol.TotalTokensField]),
	}
	if call.GetClientModel() == "" && call.GetUpstreamModel() == "" && call.GetUpstreamProtocol() == "" &&
		call.GetResponseModel() == "" && call.GetFinishReason() == "" && call.InputTokens == nil &&
		call.OutputTokens == nil && call.TotalTokens == nil {
		return nil
	}
	return call
}

func metadataTokenCount(value *structpb.Value) *uint64 {
	numberValue, ok := value.GetKind().(*structpb.Value_NumberValue)
	if !ok {
		return nil
	}
	number := numberValue.NumberValue
	// Struct 的 number 使用 double；只接收可以无损还原的非负整数，
	// 避免错误用量进入统计链路。
	if number < 0 || number > maxExactMetadataInteger || math.Trunc(number) != number {
		return nil
	}
	count := uint64(number)
	return &count
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

func resourceIDs(routeName string) (string, string, error) {
	// Controller 生成的 Route 名称格式为 ingate-route/<gateway-id>/<route-id>[/<method>][/<variant>]。
	parts := strings.Split(routeName, "/")
	if len(parts) == 0 || parts[0] != envoyRouteNamePrefix {
		return "", "", nil
	}
	if len(parts) < 3 ||
		!resourceconfig.IsCanonicalID(parts[1]) ||
		!resourceconfig.IsCanonicalID(parts[2]) {
		return "", "", errors.New("HTTP access log route identity is invalid")
	}
	return parts[1], parts[2], nil
}

func addByteCounts(left, right uint64) (uint64, error) {
	if right > math.MaxUint64-left {
		return 0, errors.New("byte count exceeds uint64")
	}
	return left + right, nil
}

func requestPath(value string) string {
	// 查询参数常含 Token、签名和业务标识；ALS 默认不采集它们，
	// 避免分析链路扩大敏感数据面。
	path, _, _ := strings.Cut(value, "?")
	return path
}

func requestHost(value string) string {
	// 控制台按域名筛选请求，监听端口已经由 Gateway 表达，不应混入 Host 维度。
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return host
	}
	return value
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
	return net.JoinHostPort(
		socket.GetAddress(),
		strconv.FormatUint(uint64(socket.GetPortValue()), 10),
	)
}
