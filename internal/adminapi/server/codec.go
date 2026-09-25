package server

import (
	"mime"
	"net/http"

	kratosprotojson "github.com/go-kratos/kratos/v3/encoding/protojson"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
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
