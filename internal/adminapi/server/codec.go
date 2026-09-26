package server

import (
	"mime"
	"net/http"

	kratosprotojson "github.com/go-kratos/kratos/v3/encoding/protojson"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"google.golang.org/protobuf/proto"
)

func decodeHTTPRequest(r *http.Request, message any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return kratoshttp.DefaultRequestDecoder(r, message)
	}

	// Kratos v3 对 application/json 使用 encoding/json，无法按 ProtoJSON 解码枚举。
	request := r.Clone(r.Context())
	request.Header.Set("Content-Type", "application/protojson")
	err = kratoshttp.DefaultRequestDecoder(request, message)
	r.Body = request.Body

	return err
}

func encodeHTTPResponse(w http.ResponseWriter, r *http.Request, value any) error {
	message, ok := value.(proto.Message)
	if !ok || r.Header.Get("Accept") == "application/protojson" {
		return kratoshttp.DefaultResponseEncoder(w, r, value)
	}

	// 普通 JSON 客户端也必须拿到标准 ProtoJSON，包括枚举和 Timestamp。
	data, err := kratosprotojson.MarshalOptions.Marshal(message)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)

	return err
}

func encodeHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	// 生成的 HTTP 入口先解码再执行中间件，解码错误需在此处统一成公开错误。
	if kratoserrors.Code(err) == http.StatusBadRequest && kratoserrors.Reason(err) == "CODEC" {
		err = adminhandler.ToInvalidArgument(err)
	}
	kratoshttp.DefaultErrorEncoder(w, r, err)
}
