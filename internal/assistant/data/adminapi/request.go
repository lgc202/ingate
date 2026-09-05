package adminapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// ListFailures 查询排障所需的失败请求元数据，不读取请求内容和凭据。
func (c *Client) ListFailures(ctx context.Context, query agenttool.FailureQuery) (agenttool.FailurePage, error) {
	request := &adminv1.ListRequestRecordsRequest{
		StartTime: timestamppb.New(query.StartTime),
		EndTime:   timestamppb.New(query.EndTime),
		Outcome:   requestOutcome(query.Outcome),
		PageSize:  query.Limit,
	}
	applyFailureScope(request, query.ScopeType, query.ScopeID)
	result, err := c.records.ListRequestRecords(ctx, request)
	if err != nil {
		return agenttool.FailurePage{}, fmt.Errorf("list request records from Admin API: %w", err)
	}
	if result == nil {
		return agenttool.FailurePage{}, errors.New("list request records from Admin API: empty response")
	}

	records := make([]agenttool.Failure, 0, len(result.GetRecords()))
	for _, record := range result.GetRecords() {
		if err := validateFailureResponse(record, query); err != nil {
			return agenttool.FailurePage{}, err
		}
		records = append(records, agenttool.Failure{
			RecordID:   record.GetId(),
			StartedAt:  protoTime(record.GetStartedAt()),
			Method:     record.GetMethod(),
			Host:       record.GetHost(),
			Path:       record.GetPath(),
			StatusCode: record.GetStatusCode(),
			Duration:   protoDuration(record.GetDuration()),
			GatewayID:  record.GetGatewayId(),
			RouteID:    record.GetRouteId(),
			ServiceID:  record.GetServiceId(),
		})
	}
	scopeName := ""
	if query.ScopeType != "all" {
		scopeName, err = c.resourceName(ctx, agenttool.TrafficDimension(query.ScopeType), query.ScopeID)
		if err != nil {
			return agenttool.FailurePage{}, err
		}
	}
	return agenttool.FailurePage{
		ScopeName: scopeName,
		Items:     records,
		HasMore:   result.GetNextPageToken() != "",
	}, nil
}

// GetRequestRecord 按列表返回的记录标识和开始时间读取单次请求元数据。
// startedAt 同时作为 ClickHouse 分区查询条件，避免为一条记录扫描全部保留数据。
func (c *Client) GetRequestRecord(
	ctx context.Context,
	recordID string,
	startedAt time.Time,
) (agenttool.RequestRecord, error) {
	result, err := c.records.GetRequestRecord(ctx, &adminv1.GetRequestRecordRequest{
		Id:        recordID,
		StartedAt: timestamppb.New(startedAt),
	})
	if err != nil {
		return agenttool.RequestRecord{}, queryTargetError(
			fmt.Sprintf("get request record %s from Admin API", recordID),
			err,
		)
	}
	if err := validateRequestRecordResponse(result, recordID, startedAt); err != nil {
		return agenttool.RequestRecord{}, err
	}

	return requestRecordFromAPI(result), nil
}

func applyFailureScope(request *adminv1.ListRequestRecordsRequest, scopeType, scopeID string) {
	switch scopeType {
	case "gateway":
		request.GatewayId = scopeID
	case "route":
		request.RouteId = scopeID
	case "service":
		request.ServiceId = scopeID
	}
}

func requestOutcome(outcome agenttool.FailureOutcome) adminv1.RequestOutcome {
	switch outcome {
	case agenttool.FailureOutcomeClientError:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR
	case agenttool.FailureOutcomeServerError:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR
	case agenttool.FailureOutcomeNoResponse:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE
	default:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_UNSPECIFIED
	}
}

