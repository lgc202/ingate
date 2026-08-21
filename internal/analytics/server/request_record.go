package server

import (
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
)

// decodeRequestRecords 解码一个 Kafka 批次，并统计无法通过协议校验的消息
func decodeRequestRecords(messages []*kgo.Record) ([]requestbiz.Record, int) {
	records := make([]requestbiz.Record, 0, len(messages))
	invalid := 0
	for _, message := range messages {
		record := new(alsv1.RequestRecord)
		if err := proto.Unmarshal(message.Value, record); err != nil || !validRecord(record) {
			invalid++
			continue
		}
		records = append(records, domainRecord(record))
	}
	return records, invalid
}

// domainRecord 在 Kafka 边界把传输协议转换为 Analytics 领域记录
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

// validRecord 校验跨进程协议边界上 ClickHouse 列类型和查询所需的最小字段
func validRecord(record *alsv1.RequestRecord) bool {
	if record.GetId() == "" || record.GetStartedAt() == nil || record.GetStartedAt().CheckValid() != nil {
		return false
	}
	if record.GetDuration() != nil && (record.GetDuration().CheckValid() != nil || record.GetDuration().AsDuration() < 0) {
		return false
	}
	if record.GetTimeToFirstByte() != nil &&
		(record.GetTimeToFirstByte().CheckValid() != nil || record.GetTimeToFirstByte().AsDuration() < 0) {
		return false
	}
	return record.GetStatusCode() <= 65535 && record.GetUpstreamAttempts() <= 65535
}
