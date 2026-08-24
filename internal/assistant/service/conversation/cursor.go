package conversation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

func encodeConversationCursor(cursor *conversationbiz.ConversationCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	value, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal conversation cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeConversationCursor(value string) (*conversationbiz.ConversationCursor, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode conversation cursor: %w", err)
	}
	var cursor conversationbiz.ConversationCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return nil, fmt.Errorf("unmarshal conversation cursor: %w", err)
	}
	if cursor.UpdatedAt.IsZero() || cursor.ID == "" {
		return nil, fmt.Errorf("conversation cursor is incomplete")
	}
	return &cursor, nil
}

func encodeMessageCursor(sequence int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(sequence, 10)))
}

func decodeMessageCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, fmt.Errorf("decode message cursor: %w", err)
	}
	sequence, err := strconv.ParseInt(string(decoded), 10, 64)
	if err != nil || sequence < 0 {
		return 0, fmt.Errorf("message cursor is invalid")
	}
	return sequence, nil
}