func requestRecordFromAPI(record *adminv1.RequestRecord) agenttool.RequestRecord {
	return agenttool.RequestRecord{
		RecordID:        record.GetId(),
		StartedAt:       protoTime(record.GetStartedAt()),
		Duration:        protoDuration(record.GetDuration()),
		TimeToFirstByte: optionalProtoDuration(record.GetTimeToFirstByte()),
		Method:          record.GetMethod(),
		Host:            record.GetHost(),
		Path:            record.GetPath(),
		StatusCode:      record.GetStatusCode(),
		Outcome:         requestOutcomeFromAPI(record.GetOutcome()),
		RequestBytes:    record.GetRequestBytes(),
		ResponseBytes:   record.GetResponseBytes(),
		GatewayID:       record.GetGatewayId(),
		RouteID:         record.GetRouteId(),
		ServiceID:       record.GetServiceId(),
		Protocol:        record.GetProtocol(),
		RejectionReason: rejectionReasonFromAPI(record.GetRejectionReason()),
		ServiceAttempts: record.GetServiceAttempts(),
		AIModelCall:     aiModelCallFromAPI(record.GetAiModelCall()),
		CallerID:        record.GetCallerId(),
	}
}

func requestOutcomeFromAPI(outcome adminv1.RequestOutcome) string {
	switch outcome {
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SUCCESS:
		return "success"
	case adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR:
		return "client_error"
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR:
		return "server_error"
	case adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE:
		return "no_response"
	default:
		return "unknown"
	}
}

func rejectionReasonFromAPI(reason adminv1.RequestRejectionReason) string {
	switch reason {
	case adminv1.RequestRejectionReason_REQUEST_REJECTION_REASON_TOKEN_QUOTA_EXCEEDED:
		return "token_quota_exceeded"
	default:
		return ""
	}
}

func aiModelCallFromAPI(call *adminv1.AIModelCall) *agenttool.AIModelCall {
	if call == nil {
		return nil
	}
	return &agenttool.AIModelCall{
		ClientModel:   call.GetClientModel(),
		TargetModel:   call.GetTargetModel(),
		Protocol:      modelProtocol(call.GetProtocol()),
		ResponseModel: call.GetResponseModel(),
		FinishReason:  call.GetFinishReason(),
		InputTokens:   copyOptionalUint64(call.InputTokens),
		OutputTokens:  copyOptionalUint64(call.OutputTokens),
		TotalTokens:   copyOptionalUint64(call.TotalTokens),
	}
}

func copyOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	return new(*value)
}

func validateFailureResponse(
	record *adminv1.RequestRecordSummary,
	query agenttool.FailureQuery,
) error {
	if record == nil || !requestrecord.IsValidID(record.GetId()) ||
		!validTimestamp(record.GetStartedAt()) ||
		!analyticsconfig.IsSupportedTime(record.GetStartedAt().AsTime()) ||
		!validDuration(record.GetDuration()) || record.GetMethod() == "" ||
		record.GetHost() == "" || record.GetPath() == "" {
		return errors.New("invalid request record summary returned by Admin API")
	}
	startedAt := record.GetStartedAt().AsTime()
	if startedAt.Before(query.StartTime) || !startedAt.Before(query.EndTime) {
		return fmt.Errorf("request record %s falls outside the requested time range", record.GetId())
	}
	if record.GetOutcome() != requestOutcome(query.Outcome) {
		return fmt.Errorf("request record %s has an unexpected outcome", record.GetId())
	}
	if !validOptionalResourceID(record.GetGatewayId()) ||
		!validOptionalResourceID(record.GetRouteId()) ||
		!validOptionalResourceID(record.GetServiceId()) {
		return fmt.Errorf("request record %s contains an invalid resource reference", record.GetId())
	}
	if (record.GetGatewayId() == "") != (record.GetRouteId() == "") {
		return fmt.Errorf("request record %s contains incomplete route identity", record.GetId())
	}
	if !failureMatchesScope(record, query) {
		return fmt.Errorf("request record %s falls outside the requested resource scope", record.GetId())
	}
	return nil
}

