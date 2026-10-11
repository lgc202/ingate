package server

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strings"

	kratosprotojson "github.com/go-kratos/kratos/v3/encoding/protojson"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
)

func decodeHTTPRequest(r *http.Request, message any) error {
	switch message.(type) {
	case *adminv1.CreateGatewayRequest, *adminv1.UpdateGatewayRequest,
		*adminv1.CreateHTTPRouteRequest, *adminv1.UpdateHTTPRouteRequest,
		*adminv1.CreateBackendRequest, *adminv1.UpdateBackendRequest:
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			return kratoserrors.BadRequest("CODEC", "invalid resource Content-Type").WithCause(err)
		}
		// Kratos 按 subtype 选择 codec；text/json 等入口也不能绕过严格解码。
		_, subtype, _ := strings.Cut(mediaType, "/")
		if subtype == "json" || subtype == "protojson" {
			return decodeResourceJSON(r, message.(proto.Message))
		}
		return kratoshttp.DefaultRequestDecoder(r, message)
	}

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

// decodeResourceJSON 拒绝未实现的字段，避免把 Header 等未知约束丢弃后扩大路径匹配。
func decodeResourceJSON(r *http.Request, message proto.Message) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return kratoserrors.BadRequest("CODEC", "read JSON request failed").WithCause(err)
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) == 0 {
		return nil
	}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(data, message); err != nil {
		return kratoserrors.BadRequest("CODEC", "invalid JSON resource contract").WithCause(err)
	}
	return nil
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
