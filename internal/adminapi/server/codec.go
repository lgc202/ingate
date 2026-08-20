package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

const userMessageMetadata = "user_message"

type responseEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func requestDecoder(request *http.Request, value any) error {
	message, ok := value.(proto.Message)
	if !ok {
		return kratoshttp.DefaultRequestDecoder(request, value)
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			serviceError := kratoserrors.New(
				http.StatusRequestEntityTooLarge,
				adminv1.ErrorReason_REQUEST_BODY_TOO_LARGE.String(),
				"request body too large",
			).WithMetadata(map[string]string{userMessageMetadata: "请求内容过大"})
			return serviceError.WithCause(err)
		}
		serviceError := kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "read request body failed").
			WithMetadata(map[string]string{userMessageMetadata: "读取请求内容失败"})
		return serviceError.WithCause(err)
	}
	if len(data) == 0 {
		return nil
	}
	// Kratos v3 默认 JSON codec 使用 encoding/json，无法按 Proto JSON 规则解析
	// 枚举名称和 json_name；Admin API 的请求与响应都应遵循同一份 Proto 契约
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, message); err != nil {
		serviceError := kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "decode request body failed").
			WithMetadata(map[string]string{userMessageMetadata: "请求内容格式不正确"})
		return serviceError.WithCause(err)
	}
	return nil
}

func responseEncoder(writer http.ResponseWriter, _ *http.Request, value any) error {
	data, err := marshalResponseData(value)
	if err != nil {
		return err
	}
	return writeJSON(writer, http.StatusOK, responseEnvelope{Code: http.StatusOK, Msg: "", Data: data})
}

func errorEncoder(writer http.ResponseWriter, _ *http.Request, err error) {
	serviceError := kratoserrors.FromError(err)
	statusCode := int(serviceError.Code)
	message := serviceError.Metadata[userMessageMetadata]
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
	// Kratos 的错误编码器不能返回写失败；此时连接通常已断开，无需再产生一条服务异常日志
	_ = writeJSON(writer, statusCode, responseEnvelope{Code: statusCode, Msg: message, Data: json.RawMessage("null")})
}

func endpointNotFoundHandler(writer http.ResponseWriter, request *http.Request) {
	err := kratoserrors.NotFound(adminv1.ErrorReason_ENDPOINT_NOT_FOUND.String(), "endpoint not found").
		WithMetadata(map[string]string{userMessageMetadata: "接口不存在"})
	errorEncoder(writer, request, err)
}

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	err := kratoserrors.New(http.StatusMethodNotAllowed, adminv1.ErrorReason_METHOD_NOT_ALLOWED.String(), "method not allowed").
		WithMetadata(map[string]string{userMessageMetadata: "请求方法不支持"})
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
		return err
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(statusCode)
	_, err = writer.Write(data)
	return err
}
