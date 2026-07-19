package sse

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDecoder_ArbitraryChunksAndFields(t *testing.T) {
	input := []byte(": comment\r\nid: 42\r\nevent: message\r\nretry: 1500\r\ndata: first\r\ndata: second\r\n\r\nretry: invalid\ndata: tail")
	var decoder Decoder
	var got []Event
	for _, value := range input {
		events, err := decoder.Push([]byte{value})
		if err != nil {
			t.Fatalf("Decoder.Push(%q) returned error: %v", value, err)
		}
		got = append(got, events...)
	}
	events, err := decoder.Finish()
	if err != nil {
		t.Fatalf("Decoder.Finish() returned error: %v", err)
	}
	got = append(got, events...)

	retry := 1500
	want := []Event{
		{Type: "message", Data: []byte("first\nsecond"), ID: "42", Retry: &retry},
		{Data: []byte("tail"), ID: "42"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Decoder events = %#v, want %#v", got, want)
	}
	if _, err := decoder.Push([]byte("data: late\n\n")); !errors.Is(err, ErrClosed) {
		t.Errorf("Decoder.Push() after Finish error = %v, want errors.Is(_, ErrClosed)", err)
	}
}

func TestDecoder_LargeEventHasNoScannerLimit(t *testing.T) {
	data := strings.Repeat("x", 128*1024)
	var decoder Decoder
	events, err := decoder.Push([]byte("data: " + data + "\n\n"))
	if err != nil {
		t.Fatalf("Decoder.Push(128KiB event) returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Decoder.Push(128KiB event) returned %d events, want 1", len(events))
	}
	if string(events[0].Data) != data {
		t.Errorf("Decoder.Push(128KiB event) data length = %d, want %d", len(events[0].Data), len(data))
	}
}

func TestDecoder_BufferLimit(t *testing.T) {
	data := strings.Repeat("x", MaxBufferedBytes-len("data: \n\n"))
	var decoder Decoder
	events, err := decoder.Push([]byte("data: " + data + "\n\n"))
	if err != nil {
		t.Fatalf("Decoder.Push(%d-byte event) returned error: %v", MaxBufferedBytes, err)
	}
	if len(events) != 1 || string(events[0].Data) != data {
		t.Errorf("Decoder.Push(%d-byte event) events = %#v, want one complete event", MaxBufferedBytes, events)
	}

	var oversized Decoder
	input := []byte("data: " + strings.Repeat("x", MaxBufferedBytes))
	if _, err := oversized.Push(input); !errors.Is(err, ErrBufferLimit) {
		t.Errorf("Decoder.Push(%d-byte unfinished event) error = %v, want errors.Is(_, ErrBufferLimit)", len(input), err)
	}

	var multiline Decoder
	line := "data: " + strings.Repeat("x", 1024) + "\n"
	input = []byte(strings.Repeat(line, MaxBufferedBytes/1024+2))
	if _, err := multiline.Push(input); !errors.Is(err, ErrBufferLimit) {
		t.Errorf("Decoder.Push(%d-byte unterminated multiline event) error = %v, want errors.Is(_, ErrBufferLimit)", len(input), err)
	}
}

func TestEncodeData_Multiline(t *testing.T) {
	got := EncodeData([]byte("first\nsecond"))
	want := []byte("data: first\ndata: second\n\n")
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeData(%q) = %q, want %q", "first\\nsecond", got, want)
	}
}
