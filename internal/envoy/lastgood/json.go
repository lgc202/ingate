package lastgood

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Marshal 完整校验 Last Good 后编码持久化协议
func Marshal(record Record) ([]byte, error) {
	if _, err := record.Config(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode last good record: %w", err)
	}
	return data, nil
}

// Decode 严格解码并完整校验 Last Good 持久化协议
func Decode(reader io.Reader) (Record, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("%w: decode record: %v", ErrCorrupt, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Record{}, fmt.Errorf("%w: record contains trailing data", ErrCorrupt)
	}
	if _, err := record.Config(); err != nil {
		return Record{}, err
	}
	return record, nil
}
