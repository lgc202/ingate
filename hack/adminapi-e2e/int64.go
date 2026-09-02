package main

import (
	"fmt"
	"strconv"
)

type int64Value int64

func (v *int64Value) UnmarshalJSON(data []byte) error {
	text := string(data)
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("decode int64: %w", err)
	}
	*v = int64Value(parsed)
	return nil
}
