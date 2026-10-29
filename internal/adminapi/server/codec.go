package server

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	kratosprotojson "github.com/go-kratos/kratos/v3/encoding/protojson"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/lgc202/ingate/internal/adminapi/errno"
)

func decodeHTTPRequest(r *http.Request, value any) error {
	message, ok := value.(proto.Message)
	if !ok {
		return kratoshttp.DefaultRequestDecoder(r, value)
	}

	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		// 无正文的 GET/DELETE 无须 Content-Type；有正文或声明了错误的媒体类型则拒绝，
		// 不能回退到宽松的默认 JSON codec，让未知字段检查被绕过。
		if contentType == "" && (r.Body == nil || r.Body == http.NoBody) {
			return kratoshttp.DefaultRequestDecoder(r, message)
		}

		return kratoserrors.BadRequest("CODEC", "invalid request Content-Type").WithCause(err)
	}

	// Kratos 按 subtype 选择 codec。所有 protobuf JSON 请求统一严格解码，
	// 既保留枚举、Timestamp 等 ProtoJSON 语义，也不让新增服务漏掉未知字段检查。
	_, subtype, _ := strings.Cut(mediaType, "/")
	if subtype == "json" || subtype == "protojson" {
		return decodeProtoJSON(r, message)
	}

	return kratoshttp.DefaultRequestDecoder(r, message)
}

// decodeProtoJSON 拒绝未实现的字段，避免把未知约束丢弃后扩大匹配或忽略用户意图。
func decodeProtoJSON(r *http.Request, message proto.Message) error {
	if r.Body == nil {
		return nil
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return kratoserrors.BadRequest("CODEC", "read JSON request failed").WithCause(err)
	}

	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) == 0 {
		return nil
	}

	if err := (protojson.UnmarshalOptions{}).Unmarshal(data, message); err != nil {
		return kratoserrors.BadRequest("CODEC", "invalid JSON request contract").WithCause(err)
	}

	return nil
}

func encodeHTTPResponse(w http.ResponseWriter, r *http.Request, value any) error {
	value = browserReply(w, r, value)
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
	public, ok := errors.AsType[*kratoserrors.Error](err)
	if !ok {
		kratoshttp.DefaultErrorEncoder(w, r, errno.Fallback(err))

		return
	}

	// 生成的 HTTP 入口先解码再执行中间件，解码错误需在此处统一成公开错误。
	if public.Code == http.StatusBadRequest && public.Reason == "CODEC" {
		err = errno.ErrInvalidArgument.WithCause(err)
	}

	kratoshttp.DefaultErrorEncoder(w, r, err)
}
