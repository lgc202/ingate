package server

import (
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

// decodeRequestRecords 解码一个 Kafka 批次，并统计无法通过协议校验的消息。
func decodeRequestRecords(messages []*kgo.Record) ([]requestbiz.Record, int) {
	records := make([]requestbiz.Record, 0, len(messages))
	invalid := 0
	for _, message := range messages {
		if len(message.Value) > requestrecord.MaxEncodedBytes {
			invalid++
			continue
		}
		record := new(alsv1.RequestRecord)
		if err := proto.Unmarshal(message.Value, record); err != nil || !validRecord(record) {
			invalid++
			continue
		}
		records = append(records, domainRecord(record))
	}
	return records, invalid
}

// domainRecord 在 Kafka 边界把传输协议转换为 Analytics 领域记录。
func domainRecord(record *alsv1.RequestRecord) requestbiz.Record {
	return requestbiz.Record{
		ID:                  record.GetId(),
		RequestID:           record.GetRequestId(),
		StartedAt:           record.GetStartedAt().AsTime(),
		Duration:            durationValue(record.GetDuration()),
		ClientIP:            record.GetClientIp(),
		Method:              record.GetMethod(),
		Host:                record.GetHost(),
		Path:                record.GetPath(),
		StatusCode:          uint16(record.GetStatusCode()),
		RequestBytes:        record.GetRequestBytes(),
		ResponseBytes:       record.GetResponseBytes(),
		GatewayID:           record.GetGatewayId(),
		RouteID:             record.GetRouteId(),
		UpstreamID:          record.GetUpstreamId(),
		CallerID:            record.GetCallerId(),
		AccessKeyID:         record.GetAccessKeyId(),
		EnvoyNodeID:         record.GetEnvoyNodeId(),
		Protocol:            record.GetProtocol(),
		ResponseCodeDetails: record.GetResponseCodeDetails(),
		UpstreamAttempts:    uint16(record.GetUpstreamAttempts()),
		UpstreamAddress:     record.GetUpstreamAddress(),
		TimeToFirstByte:     durationValue(record.GetTimeToFirstByte()),
		ModelCall:           domainModelCall(record.GetAiModelCall()),
	}
}

func domainModelCall(call *alsv1.AIModelCall) *requestbiz.ModelCall {
	if call == nil {
		return nil
	}
	return &requestbiz.ModelCall{
		ClientModel:      call.GetClientModel(),
		UpstreamModel:    call.GetUpstreamModel(),
		UpstreamProtocol: call.GetUpstreamProtocol(),
		ResponseModel:    call.GetResponseModel(),
		FinishReason:     call.GetFinishReason(),
		InputTokens:      cloneUint64(call.InputTokens),
		OutputTokens:     cloneUint64(call.OutputTokens),
		TotalTokens:      cloneUint64(call.TotalTokens),
	}
}

func durationValue(value *durationpb.Duration) *time.Duration {
	if value == nil {
		return nil
	}
	duration := value.AsDuration()
	return &duration
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// validRecord 校验跨进程协议边界上 ClickHouse 列类型和查询所需的最小字段。
func validRecord(record *alsv1.RequestRecord) bool {
	if !requestrecord.IsValidID(record.GetId()) || record.GetStartedAt() == nil ||
		record.GetStartedAt().CheckValid() != nil {
		return false
	}
	startedAt := record.GetStartedAt().AsTime()
	// 查询游标使用 Unix 纳秒；超出其可表示范围会发生整数环绕，必须在 Kafka 边界拒绝。
	if !analyticsconfig.IsSupportedTime(startedAt) {
		return false
	}
	duration := record.GetDuration()
	if duration != nil && (duration.CheckValid() != nil || duration.AsDuration() < 0) {
		return false
	}
	firstByte := record.GetTimeToFirstByte()
	if firstByte != nil && (firstByte.CheckValid() != nil || firstByte.AsDuration() < 0) {
		return false
	}
	if duration != nil && firstByte != nil && firstByte.AsDuration() > duration.AsDuration() {
		return false
	}
	statusCode := record.GetStatusCode()
	if statusCode > 65535 ||
		(statusCode > 0 && statusCode < 100) ||
		record.GetUpstreamAttempts() > 65535 {
		return false
	}
	if (record.GetGatewayId() == "") != (record.GetRouteId() == "") {
		return false
	}
	for _, resourceID := range []string{
		record.GetGatewayId(),
		record.GetRouteId(),
		record.GetUpstreamId(),
		record.GetCallerId(),
		record.GetAccessKeyId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return false
		}
	}
	if (record.GetCallerId() == "") != (record.GetAccessKeyId() == "") {
		return false
	}
	call := record.GetAiModelCall()
	if call == nil {
		return true
	}
	protocol := call.GetUpstreamProtocol()
	if protocol != "" && protocol != string(aiprotocol.UpstreamProtocolOpenAI) &&
		protocol != string(aiprotocol.UpstreamProtocolAnthropic) {
		return false
	}
	if record.GetUpstreamId() != "" && (protocol == "" || call.GetUpstreamModel() == "") {
		return false
	}
	return true
}
