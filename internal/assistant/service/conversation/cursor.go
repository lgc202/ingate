package conversation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

const maxCursorLength = 512

type cursorValue struct {
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"id"`
}

func formatConversationCursor(cursor *conversationbiz.ConversationCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	return formatCursor(cursor.UpdatedAt, cursor.ID)
}

func parseConversationCursor(value string) (*conversationbiz.ConversationCursor, error) {
	if value == "" {
		return nil, nil
	}
	cursor, err := parseCursor(value)
	if err != nil {
		return nil, err
	}
	return &conversationbiz.ConversationCursor{
		UpdatedAt: cursor.Timestamp,
		ID:        cursor.ID,
	}, nil
}

func formatMessageCursor(cursor *conversationbiz.MessageCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	return formatCursor(cursor.CreatedAt, cursor.ID)
}

func parseMessageCursor(value string) (*conversationbiz.MessageCursor, error) {
	if value == "" {
		return nil, nil
	}
	cursor, err := parseCursor(value)
	if err != nil {
		return nil, err
	}
	return &conversationbiz.MessageCursor{
		CreatedAt: cursor.Timestamp,
		ID:        cursor.ID,
	}, nil
}

func formatCursor(timestamp time.Time, id string) (string, error) {
	encoded, err := json.Marshal(cursorValue{Timestamp: timestamp, ID: id})
	if err != nil {
		return "", fmt.Errorf("marshal page cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func parseCursor(value string) (cursorValue, error) {
	if len(value) > maxCursorLength {
		return cursorValue{}, errors.New("page cursor exceeds the size limit")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursorValue{}, fmt.Errorf("decode page cursor: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	var cursor cursorValue
	if err := decoder.Decode(&cursor); err != nil {
		return cursorValue{}, fmt.Errorf("unmarshal page cursor: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return cursorValue{}, errors.New("page cursor contains trailing data")
	}
	if cursor.Timestamp.IsZero() || cursor.Timestamp.Year() < 1000 ||
		cursor.Timestamp.Year() > 9999 || uuid.Validate(cursor.ID) != nil {
		return cursorValue{}, errors.New("page cursor contains invalid values")
	}
	cursor.Timestamp = cursor.Timestamp.UTC()
	return cursor, nil
}
