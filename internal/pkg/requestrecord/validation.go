package requestrecord

import (
	"errors"

	"google.golang.org/protobuf/types/known/durationpb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// Validate 校验 ALS 与 Analytics 之间持久化请求记录的共享不变量。
func Validate(record *alsv1.RequestRecord) error {
	if record == nil {
		return errors.New("request record is nil")
	}
	if !IsValidID(record.GetId()) {
		return errors.New("request record ID is invalid")
	}
	if record.GetStartedAt() == nil || record.GetStartedAt().CheckValid() != nil {
		return errors.New("request record start time is invalid")
	}
	if !validDuration(record.GetDuration()) || !validDuration(record.GetTimeToFirstByte()) {
		return errors.New("request record duration is invalid")
	}
	if record.GetDuration() != nil && record.GetTimeToFirstByte() != nil &&
		record.GetTimeToFirstByte().AsDuration() > record.GetDuration().AsDuration() {
		return errors.New("request record time to first byte exceeds its duration")
	}

	statusCode := record.GetStatusCode()
	if statusCode > 65_535 || (statusCode > 0 && statusCode < 100) {
		return errors.New("request record status code is invalid")
	}
	if record.GetUpstreamAttempts() > 65_535 {
		return errors.New("request record upstream attempt count is invalid")
	}
	if (record.GetGatewayId() == "") != (record.GetRouteId() == "") {
		return errors.New("request record gateway and route identity is incomplete")
	}
	for _, resourceID := range []string{
		record.GetGatewayId(),
		record.GetRouteId(),
		record.GetUpstreamId(),
		record.GetCallerId(),
		record.GetAccessKeyId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return errors.New("request record contains an invalid resource ID")
		}
	}
	if (record.GetCallerId() == "") != (record.GetAccessKeyId() == "") {
		return errors.New("request record caller identity is incomplete")
	}
	return validateModelCall(record.GetUpstreamId(), record.GetAiModelCall())
}

func validDuration(value *durationpb.Duration) bool {
	if value == nil {
		return true
	}
	return value.CheckValid() == nil && value.AsDuration() >= 0
}

func validateModelCall(upstreamID string, call *alsv1.AIModelCall) error {
	if call == nil {
		return nil
	}
	if call.GetClientModel() == "" && call.GetUpstreamModel() == "" &&
		call.GetUpstreamProtocol() == "" && call.GetResponseModel() == "" &&
		call.GetFinishReason() == "" && call.InputTokens == nil &&
		call.OutputTokens == nil && call.TotalTokens == nil {
		return errors.New("request record contains an empty AI model call")
	}
	if !routeconfig.IsValidModelName(call.GetClientModel()) {
		return errors.New("request record client model is invalid")
	}
	for _, model := range []string{call.GetUpstreamModel(), call.GetResponseModel()} {
		if model != "" && !routeconfig.IsValidModelName(model) {
			return errors.New("request record contains an invalid model name")
		}
	}
	protocol := call.GetUpstreamProtocol()
	if protocol != "" && protocol != string(aiprotocol.UpstreamProtocolOpenAI) &&
		protocol != string(aiprotocol.UpstreamProtocolAnthropic) {
		return errors.New("request record upstream protocol is invalid")
	}
	hasUpstreamResult := call.GetUpstreamModel() != "" || protocol != "" ||
		call.GetResponseModel() != "" || call.GetFinishReason() != "" ||
		call.InputTokens != nil || call.OutputTokens != nil || call.TotalTokens != nil
	if hasUpstreamResult &&
		(upstreamID == "" || call.GetUpstreamModel() == "" || protocol == "") {
		return errors.New("request record AI upstream identity is incomplete")
	}
	if call.TotalTokens != nil {
		if call.InputTokens != nil && call.GetTotalTokens() < call.GetInputTokens() ||
			call.OutputTokens != nil && call.GetTotalTokens() < call.GetOutputTokens() {
			return errors.New("request record AI token usage is inconsistent")
		}
	}
	return nil
}
