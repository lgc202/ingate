package conversation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

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

func encodeMessageCursor(cursor *conversationbiz.MessageCursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	value, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal message cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeMessageCursor(value string) (*conversationbiz.MessageCursor, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode message cursor: %w", err)
	}
	var cursor conversationbiz.MessageCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return nil, fmt.Errorf("unmarshal message cursor: %w", err)
	}
	if cursor.CreatedAt.IsZero() || cursor.ID == "" {
		return nil, fmt.Errorf("message cursor is incomplete")
	}
	return &cursor, nil
}
