package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

type responseBody struct {
	Code    int             `json:"code"`
	Reason  string          `json:"reason,omitempty"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func requestDecoder(request *http.Request, value any) error {
	message, ok := value.(proto.Message)
	if !ok {
		return kratoshttp.DefaultRequestDecoder(request, value)
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return adminv1.ErrorRequestBodyTooLarge("请求内容过大").WithCause(err)
		}
		return adminv1.ErrorInvalidArgument("读取请求内容失败").WithCause(err)
	}
	if len(data) == 0 {
		return nil
	}
	// Kratos v3 默认 JSON codec 使用 encoding/json，无法按 Proto JSON 规则解析。
	// 枚举名称和 json_name；Admin API 的请求与响应都应遵循同一份 Proto 契约。
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, message); err != nil {
		return adminv1.ErrorInvalidArgument("请求内容格式不正确").WithCause(err)
	}
	return nil
}

func requestVarsDecoder(request *http.Request, value any) error {
	return decodeRequestParameters(kratoshttp.DefaultRequestVars, request, value)
}

func requestQueryDecoder(request *http.Request, value any) error {
	return decodeRequestParameters(kratoshttp.DefaultRequestQuery, request, value)
}

func decodeRequestParameters(
	decode kratoshttp.DecodeRequestFunc,
	request *http.Request,
	value any,
) error {
	if err := decode(request, value); err != nil {
		return adminv1.ErrorInvalidArgument("请求参数格式不正确").WithCause(err)
	}
	return nil
}

func responseEncoder(writer http.ResponseWriter, _ *http.Request, value any) error {
	data, err := marshalResponseData(value)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	return writeJSON(writer, http.StatusOK, responseBody{
		Code:    http.StatusOK,
		Message: "",
		Data:    data,
	})
}

func errorEncoder(writer http.ResponseWriter, _ *http.Request, err error) {
	serviceError := kerrors.FromError(err)
	if !isAdminError(serviceError) {
		serviceError = adminv1.ErrorInternalError("请求处理失败")
	}
	statusCode := int(serviceError.Code)
	reason := serviceError.Reason
	message := serviceError.Message
	if message == "" {
		switch statusCode {
		case http.StatusBadRequest:
			message = "请求参数不正确"
		case http.StatusNotFound:
			message = "接口不存在"
		default:
			message = "请求处理失败"
		}
	}
	// Kratos 的错误编码器不能返回写失败；此时连接通常已断开，
	// 无需再产生一条服务异常日志。
	_ = writeJSON(writer, statusCode, responseBody{
		Code:    statusCode,
		Reason:  reason,
		Message: message,
		Data:    json.RawMessage("null"),
	})
}

func isAdminError(err *kerrors.Error) bool {
	if err == nil || err.Code < http.StatusBadRequest || err.Code > 599 {
		return false
	}
	value, exists := adminv1.ErrorReason_value[err.Reason]
	return exists && value != int32(adminv1.ErrorReason_ERROR_REASON_UNSPECIFIED)
}

func endpointNotFoundHandler(writer http.ResponseWriter, request *http.Request) {
	err := adminv1.ErrorEndpointNotFound("接口不存在")
	errorEncoder(writer, request, err)
}

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	err := adminv1.ErrorMethodNotAllowed("请求方法不支持")
	errorEncoder(writer, request, err)
}

func marshalResponseData(value any) ([]byte, error) {
	if message, ok := value.(proto.Message); ok {
		return protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(message)
	}
	return json.Marshal(value)
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal JSON response: %w", err)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(statusCode)
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("write JSON response: %w", err)
	}
	return nil
}
