package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const jsonContentType = "application/json; charset=utf-8"

func responseEncoder(writer http.ResponseWriter, _ *http.Request, value any) error {
	data, err := marshalResponse(value)
	if err != nil {
		return fmt.Errorf("encode Assistant response: %w", err)
	}
	writer.Header().Set("Content-Type", jsonContentType)
	_, err = writer.Write(data)
	return err
}

func marshalResponse(value any) ([]byte, error) {
	if message, ok := value.(proto.Message); ok {
		return protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(message)
	}
	return json.Marshal(value)
}
