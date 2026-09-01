package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

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
		return fmt.Errorf("encode Assistant response: %w", err)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = writer.Write(data)
	return err
}
