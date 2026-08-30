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
)

func requestDecoder(request *http.Request, value any) error {
	message, ok := value.(proto.Message)
	if !ok {
		return kratoshttp.DefaultRequestDecoder(request, value)
	}
	data, err := io.ReadAll(request.Body)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return kerrors.New(
				http.StatusRequestEntityTooLarge,
				"REQUEST_BODY_TOO_LARGE",
				"request body too large",
			).WithCause(err)
		}
		return kerrors.BadRequest("INVALID_ARGUMENT", "read request body failed").WithCause(err)
	}
	if len(data) == 0 {
		return nil
	}
	// Assistant API 使用 Proto JSON 契约，使枚举、时间和 json_name 在前后端保持一致。
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, message); err != nil {
		return kerrors.BadRequest("INVALID_ARGUMENT", "decode request body failed").WithCause(err)
	}
	return nil
}

func responseEncoder(writer http.ResponseWriter, _ *http.Request, value any) error {
	var (
		data []byte
		err  error
	)
	if message, ok := value.(proto.Message); ok {
		data, err = (protojson.MarshalOptions{EmitUnpopulated: true}).Marshal(message)
	} else {
		data, err = json.Marshal(value)
	}
	if err != nil {
		return fmt.Errorf("encode assistant response: %w", err)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = writer.Write(data)
	return err
}
