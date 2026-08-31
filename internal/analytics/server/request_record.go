package server

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

type decodedRecords struct {
	records         []requestbiz.Record
	invalidCount    int
	duplicateCount  int
	firstInvalidErr error
}

// decodeRequestRecords 解码一个 Kafka 批次，并统计无效和重复消息。
func decodeRequestRecords(messages []*kgo.Record) decodedRecords {
	decoded := decodedRecords{records: make([]requestbiz.Record, 0, len(messages))}
	seen := make(map[string]*alsv1.RequestRecord, len(messages))
	for _, message := range messages {
		record, err := decodeRequestRecord(message)
		if err != nil {
			decoded.invalidCount++
			if decoded.firstInvalidErr == nil {
				decoded.firstInvalidErr = err
			}
			continue
		}
		if previous, exists := seen[record.GetId()]; exists {
			if proto.Equal(previous, record) {
				decoded.duplicateCount++
			} else {
				decoded.invalidCount++
				if decoded.firstInvalidErr == nil {
					decoded.firstInvalidErr = errors.New("request record ID is reused with different content")
				}
			}
			continue
		}
		seen[record.GetId()] = record
		decoded.records = append(decoded.records, domainRecord(record))
	}
	return decoded
}

func decodeRequestRecord(message *kgo.Record) (*alsv1.RequestRecord, error) {
	if message == nil {
		return nil, errors.New("kafka message is nil")
	}
	if len(message.Value) > requestrecord.MaxEncodedBytes {
		return nil, errors.New("request record exceeds the encoded size limit")
	}
	record := new(alsv1.RequestRecord)
	if err := proto.Unmarshal(message.Value, record); err != nil {
		return nil, fmt.Errorf("decode request record protobuf: %w", err)
	}
	if !validRequestRecordEnvelope(message, record) {
		return nil, errors.New("request record Kafka envelope is invalid")
	}
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	return record, nil
}

func validRequestRecordEnvelope(message *kgo.Record, record *alsv1.RequestRecord) bool {
	return bytes.Equal(message.Key, []byte(record.GetId())) &&
		hasHeader(message.Headers, requestrecord.ContentTypeHeader, requestrecord.ContentType) &&
		hasHeader(message.Headers, requestrecord.MessageTypeHeader, requestrecord.MessageType)
}

func hasHeader(headers []kgo.RecordHeader, key, value string) bool {
	found := false
	for _, header := range headers {
		if header.Key != key {
			continue
		}
		if found || !bytes.Equal(header.Value, []byte(value)) {
			return false
		}
		found = true
	}
	return found
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

// validateRecord 校验跨进程协议边界上 ClickHouse 列类型和查询所需的最小字段。
func validateRecord(record *alsv1.RequestRecord) error {
	if err := requestrecord.Validate(record); err != nil {
		return err
	}
	startedAt := record.GetStartedAt().AsTime()
	// 查询游标使用 Unix 纳秒；超出其可表示范围会发生整数环绕，必须在 Kafka 边界拒绝。
	if !analyticsconfig.IsSupportedTime(startedAt) {
		return errors.New("request record start time is outside the supported range")
	}
	return nil
}