func validateRequestRecordResponse(
	record *adminv1.RequestRecord,
	recordID string,
	startedAt time.Time,
) error {
	if record == nil || record.GetId() != recordID || !requestrecord.IsValidID(record.GetId()) ||
		!validTimestamp(record.GetStartedAt()) ||
		!analyticsconfig.IsSupportedTime(record.GetStartedAt().AsTime()) ||
		!record.GetStartedAt().AsTime().Equal(startedAt) || !validDuration(record.GetDuration()) ||
		record.GetMethod() == "" || record.GetHost() == "" || record.GetPath() == "" ||
		!validRequestOutcome(record.GetOutcome()) {
		return errors.New("invalid request record returned by Admin API")
	}
	if firstByte := record.GetTimeToFirstByte(); firstByte != nil &&
		(!validDuration(firstByte) || firstByte.AsDuration() > record.GetDuration().AsDuration()) {
		return fmt.Errorf("request record %s has an invalid time to first byte", recordID)
	}
	if !validOptionalResourceID(record.GetGatewayId()) ||
		!validOptionalResourceID(record.GetRouteId()) ||
		!validOptionalResourceID(record.GetServiceId()) ||
		!validOptionalResourceID(record.GetCallerId()) {
		return fmt.Errorf("request record %s contains an invalid resource reference", recordID)
	}
	if (record.GetGatewayId() == "") != (record.GetRouteId() == "") {
		return fmt.Errorf("request record %s contains incomplete route identity", recordID)
	}
	statusCode := record.GetStatusCode()
	if statusCode > 65_535 || statusCode > 0 && statusCode < 100 ||
		record.GetServiceAttempts() > 65_535 {
		return fmt.Errorf("request record %s contains invalid HTTP result metadata", recordID)
	}
	if reason := record.GetRejectionReason(); reason !=
		adminv1.RequestRejectionReason_REQUEST_REJECTION_REASON_UNSPECIFIED &&
		reason != adminv1.RequestRejectionReason_REQUEST_REJECTION_REASON_TOKEN_QUOTA_EXCEEDED {
		return fmt.Errorf("request record %s contains an invalid rejection reason", recordID)
	}
	if err := validateAIModelCallResponse(record.GetAiModelCall(), record.GetServiceId()); err != nil {
		return fmt.Errorf("request record %s: %w", recordID, err)
	}
	return nil
}

func failureMatchesScope(record *adminv1.RequestRecordSummary, query agenttool.FailureQuery) bool {
	switch query.ScopeType {
	case "gateway":
		return record.GetGatewayId() == query.ScopeID
	case "route":
		return record.GetRouteId() == query.ScopeID
	case "service":
		return record.GetServiceId() == query.ScopeID
	default:
		return true
	}
}

func validateAIModelCallResponse(call *adminv1.AIModelCall, serviceID string) error {
	if call == nil {
		return nil
	}
	if !routeconfig.IsValidModelName(call.GetClientModel()) {
		return errors.New("AI client model is invalid")
	}
	for _, model := range []string{call.GetTargetModel(), call.GetResponseModel()} {
		if model != "" && !routeconfig.IsValidModelName(model) {
			return errors.New("AI model name is invalid")
		}
	}
	protocol := call.GetProtocol()
	if protocol != adminv1.ModelProtocol_MODEL_PROTOCOL_UNSPECIFIED &&
		protocol != adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI &&
		protocol != adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC {
		return errors.New("AI model protocol is invalid")
	}
	hasServiceResult := call.GetTargetModel() != "" ||
		protocol != adminv1.ModelProtocol_MODEL_PROTOCOL_UNSPECIFIED ||
		call.GetResponseModel() != "" || call.GetFinishReason() != "" ||
		call.InputTokens != nil || call.OutputTokens != nil || call.TotalTokens != nil
	if hasServiceResult && (serviceID == "" || call.GetTargetModel() == "" ||
		protocol == adminv1.ModelProtocol_MODEL_PROTOCOL_UNSPECIFIED) {
		return errors.New("AI service identity is incomplete")
	}
	if call.InputTokens != nil && call.OutputTokens != nil &&
		call.GetInputTokens() > math.MaxUint64-call.GetOutputTokens() {
		return errors.New("AI token usage exceeds the supported range")
	}
	minimumTotal := max(call.GetInputTokens(), call.GetOutputTokens())
	if call.InputTokens != nil && call.OutputTokens != nil {
		minimumTotal = call.GetInputTokens() + call.GetOutputTokens()
	}
	if call.TotalTokens != nil && call.GetTotalTokens() < minimumTotal {
		return errors.New("AI token usage is inconsistent")
	}
	return nil
}

func validRequestOutcome(outcome adminv1.RequestOutcome) bool {
	switch outcome {
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SUCCESS,
		adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR,
		adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR,
		adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE:
		return true
	default:
		return false
	}
}
