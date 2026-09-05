package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

type requestRecordInput struct {
	RecordID  string `json:"record_id" jsonschema_description:"list_recent_failures 返回的记录 ID"`
	StartedAt string `json:"started_at" jsonschema_description:"list_recent_failures 返回的请求开始时间，RFC3339 格式"`
}

type requestRecordOutput struct {
	Summary string             `json:"summary"`
	Source  string             `json:"source,omitempty"`
	Status  string             `json:"status"`
	Record  *requestRecordInfo `json:"record,omitempty"`
}

type requestRecordInfo struct {
	RecordID              string           `json:"record_id"`
	StartedAt             string           `json:"started_at"`
	Method                string           `json:"method"`
	Host                  string           `json:"host"`
	Path                  string           `json:"path"`
	StatusCode            uint32           `json:"status_code"`
	Outcome               string           `json:"outcome"`
	DurationMillis        float64          `json:"duration_millis"`
	TimeToFirstByteMillis *float64         `json:"time_to_first_byte_millis,omitempty"`
	RequestBytes          uint64           `json:"request_bytes"`
	ResponseBytes         uint64           `json:"response_bytes"`
	GatewayID             string           `json:"gateway_id,omitempty"`
	RouteID               string           `json:"route_id,omitempty"`
	ServiceID             string           `json:"service_id,omitempty"`
	Protocol              string           `json:"protocol,omitempty"`
	RejectionReason       string           `json:"rejection_reason,omitempty"`
	ServiceAttempts       uint32           `json:"service_attempts"`
	AIModelCall           *aiModelCallInfo `json:"ai_model_call,omitempty"`
	CallerID              string           `json:"caller_id,omitempty"`
}

type aiModelCallInfo struct {
	ClientModel   string  `json:"client_model,omitempty"`
	TargetModel   string  `json:"target_model,omitempty"`
	Protocol      string  `json:"protocol,omitempty"`
	ResponseModel string  `json:"response_model,omitempty"`
	FinishReason  string  `json:"finish_reason,omitempty"`
	InputTokens   *uint64 `json:"input_tokens,omitempty"`
	OutputTokens  *uint64 `json:"output_tokens,omitempty"`
	TotalTokens   *uint64 `json:"total_tokens,omitempty"`
}

// RequestRecordReader 是单次请求明细工具实际使用的查询边界。
type RequestRecordReader interface {
	GetRequestRecord(context.Context, string, time.Time) (RequestRecord, error)
}

func newRequestRecordTool(records RequestRecordReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		getRequestRecordTool,
		"查询 list_recent_failures 返回的某一条请求明细。仅在需要解释具体失败样本时使用，返回耗时、流量大小、转发次数、拒绝原因和可用的模型调用结果。",
		func(ctx context.Context, input requestRecordInput) (requestRecordOutput, error) {
			return getRequestRecord(ctx, records, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", getRequestRecordTool, err)
	}
	return definition, nil
}

func getRequestRecord(
	ctx context.Context,
	records RequestRecordReader,
	input requestRecordInput,
) (requestRecordOutput, error) {
	recordID := strings.TrimSpace(input.RecordID)
	if !requestrecord.IsValidID(recordID) {
		return requestRecordErrorResult(
			invalidInputf("record_id must use the exact value returned by list_recent_failures"),
		)
	}
	startedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(input.StartedAt))
	if err != nil {
		return requestRecordErrorResult(
			invalidInputf("started_at must be the RFC3339 value returned by list_recent_failures"),
		)
	}
	if !analyticsconfig.IsSupportedTime(startedAt) {
		return requestRecordErrorResult(
			invalidInputf("started_at is outside the supported request record range"),
		)
	}

	record, err := records.GetRequestRecord(ctx, recordID, startedAt)
	if err != nil {
		return requestRecordErrorResult(err)
	}
	return requestRecordOutput{
		Summary: fmt.Sprintf(
			"已读取 %s %s%s 的单次请求元数据",
			record.Method,
			record.Host,
			record.Path,
		),
		Source: "request_records",
		Status: "complete",
		Record: requestRecordInfoFromRecord(record),
	}, nil
}

func requestRecordInfoFromRecord(record RequestRecord) *requestRecordInfo {
	return &requestRecordInfo{
		RecordID:              record.RecordID,
		StartedAt:             record.StartedAt.Format(time.RFC3339Nano),
		Method:                record.Method,
		Host:                  record.Host,
		Path:                  record.Path,
		StatusCode:            record.StatusCode,
		Outcome:               record.Outcome,
		DurationMillis:        durationMillis(record.Duration),
		TimeToFirstByteMillis: durationMillisPointer(record.TimeToFirstByte),
		RequestBytes:          record.RequestBytes,
		ResponseBytes:         record.ResponseBytes,
		GatewayID:             record.GatewayID,
		RouteID:               record.RouteID,
		ServiceID:             record.ServiceID,
		Protocol:              record.Protocol,
		RejectionReason:       record.RejectionReason,
		ServiceAttempts:       record.ServiceAttempts,
		AIModelCall:           aiModelCallInfoFromRecord(record.AIModelCall),
		CallerID:              record.CallerID,
	}
}

func durationMillisPointer(value *time.Duration) *float64 {
	if value == nil {
		return nil
	}
	return new(durationMillis(*value))
}

func aiModelCallInfoFromRecord(call *AIModelCall) *aiModelCallInfo {
	if call == nil {
		return nil
	}
	return &aiModelCallInfo{
		ClientModel:   call.ClientModel,
		TargetModel:   call.TargetModel,
		Protocol:      call.Protocol,
		ResponseModel: call.ResponseModel,
		FinishReason:  call.FinishReason,
		InputTokens:   call.InputTokens,
		OutputTokens:  call.OutputTokens,
		TotalTokens:   call.TotalTokens,
	}
}

func requestRecordErrorResult(err error) (requestRecordOutput, error) {
	summary, status, ok := recoverableToolError(err)
	if !ok {
		return requestRecordOutput{}, err
	}
	return requestRecordOutput{
		Summary: summary,
		Status:  status,
	}, nil
}
