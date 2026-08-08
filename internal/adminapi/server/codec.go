package server

import (
	"encoding/json"
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

const userMessageMetadata = "user_message"

type response struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func responseEncoder(writer http.ResponseWriter, _ *http.Request, value any) error {
	data, err := marshal(value)
	if err != nil {
		return err
	}
	return writeJSON(writer, http.StatusOK, response{Code: http.StatusOK, Msg: "", Data: data})
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
	_ = writeJSON(writer, statusCode, response{Code: statusCode, Msg: message, Data: json.RawMessage("null")})
}

func notFound(writer http.ResponseWriter, request *http.Request) {
	errorEncoder(writer, request, kratoserrors.NotFound(adminv1.ErrorReason_ROUTE_NOT_FOUND.String(), "route not found"))
}

func methodNotAllowed(writer http.ResponseWriter, request *http.Request) {
	err := kratoserrors.New(http.StatusMethodNotAllowed, adminv1.ErrorReason_METHOD_NOT_ALLOWED.String(), "method not allowed").
		WithMetadata(map[string]string{userMessageMetadata: "请求方法不支持"})
	errorEncoder(writer, request, err)
}

func marshal(value any) ([]byte, error) {
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
